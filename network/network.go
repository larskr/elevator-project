// The network package implements a circular overlay network which
// maintains itself and allow new nodes to connect. Each node keeps
// track of its left and right neighbour as well as its second neighbour
// on the left. This allows for easy maintainence of the circular
// overlay network. The network can recover from the simultaneous loss
// of multiple nonconsecutive nodes.
package network

import (
	"errors"
	"log"
	"fmt"
	"math/rand"
	"time"
)

const (
	aliveTime          = 50 * time.Millisecond
	kickTime           = 250 * time.Millisecond
	lonelyTime         = 500 * time.Millisecond // => 2 * kickTime
	broadcastTime      = 500 * time.Millisecond
	msgResendInterval  = 200 * time.Millisecond
	kickResendInterval = 20 * time.Millisecond
	lonelyDelay        = 100 * time.Millisecond
)

const (
	bufferSize     = 32
	maxDataLength  = 248
	maxResendCount = 5
	maxReadCount   = 100
	maxResenders   = 100
)

// Message types. User-defined message types must be >= 16.
// 0-15 are reserved.
const (
	BROADCAST = 0x0 // Announce that node is ready to connect.
	HELLO     = 0x1 // Reply to broadcasting node with new possible links.
	UPDATE    = 0x2 // Update links on neighbouring nodes.
	GET       = 0x3 // Request for UPDATE of left2ndNode.
	PING      = 0x4 // Check that node is alive.
	ALIVE     = 0x5 // Reply to PING.
	KICK      = 0x6 // Inform network that a node has been kicked.
)

// The Message type is what is packed into the UDP datagram and sent
// through the network. The Data slice indicates what region of buf
// that should be sent.
type Message struct {
	ID        uint32
	Type      uint8
	ReadCount uint8
	_         uint16
	buf       [maxDataLength]byte

	Data []byte // points into buf
}

// NewMessage allocates and initializes a Message copying from the data
// slice.
func NewMessage(mtype uint8, data []byte) *Message {
	msg := &Message{ID: rand.Uint32(), Type: uint8(mtype)}
	n := 0
	if data != nil {
		n = copy(msg.buf[:], data)
	}
	msg.Data = msg.buf[:n]
	return msg
}

type helloData struct {
	newRight   Addr
	newLeft    Addr
	newLeft2nd Addr
}

type updateData struct {
	right   Addr
	left    Addr
	left2nd Addr
}

type kickData struct {
	deadNode   Addr
	senderNode Addr
}

type resender struct {
	msg            *Message
	timer          *SafeTimer
	resendInterval time.Duration
	triesLeft      int
	stopc          chan struct{}
}

type nodeState int

const (
	connected nodeState = iota
	disconnected
	stopped
	detached2ndLeft
	ready
)

type Node struct {
	state nodeState
	stopc chan struct{}

	// Only thisNode and anyNode are guaranteed to be nonnil at all times.
	thisNode    Addr
	leftNode    Addr
	left2ndNode Addr
	rightNode   Addr
	anyNode     Addr

	udp *UDPService

	// Channels are for user-defined messages. They are buffered and
	// when they are full new messages will be dropped.
	fromUserToUser  chan *Message
	fromUserToOther chan *Message
	toSend          chan *Message
	toForward       chan *Message

	deadNodes chan Addr

	// Timers for keeping track of the two next nodes on the left.
	aliveTimer     *SafeTimer
	kickTimer      *SafeTimer
	leftIsAlive    bool
	left2ndIsAlive bool

	// If node is kicked without knowing it (it may have been busy
	// and not able to respond within kickTime) this timer will
	// expire and the node will disconnect.
	lonelyTimer *SafeTimer
	
	broadcastTimer *SafeTimer

	// Note: The map datatype in Go is not thread-safe. In this
	// case access is controlled by the for/select loop in maintainNetwork.
	resenders        map[uint32]*resender
	resenderTimedOut chan uint32
}

func NewNode() *Node {
	n := new(Node)
	n.resenders = make(map[uint32]*resender)
	n.resenderTimedOut = make(chan uint32, maxResenders)

	n.fromUserToUser = make(chan *Message, bufferSize)
	n.fromUserToOther = make(chan *Message, bufferSize)
	n.toSend = make(chan *Message, bufferSize)
	n.toForward = make(chan *Message, bufferSize)

	n.deadNodes = make(chan Addr, bufferSize)

	n.stopc = make(chan struct{})

	n.updateState(ready)
	return n
}

func (n *Node) Start() error {
	if n.state == ready {
		var err error
		n.thisNode, err = NetworkAddr()
		if err != nil {
			return err
		}

		n.anyNode, err = BroadcastAddr()
		if err != nil {
			return err
		}

		n.udp, err = NewUDPService()
		if err != nil {
			return err
		}
		n.aliveTimer = NewSafeTimer(aliveTime)
		n.kickTimer = NewSafeTimer(kickTime)
		n.lonelyTimer = NewSafeTimer(lonelyTime)
		n.broadcastTimer = NewSafeTimer(broadcastTime)

		go n.maintainNetwork()
		n.updateState(disconnected)

		log.Printf("Node is running on %v.\n", n.thisNode)
	}
	return nil
}

func (n *Node) IsRunning() bool {
	return n.state != stopped
}

func (n *Node) IsConnected() bool {
	return n.state == connected || n.state == detached2ndLeft
}

func (n *Node) Stop() {
	close(n.stopc)
}

func (n *Node) ReceiveMyMessage() *Message {
	return <-n.fromUserToUser
}

func (n *Node) ReceiveMessage() *Message {
	return <-n.fromUserToUser
}

func (n *Node) ForwardMessage(msg *Message) {
	if msg.Type < 16 {
		return
	}
	n.toForward <- msg
}

func (n *Node) SendMessage(msg *Message) {
	if msg.Type < 16 {
		return
	}
	n.toSend <- msg
}

func (n *Node) Addr() Addr {
	return n.thisNode
}

func (n *Node) GetDeadNode() Addr {
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
					n.updateState(disconnected)
				}
			}

		case <-n.aliveTimer.C:
			n.aliveTimer.Seen()
			if n.state == connected {
				n.sendData(n.leftNode, PING, nil)
				if n.state != detached2ndLeft &&
					n.left2ndNode != n.thisNode {
					n.sendData(n.left2ndNode, PING, nil)
				}
				n.kickTimer.SafeReset(kickTime)
			}

		case <-n.kickTimer.C:
			n.kickTimer.Seen()
			if n.state == connected {
				if n.leftIsAlive {
					// Since kick timer expired left2ndNode
					// must be dead, but that is leftNode's
					// responsibility so we don't care.
					n.aliveTimer.SafeReset(aliveTime)
				} else {
					// Either one or both of the
					// other nodes are dead. See
					// if we can restore the
					// connection.
					n.restoreNetwork()
				}
				n.leftIsAlive = false
				n.left2ndIsAlive = false
			}

		case <-n.lonelyTimer.C:
			n.lonelyTimer.Seen()
			if n.state == connected {
				// We are not receiving pings from other nodes.
				n.updateState(disconnected)
			}

		case <-n.broadcastTimer.C:
			n.broadcastTimer.Seen()
			if n.state == disconnected {
				n.sendData(n.anyNode, BROADCAST, nil)
				n.broadcastTimer.SafeReset(broadcastTime)
			}

		case <-n.stopc:
			n.state = stopped
			for _, re := range n.resenders {
				n.removeResender(re)
			}
			return
		}
	}
}

func (n *Node) restoreNetwork() error {
	if !n.leftIsAlive && !n.left2ndIsAlive {
		// Both nodes on the left are dead. Must disconnect.
		//
		// This case also covers the cases where leftNode is
		// dead and either:
		//  (a)  n.state == detached2ndLeft
		// or
		//  (b)  n.left2ndNode == n.thisNode
		// is true.
		//
		// This is true because left2ndNode is never pinged if
		// either of these cases are true, so left2ndIsAlive is false
		// since it is not updated. Also restoreNetwork is only called
		// when leftIsAlive is false.
		//
		// Case (b) happens when there are only two nodes in the
		// network and case (a) when left2ndNode has not yet been updated
		// by leftNode. Thus since leftIsAlive is false there is
		// no way of recovering and we have no choice but to
		// disconnect.
		n.updateState(disconnected)
		return errors.New("Not able to restore connectivity.")
	} else if !n.leftIsAlive && n.left2ndIsAlive {
		// Easy removal of dead node is possible.
		deadNode := n.leftNode
		n.leftNode = n.left2ndNode
		n.sendData(n.rightNode, UPDATE, &updateData{
			left2nd: n.left2ndNode,
		})
		n.sendData(n.left2ndNode, UPDATE, &updateData{
			right: n.thisNode,
		})
		// This leaves n.left2ndNode incorrectly pointing to
		// n.leftNode. The link to the 2nd node is not critical
		// for forwarding messages, but we can't start pinging it
		// before it is set correctly. One solution is to set
		// it to zero and don't ping untill it is set by an UPDATE.
		n.updateState(detached2ndLeft)
		n.sendData(n.leftNode, GET, nil)

		// Create and send a kick message.
		var data [32]byte
		kd := kickData{
			deadNode:   deadNode,
			senderNode: n.thisNode,
		}
		Pack(data[:], "16b16b", kd.deadNode[:], n.thisNode[:])
		n.addResender(NewMessage(KICK, data[:]), kickResendInterval)

		n.aliveTimer.SafeReset(aliveTime)
		return nil
	} else {
		// This should never happen.
		log.Fatalf("Kick timer expired and was not handeled.")
	}
	return errors.New("Not able to restore connectivity.")
}

func (n *Node) processUDPMessage(umsg *UDPMessage) {
	msg := new(Message)
	Unpack(umsg.payload, "ubb", &msg.ID, &msg.Type, &msg.ReadCount)
	nc := copy(msg.buf[:], umsg.payload[8:])
	msg.Data = msg.buf[:nc]

	if msg.ReadCount > maxReadCount {
		return
	}
	msg.ReadCount++

	switch msg.Type {
	case BROADCAST:
		if umsg.from != n.thisNode {
			var hd helloData
			if n.state == connected {
				hd.newRight = n.rightNode
				hd.newLeft = n.thisNode
				hd.newLeft2nd = n.leftNode
			} else {
				hd.newRight = n.thisNode
				hd.newLeft = n.thisNode
				hd.newLeft2nd = umsg.from
			}

			// Avoid forming disjoint networks at statup.
			if n.state == disconnected {
				time.Sleep(lonelyDelay)
			}
			n.sendData(umsg.from, HELLO, &hd)
		}

	case HELLO:
		if n.state == disconnected {
			var hd helloData
			Unpack(msg.Data, "16b16b16b",
				hd.newRight[:], hd.newLeft[:], hd.newLeft2nd[:])

			n.rightNode = hd.newRight
			n.leftNode = hd.newLeft
			n.left2ndNode = hd.newLeft2nd
			n.updateState(connected)

			if hd.newRight == hd.newLeft {
				// two disconnected nodes are connecting
				n.sendData(n.leftNode, UPDATE, &updateData{
					right:   n.thisNode,
					left:    n.thisNode,
					left2nd: umsg.from,
				})
			} else if hd.newRight == hd.newLeft2nd {
				// connecting to a connected doublet
				n.sendData(n.rightNode, UPDATE, &updateData{
					left:    n.thisNode,
					left2nd: n.leftNode,
				})
				n.sendData(n.leftNode, UPDATE, &updateData{
					right: n.thisNode,
					left2nd: n.thisNode,
				})
			} else {
				n.sendData(n.rightNode, UPDATE, &updateData{
					left:    n.thisNode,
					left2nd: n.leftNode,
				})
				n.sendData(n.leftNode, UPDATE, &updateData{
					right: n.thisNode})
			}
			// This connection protocol works perfectly if
			// all messages are received.  TODO: What
			// happens if one or both UPDATE messages
			// never arrive? Maybe this could be solved by
			// using an acknowledge message to respond to
			// updates when a node is connecting.
		}

	case UPDATE:
		var ud updateData
		Unpack(msg.Data, "16b16b16b",
			ud.right[:], ud.left[:], ud.left2nd[:])

		if !ud.right.IsZero() {
			n.rightNode = ud.right
		}
		if !ud.left.IsZero() {
			n.leftNode = ud.left
		}
		if !ud.left2nd.IsZero() {
			n.left2ndNode = ud.left2nd
		}

		// A disconnected node should not receive an UPDATE
		// message unless when two disconnected nodes are
		// connecting. This change also covers the case where
		// a node in the detached2ndLeft state receives an UPDATE.
		n.updateState(connected)

	case GET:
		if n.state == connected {
			n.sendData(umsg.from, UPDATE,
				&updateData{left2nd: n.leftNode})
		}

	case PING:
		if n.state == connected {
			n.sendData(umsg.from, ALIVE, nil)
			n.lonelyTimer.SafeReset(lonelyTime)
		}

	case ALIVE:
		if n.state == connected {
			if umsg.from == n.leftNode {
				n.leftIsAlive = true
			} else if umsg.from == n.left2ndNode {
				n.left2ndIsAlive = true
			}

			if n.leftIsAlive && n.left2ndIsAlive {
				// all good
				if ok := n.kickTimer.Stop(); !ok {
					// This should never happen.
					log.Fatalf("Kick timer has been stopped"+
						" after it expired.\n")
				}
				n.aliveTimer.SafeReset(aliveTime)
				n.leftIsAlive = false
				n.left2ndIsAlive = false
			}
		}

	case KICK:
		if n.state == connected {
			var kick kickData
			Unpack(msg.Data, "16b16b", kick.deadNode[:],
				kick.senderNode[:])

			select {
			case n.deadNodes <- kick.deadNode:
			default:
			}

			n.forwardMsg(msg)
		}
	}

	// User-defined message type
	if msg.Type >= 16 {
		if n.state == connected && umsg.from == n.rightNode {
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
	re := &resender{
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

func (n *Node) removeResender(re *resender) {
	// Stop forwarding goroutine.
	close(re.stopc)
	delete(n.resenders, re.msg.ID)
}

func (n *Node) sendData(to Addr, mtype uint8, data interface{}) {
	umsg := &UDPMessage{to: to, from: n.thisNode}
	Pack(umsg.buf[:], "ub3_", rand.Uint32(), mtype)
	umsg.payload = umsg.buf[:8]
	if data != nil {
		var np int
		switch d := data.(type) {
		case *helloData:
			np, _ = Pack(umsg.buf[8:], "16b16b16b",
				d.newRight[:], d.newLeft[:], d.newLeft2nd[:])
		case *updateData:
			np, _ = Pack(umsg.buf[8:], "16b16b16b",
				d.right[:], d.left[:], d.left2nd[:])
		case *kickData:
			np, _ = Pack(umsg.buf[8:], "16b16b",
				d.deadNode[:], d.senderNode[:])
		default:
			return
		}
		umsg.payload = umsg.buf[:np+8]
	}
	n.udp.Send(umsg)
}

func (n *Node) forwardMsg(msg *Message) {
	umsg := &UDPMessage{to: n.leftNode, from: n.thisNode}

	Pack(umsg.buf[:], "ubb", msg.ID, msg.Type, msg.ReadCount)
	nc := copy(umsg.buf[8:], msg.Data)

	umsg.payload = umsg.buf[:nc+8]
	n.udp.Send(umsg)
}

func (n *Node) updateState(s nodeState) {
	switch s {
	case connected:
		// sanity check
		if n.leftNode.IsZero() || n.rightNode.IsZero() || n.left2ndNode.IsZero() {
			log.Fatalf("Invalid node state.\n")
		}

		log.Printf("Node connected as:")
		fmt.Printf("                    %v -> \x1b[35m%v\x1b[m -> %v -> %v\n",
			n.rightNode, n.thisNode, n.leftNode, n.left2ndNode);
		
		n.state = connected
		n.aliveTimer.SafeReset(aliveTime)
		n.lonelyTimer.SafeReset(lonelyTime)
	case disconnected:
		// Setting these to zero should not be necessary, but
		// useful for debugging because we can detect if a
		// link is not updated when connecting.
		n.leftNode.SetZero()
		n.rightNode.SetZero()
		n.left2ndNode.SetZero()

		log.Printf("Node was disconnected.\n")
		
		n.state = disconnected
		n.broadcastTimer.SafeReset(broadcastTime)
	case detached2ndLeft:
		n.left2ndNode.SetZero()
		n.state = detached2ndLeft
	default:
		n.state = s
	}
}
