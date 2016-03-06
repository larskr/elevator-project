package network

import (
	"bytes"
	"encoding/binary"
	"erros"
	"io"
	"math/rand"
	"time"
)

const (
	ALIVETIME           = 50 * time.Millisecond
	KICKTIME            = 250 * time.Millisecond
	BROADCASTTIME       = 500 * time.Millisecond
	MSG_RESEND_INTERVAL = 200 * time.Millisecond
)

const (
	MAX_RESENDERS      = 100
	MAX_DATA_SIZE      = 240
	BUFFERED_CHAN_SIZE = 32
)

type Addr uint32

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
	Type      uint32
	ReadCount uint32
	_         uint32 // padding
	Data      [MAX_DATA_SIZE]byte
}

func (msg *Message) Write(p []byte) (n int, err error) {
	if len(p) > MAX_DATA_SIZE {
		n = 0
		err = errors.New("Message data size is greater than MAX_DATA_SIZE.")
		return
	}
	n = copy(msg.Data[:], p)
}

func (umsg *Message) Read(p []byte) (n int, err error) {
	if len(p) > MAX_DATA_SIZE {
		err = io.EOF
	}
	n = copy(p, msg.Data[:])
}

type HelloData struct {
	possibleNewRight Addr
	possibleNewRight Addr
}

type AddData struct {
	asRight uint32
	asLeft  uint32
}

type KickData struct {
	deadNode   Addr
	senderNode Addr
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

	thisNode  Addr
	leftNode  Addr
	rightNode Addr
	anyNode   Addr

	udp *UDPService

	// Channels to receive user-defined messages.
	ReceiveMyMessage    chan *Message
	ReceiveOtherMessage chan *Message

	msgsToSend    chan *Message
	msgsToForward chan *Message

	deadNodes *Queue

	running bool
	stopc   chan struct{}

	aliveTimer     *time.Timer
	kickTimer      *time.Timer
	broadcastTimer *time.Timer

	resenders        [MAX_RESENDERS]*Resender
	resenderTimedOut chan *Resender
}

func NewNode() *NetworkNode {
	node := &NetworkNode{}
	return node
}

func (node *NetworkNode) Start() {
	if !running {
		var err error
		node.udp, err = NewUDPService()
		if err != nil {
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

		node.resenderTimedOut = make(chan *Resender, BUFFERED_CHAN_SIZE)
		node.lastResenderIndex = -1

		node.ReceiveMyMessage = make(chan *Message, BUFFERED_CHAN_SIZE)
		node.ReceiveOtherMessage = make(chan *Message, BUFFERED_CHAN_SIZE)

		node.msgsToSend = make(chan *Message, BUFFERED_CHAN_SIZE)
		node.msgsToForward = make(chan *Message, BUFFERED_CHAN_SIZE)

		node.deadNodes = NewQueue()

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
		return nil
	}
	node.msgsToForward <- msg
}

func (node *NetworkNode) SendMessage(msg *Message) {
	if msg.Type < 5 {
		return nil
	}
	node.msgsToSend <- msg
}

func (node *NetworkNode) Addr() Addr {
	return node.thisNode
}

func (node *NetworkNode) GetDeadNodeAddr() {
}

func (node *NetworkNode) maintainNetwork() {
	var aliveTimer, kickTimer, broadcastTimer <-chan timer.Time
	for {
		node.updateConnected()

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
		case msg := <-udp.receivec:
			node.processUDPMessage(msg)

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
			node.sendData(node.leftNode, ALIVE, nil)
			node.aliveTimer.Reset(ALIVETIME)

		case <-kickTimer:
			kickMsg := node.createKickMsg()
			node.addResender(kickMsg, KICK_RESEND_INTERVAL)
			node.kickTimer.Reset(KICKTIME)
			node.deadNodes.Add(node.rightNode)

		case <-broadcastTimer:
			node.sendData(node.anyNode, BROADCAST, nil)
			node.broadcastTimer.Reset(BROADCASTTIME)

		case <-node.stopc:
			node.running = false
			return
		default:

		}
	}
}

func (node *NetworkNode) processUDPMessage(umsg *UDPMessage) {

}

func (node *NetworkNode) sendData(to Addr, typ uint32, data interface{}) {
	umsg := &UDPMessage{
		to:   to,
		from: node.thisNode,
	}

	msg := Message{
		ID:        rand.Uint32(),
		Type:      typ,
		ReadCount: 0,
	}

	if data != nil {
		binary.Write(msg, binary.BigEndian, data)
	}
	binary.Write(umsg, binary.BigEndian, &msg)

	udp.Send(umsg)
}

func (node *NetworkNode) forwardMsg(msg *Message, direction int) {
	umsg := new(UDPMessage)
	umsg.from = node.thisNode
	if direction == RIGHT {
		umsg.to = node.rightNode
	} else {
		umsg.to = node.leftNode
	}
	binary.Write(umsg, binary.BigEndian, msg)

	udp.Send(umsg)
}

func (node *NetworkNode) updateConnected() {
	if node.rightNode == 0 || node.leftNode == 0 {
		node.rightNode = 0
		node.leftNode = 0
		node.connected = false
	} else if !node.connected {
		node.connected = true
		node.aliveTimer.Reset(ALIVETIME)
		node.kickTimer.Reset(KICKTIME)
	}
}
