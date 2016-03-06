package network

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

const (
	ALIVETIME            = 50 * time.Millisecond
	KICKTIME             = 250 * time.Millisecond
	BROADCASTTIME        = 5000 * time.Millisecond
	MSG_RESEND_INTERVAL  = 200 * time.Millisecond
	KICK_RESEND_INTERVAL = 20 * time.Millisecond
	LONELY_DELAY         = 100 * time.Millisecond
	USER_RECV_DEADLINE   = time.Second
)

const (
	BUFFERED_CHAN_SIZE = 32
	MAX_DATA_SIZE      = 240
	MAX_RESEND_COUNT   = 5
	MAX_READ_COUNT     = 100
)

// Message types. User-defined message types must be greater than 4.
type MsgType uint32

const (
	BROADCAST MsgType = 0x0
	HELLO     MsgType = 0x1
	ADD       MsgType = 0x2
	KICK      MsgType = 0x3
	ALIVE     MsgType = 0x4
)

// Direction to send message.
type MsgDirection int

const (
	RIGHT MsgDirection = -1
	LEFT  MsgDirection = 1
)

type Message struct {
	ID        uint32
	Type      MsgType // sizeof(MsgType) = 4
	ReadCount uint32
	_         uint32 // padding
	Data      [MAX_DATA_SIZE]byte
}

type HelloData struct {
	possibleNewRight uint32
	possibleNewLeft  uint32
}

type AddData struct {
	asRight uint32
	asLeft  uint32
}

type KickData struct {
	deadNode   uint32
	senderNode uint32
}

type Resender struct {
	msg            *Message
	timer          *time.Timer
	resendInterval time.Duration
	triesLeft      int
	stopc          chan struct{}
}

type NetworkNode struct {
	connected bool

	thisNode  uint32
	leftNode  uint32
	rightNode uint32
	anyNode   uint32

	udp *UDPService

	// Channels to receive user-defined messages.
	ReceiveMyMessage    chan *Message
	ReceiveOtherMessage chan *Message

	msgsToSend    chan *Message
	msgsToForward chan *Message

	deadNodes *Queue // of type uint32

	running bool
	stopc   chan struct{}

	aliveTimer     *time.Timer
	kickTimer      *time.Timer
	broadcastTimer *time.Timer

	// Note: The map datatype in Go is not thread-safe. In this
	// case access is controlled by the for/select loop in maintainNetwork.
	resenders        map[uint32]*Resender
	resenderTimedOut chan *Resender
}

func (node *NetworkNode) Start() {
	if !node.running {
		var err error
		node.udp, err = NewUDPService()
		if err != nil {
			fmt.Println(err)
			return
		}

		node.thisNode = NetworkAddr()
		node.anyNode = BroadcastAddr()
		if node.thisNode == 0 && node.anyNode == 0 {
			return
		}

		node.aliveTimer = time.NewTimer(ALIVETIME)
		node.kickTimer = time.NewTimer(KICKTIME)
		node.broadcastTimer = time.NewTimer(BROADCASTTIME)

		node.resenders = make(map[uint32]*Resender)
		node.resenderTimedOut = make(chan *Resender, BUFFERED_CHAN_SIZE)

		node.ReceiveMyMessage = make(chan *Message, BUFFERED_CHAN_SIZE)
		node.ReceiveOtherMessage = make(chan *Message, BUFFERED_CHAN_SIZE)

		node.msgsToSend = make(chan *Message, BUFFERED_CHAN_SIZE)
		node.msgsToForward = make(chan *Message, BUFFERED_CHAN_SIZE)

		node.deadNodes = NewQueue()

		node.stopc = make(chan struct{})

		go node.maintainNetwork()
		node.running = true
	}
}

func (node *NetworkNode) IsRunning() bool {
	return node.running
}

func (node *NetworkNode) IsConnected() bool {
	return node.connected
}

func (node *NetworkNode) Stop() {
	close(node.stopc)
	node.running = false
}

func (node *NetworkNode) ForwardMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	node.msgsToForward <- msg
}

func (node *NetworkNode) SendMessage(msg *Message) {
	if msg.Type < 5 {
		return
	}
	node.msgsToSend <- msg
}

func (node *NetworkNode) Addr() uint32 {
	return node.thisNode
}

func (node *NetworkNode) GetDeadNodeAddr() uint32 {
	return node.deadNodes.Pop().(uint32)
}

func (node *NetworkNode) maintainNetwork() {
	var aliveTimer, kickTimer, broadcastTimer <-chan time.Time
	for {
		node.updateConnected()

		// Remember reading from nil channels in Go always blocks.
		if node.connected {
			aliveTimer = node.aliveTimer.C
			kickTimer = node.kickTimer.C
			broadcastTimer = nil
		} else {
			aliveTimer = nil
			kickTimer = nil
			broadcastTimer = node.broadcastTimer.C
		}

		select {
		case umsg := <-node.udp.receivec:
			// Note: This must not block when sending user-defined
			// message to the ReceiveMyMessage or ReceiveOtherMessage.
			if umsg.from != node.thisNode {
				fmt.Println(umsg)
			}
			node.processUDPMessage(umsg)

		case msg := <-node.msgsToForward:
			node.forwardMsg(msg, RIGHT)

		case msg := <-node.msgsToSend:
			node.addResender(msg, MSG_RESEND_INTERVAL)

		case re := <-node.resenderTimedOut:
			if re.triesLeft > 0 {
				if re.msg.Type == KICK {
					node.forwardMsg(re.msg, LEFT)
				} else {
					node.forwardMsg(re.msg, RIGHT)
				}
				re.triesLeft--
				re.timer.Reset(re.resendInterval)
			} else {
				node.removeResender(re)
				node.leftNode = 0
				node.rightNode = 0
				node.updateConnected()
			}

		case <-aliveTimer:
			node.sendData(node.leftNode, ALIVE, nil, 0)
			node.aliveTimer.Reset(ALIVETIME)

		case <-kickTimer:
			kickMsg := node.createKickMsg()
			node.addResender(kickMsg, KICK_RESEND_INTERVAL)
			node.kickTimer.Reset(KICKTIME)
			node.deadNodes.Add(node.rightNode)

		case <-broadcastTimer:
			node.sendData(node.anyNode, BROADCAST, nil, 0)
			node.broadcastTimer.Reset(BROADCASTTIME)

		case <-node.stopc:
			node.running = false
			for _, re := range node.resenders {
				node.removeResender(re)
			}
			return
		}
	}
}

func (node *NetworkNode) processUDPMessage(umsg *UDPMessage) {
	msg := new(Message)
	msg.ID = binary.BigEndian.Uint32(umsg.payload[:])
	msg.Type = MsgType(binary.BigEndian.Uint32(umsg.payload[4:]))
	msg.ReadCount = binary.BigEndian.Uint32(umsg.payload[8:])
	copy(msg.Data[:], umsg.payload[16:])

	if msg.ReadCount > MAX_READ_COUNT {
		return
	}
	msg.ReadCount++

	switch msg.Type {
	case BROADCAST:
		if umsg.from != node.thisNode {
			var data [8]byte
			var hd HelloData
			hd.possibleNewLeft = node.thisNode
			if node.connected {
				hd.possibleNewRight = node.rightNode
			} else {
				hd.possibleNewRight = node.thisNode
			}
			binary.BigEndian.PutUint32(data[:], uint32(hd.possibleNewRight))
			binary.BigEndian.PutUint32(data[4:], uint32(hd.possibleNewLeft))

			if !node.connected {
				time.Sleep(LONELY_DELAY)
			}
			node.sendData(umsg.from, HELLO, data[:], len(data))
		}

	case HELLO:
		if !node.connected {
			var hd HelloData
			hd.possibleNewRight = binary.BigEndian.Uint32(msg.Data[:])
			hd.possibleNewLeft = binary.BigEndian.Uint32(msg.Data[4:])

			node.rightNode = hd.possibleNewRight
			node.leftNode = hd.possibleNewLeft
			node.updateConnected()

			var data [8]byte
			var ad AddData
			if hd.possibleNewRight == hd.possibleNewLeft {
				ad.asRight = 1
				ad.asLeft = 1
				binary.BigEndian.PutUint32(data[:], ad.asRight)
				binary.BigEndian.PutUint32(data[4:], ad.asLeft)
				node.sendData(node.leftNode, ADD, data[:], len(data))
			} else {
				ad.asRight = 0
				ad.asLeft = 1
				binary.BigEndian.PutUint32(data[:], ad.asRight)
				binary.BigEndian.PutUint32(data[4:], ad.asLeft)
				node.sendData(node.rightNode, ADD, data[:], len(data))

				ad.asRight = 1
				ad.asLeft = 0
				binary.BigEndian.PutUint32(data[:], ad.asRight)
				binary.BigEndian.PutUint32(data[4:], ad.asLeft)
				node.sendData(node.leftNode, ADD, data[:], len(data))
			}
		}

	case ADD:
		var add AddData
		add.asRight = binary.BigEndian.Uint32(msg.Data[:])
		add.asLeft = binary.BigEndian.Uint32(msg.Data[4:])
		if add.asRight == 1 {
			node.rightNode = umsg.from
		}
		if add.asLeft == 1 {
			node.leftNode = umsg.from
		}
		node.updateConnected()

	case ALIVE:
		if node.connected && umsg.from == node.rightNode {
			node.kickTimer.Reset(KICKTIME)
		}

	case KICK:
		if node.connected {
			var kick KickData
			kick.deadNode = binary.BigEndian.Uint32(msg.Data[:])
			kick.senderNode = binary.BigEndian.Uint32(msg.Data[4:])

			if re, ok := node.resenders[msg.ID]; ok {
				node.rightNode = umsg.from
				node.removeResender(re)
				node.kickTimer.Reset(KICKTIME)
			} else {
				if kick.deadNode == node.leftNode {
					node.leftNode = kick.senderNode
				}
				node.forwardMsg(msg, LEFT)
				node.deadNodes.Add(kick.deadNode)
			}
		}
	}

	// User-defined message type
	if msg.Type >= 5 {
		if node.connected && umsg.from == node.leftNode {
			var c chan *Message
			if re, ok := node.resenders[msg.ID]; ok {
				c = node.ReceiveMyMessage
				node.removeResender(re)
			} else {
				c = node.ReceiveOtherMessage
			}

			// Do not block if the user doesn't process messages.
			go func(c chan *Message, m *Message) {
				timeOut := time.After(USER_RECV_DEADLINE)
				for {
					select {
					case <-timeOut:
						return
					case c <- m:
						return
					}
				}
			}(c, msg)
		}
	}
}

func (node *NetworkNode) createKickMsg() *Message {
	msg := &Message{
		ID:        rand.Uint32(),
		Type:      KICK,
		ReadCount: 0,
	}

	kd := KickData{
		deadNode:   node.rightNode,
		senderNode: node.thisNode,
	}
	binary.BigEndian.PutUint32(msg.Data[:], kd.deadNode)
	binary.BigEndian.PutUint32(msg.Data[4:], kd.senderNode)

	return msg
}

func (node *NetworkNode) addResender(msg *Message, resendInterval time.Duration) {
	re := &Resender{
		msg:       msg,
		timer:     time.NewTimer(resendInterval),
		triesLeft: MAX_RESEND_COUNT,
		stopc:     make(chan struct{}),
	}
	node.resenders[msg.ID] = re

	// Goroutine for sending resender to node.resenderTimedOut on time out.
	// Remember to close re.stopc when removing a resender.
	go func(n *NetworkNode, r *Resender) {
		for {
			select {
			case <-r.stopc:
				return
			default:
				<-r.timer.C
				n.resenderTimedOut <- r
			}
		}
	}(node, re)
}

func (node *NetworkNode) removeResender(re *Resender) {
	// Stop forwarding goroutine.
	close(re.stopc)
	delete(node.resenders, re.msg.ID)
}

func (node *NetworkNode) sendData(to uint32, mtype MsgType, data []byte, size int) {
	umsg := &UDPMessage{
		to:   to,
		from: node.thisNode,
	}

	binary.BigEndian.PutUint32(umsg.payload[:], rand.Uint32())
	binary.BigEndian.PutUint32(umsg.payload[4:], uint32(mtype))
	if data != nil {
		copy(umsg.payload[16:], data[:size])
	}

	fmt.Println(umsg)
	node.udp.Send(umsg)
}

func (node *NetworkNode) forwardMsg(msg *Message, direction MsgDirection) {
	umsg := new(UDPMessage)
	umsg.from = node.thisNode
	if direction == RIGHT {
		umsg.to = node.rightNode
	} else {
		umsg.to = node.leftNode
	}

	binary.BigEndian.PutUint32(umsg.payload[:], msg.ID)
	binary.BigEndian.PutUint32(umsg.payload[4:], uint32(msg.Type))
	binary.BigEndian.PutUint32(umsg.payload[8:], msg.ReadCount)
	copy(umsg.payload[16:], msg.Data[:])

	fmt.Println(umsg)
	node.udp.Send(umsg)
}

func (node *NetworkNode) updateConnected() {
	if node.rightNode == 0 || node.leftNode == 0 {
		node.rightNode = 0
		node.leftNode = 0
		node.connected = false
		//node.broadcastTimer.Reset(BROADCASTTIME)
	} else if !node.connected {
		node.connected = true
		node.aliveTimer.Reset(ALIVETIME)
		node.kickTimer.Reset(KICKTIME)
	}
}
