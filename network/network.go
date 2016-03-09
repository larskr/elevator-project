// The network package implements a circular overlay network which maintains itself and
// allow new nodes to connect. Each node keeps track of its left and right neighbour as
// well as its second neighbour on the left. This allows for easy maintainence of the
// circular overlay network. The network can recover from the simultaneous loss of
// multiple nonconsecutive nodes. 
package network

import (
	"encoding/binary"
	"math/rand"
	"time"
)

const (
	ALIVETIME            = 500 * time.Millisecond
	KICKTIME             = 2500 * time.Millisecond
	BROADCASTTIME        = 5000 * time.Millisecond
	MSG_RESEND_INTERVAL  = 2000 * time.Millisecond
	KICK_RESEND_INTERVAL = 200 * time.Millisecond
	LONELY_DELAY         = 1000 * time.Millisecond
)

const (
	BUFFERED_CHAN_SIZE = 32
	MAX_DATA_SIZE      = 240
	MAX_RESEND_COUNT   = 5
	MAX_READ_COUNT     = 100
)

// Message types. User-defined message types must be greater than 4.
const (
	BROADCAST = 0x0
	HELLO     = 0x1
	ADD       = 0x2
	KICK      = 0x3
	ALIVE     = 0x4
)

// Direction to send message.
type MsgDirection int

const (
	RIGHT MsgDirection = -1
	LEFT  MsgDirection = 1
)

type Message struct {
	ID        uint32
	Type      uint32
	ReadCount uint32
	_         uint32 // padding
	Data      [MAX_DATA_SIZE]byte
}

func NewMessage(mtype uint32, data BinaryMarshaller) *Message {
	msg := &Message{ID: rand.Uint32(), Type: mtype}
	if data != nil {
		data.Encode(msg.Data[:])
	}
	return msg
}

type HelloData struct {
	possibleNewRight uint32
	possibleNewLeft  uint32
}

type AddData struct {
	asRight uint32
	asLeft  uint32
	as2ndLeft uint32
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

	thisNode  uint32
	leftNode  uint32
	rightNode uint32
	anyNode   uint32

	udp *UDPService

	msgsFromUserToUser  chan *Message
	msgsFromUserToOther chan *Message
	msgsToSend          chan *Message
	msgsToForward       chan *Message

	deadNodes chan uint32

	running bool
	stopc   chan struct{}

	aliveTimer     *SafeTimer
	kickTimer      *SafeTimer
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

		n.aliveTimer = NewSafeTimer(ALIVETIME)
		n.kickTimer = NewSafeTimer(KICKTIME)
		n.broadcastTimer = NewSafeTimer(BROADCASTTIME)

		n.resenders = make(map[uint32]*Resender)
		n.resenderTimedOut = make(chan uint32, BUFFERED_CHAN_SIZE)

		n.msgsFromUserToUser = make(chan *Message, BUFFERED_CHAN_SIZE)
		n.msgsFromUserToOther = make(chan *Message, BUFFERED_CHAN_SIZE)

		n.msgsToSend = make(chan *Message, BUFFERED_CHAN_SIZE)
		n.msgsToForward = make(chan *Message, BUFFERED_CHAN_SIZE)

		n.deadNodes = make(chan uint32, BUFFERED_CHAN_SIZE)

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
	return <-n.msgsFromUserToUser
}

func (n *Node) ReceiveMessage() *Message {
	return <-n.msgsFromUserToUser
}

func (n *Node) ForwardMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	n.msgsToForward <- msg
}

func (n *Node) SendMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	n.msgsToSend <- msg
}

func (n *Node) Addr() uint32 {
	return n.thisNode
}

func (n *Node) GetDeadNodeAddr() uint32 {
	return <-n.deadNodes
}

func (n *Node) maintainNetwork() {
	for {
		n.updateConnected()

		select {
		case umsg := <-n.udp.receivec:
			// Note: This must not block when sending user-defined
			// message to the msgsFromUserToUser or msgsFromUserToOther.
			n.processUDPMessage(umsg)

		case msg := <-n.msgsToForward:
			n.forwardMsg(msg, RIGHT)

		case msg := <-n.msgsToSend:
			n.addResender(msg, MSG_RESEND_INTERVAL)

		case ID := <-n.resenderTimedOut:
			if re, ok := n.resenders[ID]; ok {
				if re.triesLeft > 0 {
					if re.msg.Type == KICK {
						n.forwardMsg(re.msg, LEFT)
					} else {
						n.forwardMsg(re.msg, RIGHT)
					}
					re.triesLeft--
					re.timer.SafeReset(re.resendInterval)
				} else {
					n.removeResender(re)
					n.leftNode = 0
					n.rightNode = 0
					n.updateConnected()
				}
			}

		case <-n.aliveTimer.C:
			n.aliveTimer.Seen()
			if n.connected {
				n.sendData(n.leftNode, ALIVE, nil)
				n.aliveTimer.SafeReset(ALIVETIME)
			}

		case <-n.kickTimer.C:
			n.kickTimer.Seen()
			if n.connected {
				kickMsg := NewMessage(KICK, &KickData{
					deadNode:   n.rightNode,
					senderNode: n.thisNode,
				})
				n.addResender(kickMsg, KICK_RESEND_INTERVAL)
				n.kickTimer.SafeReset(KICKTIME)
				select {
				case n.deadNodes <- n.rightNode:
				default:
				}
			}

		case <-n.broadcastTimer.C:
			n.broadcastTimer.Seen()
			if !n.connected {
				n.sendData(n.anyNode, BROADCAST, nil)
				n.broadcastTimer.SafeReset(BROADCASTTIME)
			}

		case <-n.stopc:
			n.running = false
			for _, re := range n.resenders {
				n.removeResender(re)
			}
			return
		}
	}
}

func (n *Node) processUDPMessage(umsg *UDPMessage) {
	msg := new(Message)
	msg.Decode(umsg.payload[:])

	if msg.ReadCount > MAX_READ_COUNT {
		return
	}
	msg.ReadCount++

	switch msg.Type {
	case BROADCAST:
		if umsg.from != n.thisNode {
			var hd HelloData
			hd.possibleNewLeft = n.thisNode
			if n.connected {
				hd.possibleNewRight = n.rightNode
			} else {
				hd.possibleNewRight = n.thisNode
			}

			if !n.connected {
				time.Sleep(LONELY_DELAY)
			}
			n.sendData(umsg.from, HELLO, &hd)
		}

	case HELLO:
		if !n.connected {
			var hd HelloData
			hd.Decode(msg.Data[:])

			n.rightNode = hd.possibleNewRight
			n.leftNode = hd.possibleNewLeft
			n.updateConnected()

			var add AddData
			if hd.possibleNewRight == hd.possibleNewLeft {
				add.asRight = 1
				add.asLeft = 1
				n.sendData(n.leftNode, ADD, &add)
			} else {
				add.asRight = 0
				add.asLeft = 1
				n.sendData(n.rightNode, ADD, &add)

				add.asRight = 1
				add.asLeft = 0
				n.sendData(n.leftNode, ADD, &add)
			}
		}

	case ADD:
		var add AddData
		add.Decode(msg.Data[:])
		if add.asRight == 1 {
			n.rightNode = umsg.from
		}
		if add.asLeft == 1 {
			n.leftNode = umsg.from
		}
		n.updateConnected()

	case ALIVE:
		if n.connected && umsg.from == n.rightNode {
			n.kickTimer.SafeReset(KICKTIME)
		}

	case KICK:
		if n.connected {
			var kick KickData
			kick.Decode(msg.Data[:])

			if re, ok := n.resenders[msg.ID]; ok {
				n.rightNode = umsg.from
				n.removeResender(re)
				n.kickTimer.SafeReset(KICKTIME)
			} else {
				if kick.deadNode == n.leftNode {
					n.leftNode = kick.senderNode
				}
				n.forwardMsg(msg, LEFT)
				n.deadNodes.Add(kick.deadNode)
			}
		}
	}

	// User-defined message type
	if msg.Type >= 5 {
		if n.connected && umsg.from == n.leftNode {
			var c chan *Message
			if re, ok := n.resenders[msg.ID]; ok {
				c = n.msgsFromUserToUser
				n.removeResender(re)
			} else {
				c = n.msgsFromUserToOther
			}

			select {
			case c <- msg: // Do not block if buffer is full.
			default:
			}
		}
	}
}

func (n *Node) addResender(msg *Message, resendInterval time.Duration) {
	re := &Resender{
		msg:       msg,
		timer:     NewSafeTimer(resendInterval),
		triesLeft: MAX_RESEND_COUNT,
		stopc:     make(chan struct{}),
	}
	n.resenders[msg.ID] = re

	// Goroutine for sending resender to n.resenderTimedOut on time out.
	// Remember to close re.stopc when removing a resender.
	//
	// Potential race: this goroutine has a pointer to a resender after it may
	// have been removed. This may result in a second call to removeResender
	// trying to close a closed channel.
	//
	// Solution: send msg.ID instead and let the receiver lookup the resender.
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
		data.Encode(umsg.payload[:])
	}
	n.udp.Send(umsg)
}

func (n *Node) forwardMsg(msg *Message, direction MsgDirection) {
	umsg := new(UDPMessage)
	umsg.from = n.thisNode
	if direction == RIGHT {
		umsg.to = n.rightNode
	} else {
		umsg.to = n.leftNode
	}
	msg.Encode(umsg.payload[:])
	n.udp.Send(umsg)
}

func (n *Node) updateConnected() {
	if n.rightNode == 0 || n.leftNode == 0 {
		n.rightNode = 0
		n.leftNode = 0
		n.connected = false
		n.broadcastTimer.SafeReset(BROADCASTTIME)
	} else if !n.connected {
		n.connected = true
		n.aliveTimer.SafeReset(ALIVETIME)
		n.kickTimer.SafeReset(KICKTIME)
	}
}
