// The network package implements a circular overlay network which
// maintains itself and allow new nodes to connect. Each node keeps
// track of its left and right neighbour as well as its second neighbour
// on the left. This allows for easy maintainence of the circular
// overlay network. The network can recover from the simultaneous loss
// of multiple nonconsecutive nodes.
package network

import (
	"encoding/binary"
	"math/rand"
	"time"
	"errors"
)

const (
	aliveTime          = 1000 * time.Millisecond
	kickTime           = 750 * time.Millisecond
	broadcastTime      = 5000 * time.Millisecond
	msgResendInterval  = 2000 * time.Millisecond
	kickResendInterval = 200 * time.Millisecond
	lonelyDelay        = 1000 * time.Millisecond
)

const (
	bufferSize     = 32
	maxDataSize    = 240
	maxResendCount = 5
	maxReadCount   = 100
	maxResenders   = 100
)

// Message types. User-defined message types must be >= 16.
// 0-15 are reserved for future use.
const (
	BROADCAST = 0x0 // Announce that node is ready to connect.
	HELLO     = 0x1 // Reply to broadcasting node with new possible links.
	UPDATE    = 0x2 // Update links on neighbouring nodes.
	GET       = 0x3 // Request for UPDATE of left2ndNode.
	PING      = 0x4 // Check that node is alive.
	ALIVE     = 0x5 // Reply to PING.
	KICK      = 0x6
)

type Message struct {
	ID        uint32
	Type      uint32
	ReadCount uint32
	_         uint32 // padding
	Data      [maxDataSize]byte
}

func NewMessage(mtype uint32, data EncoderDecoder) *Message {
	msg := &Message{ID: rand.Uint32(), Type: mtype}
	if data != nil {
		data.Encode(msg.Data[:])
	}
	return msg
}

type HelloData struct {
	newRight   uint32
	newLeft    uint32
	newLeft2nd uint32
}

type UpdateData struct {
	right   uint32
	left    uint32
	left2nd uint32
}

type KickData struct {
	deadNode   uint32
	senderNode uint32
}

type Resender struct {
	msg            *Message
	timer          *SafeTimer
	resendInterval time.Duration
	triesLeft      int
	stopc          chan struct{}
}

type Node struct {
	connected bool
	running   bool
	stopc     chan struct{}

	// Only thisNode and anyNode are guaranteed to be nonzero at all times.
	thisNode    uint32
	leftNode    uint32
	left2ndNode uint32
	rightNode   uint32
	anyNode     uint32

	udp *UDPService

	// Channels are for user-defined messages. They are buffered and
	// when they are full new messages will be dropped.
	fromUserToUser  chan *Message
	fromUserToOther chan *Message
	toSend          chan *Message
	toForward       chan *Message

	deadNodes chan uint32

	aliveTimer     *SafeTimer
	kickTimer      *SafeTimer
	leftIsAlive    bool
	left2ndIsAlive bool

	broadcastTimer *SafeTimer

	// Note: The map datatype in Go is not thread-safe. In this
	// case access is controlled by the for/select loop in maintainNetwork.
	resenders        map[uint32]*Resender
	resenderTimedOut chan uint32
}

func (n *Node) Start() error {
	if !n.running {
		var err error
		n.udp, err = NewUDPService()
		if err != nil {
			return err
		}

		n.thisNode = NetworkAddr()
		n.anyNode = BroadcastAddr()
		if n.thisNode == 0 && n.anyNode == 0 {
			return nil
		}

		n.aliveTimer = NewSafeTimer(aliveTime)
		n.kickTimer = NewSafeTimer(kickTime)
		n.broadcastTimer = NewSafeTimer(broadcastTime)

		n.resenders = make(map[uint32]*Resender)
		n.resenderTimedOut = make(chan uint32, maxResenders)

		n.fromUserToUser = make(chan *Message, bufferSize)
		n.fromUserToOther = make(chan *Message, bufferSize)

		n.toSend = make(chan *Message, bufferSize)
		n.toForward = make(chan *Message, bufferSize)

		n.deadNodes = make(chan uint32, bufferSize)

		n.stopc = make(chan struct{})

		go n.maintainNetwork()
		n.running = true
	}
	return nil
}

func (n *Node) IsRunning() bool {
	return n.running
}

func (n *Node) IsConnected() bool {
	return n.connected
}

func (n *Node) Stop() {
	close(n.stopc)
	n.running = false
}

func (n *Node) ReceiveMyMessage() *Message {
	return <-n.fromUserToUser
}

func (n *Node) ReceiveMessage() *Message {
	return <-n.fromUserToUser
}

func (n *Node) ForwardMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	n.toForward <- msg
}

func (n *Node) SendMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	n.toSend <- msg
}

func (n *Node) Addr() uint32 {
	return n.thisNode
}

func (n *Node) GetDeadNodeAddr() uint32 {
	return <-n.deadNodes
}

func (n *Node) maintainNetwork() {
	for {
		select {
		case umsg := <-n.udp.receivec:
			n.processUDPMessage(umsg)

		case msg := <-n.toForward:
			n.forwardMsg(msg)

		case msg := <-n.toSend:
			n.addResender(msg, msgResendInterval)

		case ID := <-n.resenderTimedOut:
			if re, ok := n.resenders[ID]; ok {
				if re.triesLeft > 0 {
					n.forwardMsg(re.msg)
					re.triesLeft--
					re.timer.SafeReset(re.resendInterval)
				} else {
					n.removeResender(re)
					n.leftNode = 0
					n.rightNode = 0
					n.update()
				}
			}

		case <-n.aliveTimer.C:
			n.aliveTimer.Seen()
			if n.connected {
				n.leftIsAlive = false
				n.left2ndIsAlive = false
				n.sendData(n.leftNode, PING, nil)
				if n.left2ndNode != 0 && n.left2ndNode != n.thisNode {
					n.sendData(n.left2ndNode, PING, nil)
				}
				n.kickTimer.SafeReset(kickTime)
			}

		case <-n.kickTimer.C:
			n.kickTimer.Seen()
			if n.connected {
				if n.leftIsAlive && n.left2ndIsAlive {
					n.aliveTimer.SafeReset(aliveTime)
				} else {
					n.restoreNetwork()
				}
			}

		case <-n.broadcastTimer.C:
			n.broadcastTimer.Seen()
			if !n.connected {
				n.sendData(n.anyNode, BROADCAST, nil)
				n.broadcastTimer.SafeReset(broadcastTime)
			}

		case <-n.stopc:
			n.running = false
			n.connected = false
			for _, re := range n.resenders {
				n.removeResender(re)
			}
			return
		}
	}
}

func (n *Node) restoreNetwork() error {
	if (!n.leftIsAlive && !n.left2ndIsAlive) &&
		(n.left2ndNode == 0 || n.left2ndNode == n.thisNode) {
		// This happens when there are only two nodes in the
		// network and when left2ndNode has not yet been updated
		// by leftNode. Thus since leftIsAlive is false there is
		// no way of recovering and we have no choice but to
		// disconnect.
		n.leftNode = 0
		n.update()
		return errors.New("Not able to restore connectivity.")
	} else if !n.leftIsAlive && n.left2ndIsAlive {
		// Easy removal of dead node is possible.
		deadNode := n.leftNode
		n.leftNode = n.left2ndNode
		n.sendData(n.rightNode, UPDATE, &UpdateData{
			left2nd: n.left2ndNode,
		})
		n.sendData(n.leftNode, UPDATE, &UpdateData{
			right: n.thisNode,
		})
		// This leaves n.left2ndNode incorrectly pointing to
		// n.leftNode. The link to the 2nd node is not critical
		// for forwarding messages, but we can't start pinging it
		// before it is set correctly. One solution is to set
		// it to zero and don't ping untill it is set by an UPDATE.
		n.left2ndNode = 0
		n.sendData(n.leftNode, GET, nil)

		n.addResender(NewMessage(KICK, &KickData{
			deadNode:   deadNode,
			senderNode: n.thisNode,
		}), kickResendInterval)

		n.aliveTimer.SafeReset(aliveTime)
		return nil
	}
	return errors.New("Not able to restore connectivity.")
}

func (n *Node) processUDPMessage(umsg *UDPMessage) {
	msg := new(Message)
	msg.Decode(umsg.payload[:])

	if msg.ReadCount > maxReadCount {
		return
	}
	msg.ReadCount++

	switch msg.Type {
	case BROADCAST:
		if umsg.from != n.thisNode {
			var hd HelloData
			if n.connected {
				hd.newRight = n.rightNode
				hd.newLeft = n.thisNode
				hd.newLeft2nd = n.leftNode
			} else {
				hd.newRight = n.thisNode
				hd.newLeft = n.thisNode
				hd.newLeft2nd = umsg.from
			}

			// Avoid forming disjoint networks at statup.
			if !n.connected {
				time.Sleep(lonelyDelay)
			}
			n.sendData(umsg.from, HELLO, &hd)
		}

	case HELLO:
		if !n.connected {
			var hd HelloData
			hd.Decode(msg.Data[:])

			n.rightNode = hd.newRight
			n.leftNode = hd.newLeft
			n.update()
			
			if hd.newRight == hd.newLeft {
				n.sendData(n.leftNode, UPDATE, &UpdateData{
					right:   n.thisNode,
					left:    n.thisNode,
					left2nd: umsg.from,
				})
			} else {
				n.sendData(n.rightNode, UPDATE, &UpdateData{
					left:    n.thisNode,
					left2nd: n.leftNode,
				})
				n.sendData(n.leftNode, UPDATE, &UpdateData{right: n.thisNode})
			}
			// This connection protocol works perfectly if all messages are received.
			// TODO: What happens if one or both UPDATE messages never arrive?
		}

	case UPDATE:
		var ud UpdateData
		ud.Decode(msg.Data[:])
		if ud.right != 0 {
			n.rightNode = ud.right
		}
		if ud.left != 0 {
			n.leftNode = ud.right
		}
		if ud.left2nd != 0 {
			n.left2ndNode = ud.left2nd
		}
		n.update()

	case GET:
		if n.connected {
			n.sendData(umsg.from, UPDATE, &UpdateData{left2nd: n.leftNode})
		}

	case PING:
		if n.connected {
			n.sendData(umsg.from, ALIVE, nil)
		}

	case ALIVE:
		if n.connected {
			switch umsg.from {
			case n.leftNode:
				n.leftIsAlive = true
			case n.left2ndNode:
				n.left2ndIsAlive = true
			}
		}

	case KICK:
		if n.connected {
			var kick KickData
			kick.Decode(msg.Data[:])

			select {
			case n.deadNodes <- kick.deadNode:
			default:
			}
		}
	}

	// User-defined message type
	if msg.Type >= 5 {
		if n.connected && umsg.from == n.leftNode {
			var c chan *Message
			if re, ok := n.resenders[msg.ID]; ok {
				c = n.fromUserToUser
				n.removeResender(re)
			} else {
				c = n.fromUserToOther
			}

			select {
			case c <- msg:
			default:
			}
		}
	}
}

func (n *Node) addResender(msg *Message, resendInterval time.Duration) {
	re := &Resender{
		msg:       msg,
		timer:     NewSafeTimer(resendInterval),
		triesLeft: maxResendCount,
		stopc:     make(chan struct{}),
	}
	n.resenders[msg.ID] = re

	go func(n *Node, msg *Message) {
		for {
			select {
			case <-re.stopc:
				return
			case <-re.timer.C:
				re.timer.Seen()
				n.resenderTimedOut <- msg.ID
			}
		}
	}(n, msg)
}

func (n *Node) removeResender(re *Resender) {
	// Stop forwarding goroutine.
	close(re.stopc)
	delete(n.resenders, re.msg.ID)
}

func (n *Node) sendData(to uint32, mtype uint32, data EncoderDecoder) {
	umsg := &UDPMessage{to: to, from: n.thisNode}
	binary.BigEndian.PutUint32(umsg.payload[:], rand.Uint32())
	binary.BigEndian.PutUint32(umsg.payload[4:], mtype)
	if data != nil {
		data.Encode(umsg.payload[16:])
	}
	n.udp.Send(umsg)
}

func (n *Node) forwardMsg(msg *Message) {
	umsg := new(UDPMessage)
	umsg.from = n.thisNode
	umsg.to = n.rightNode
	msg.Encode(umsg.payload[:])
	n.udp.Send(umsg)
}

func (n *Node) update() {
	if n.leftNode == 0 || n.rightNode == 0 {
		n.leftNode = 0
		n.rightNode = 0
		n.left2ndNode = 0
	} else if !n.connected {
		n.connected = true
		n.aliveTimer.SafeReset(aliveTime)
	}
}
