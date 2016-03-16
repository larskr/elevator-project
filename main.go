package main

import (
	"encoding/binary"
	"fmt"
	"os"
	
	"github.com/BurntSushi/toml"

	"elevator-project/elev"
	"elevator-project/network"
)

type tomlConfig struct {
	Elevator elev.Config
	Network  network.Config
}

func main() {
	var config tomlConfig
	if _, err := toml.DecodeFile("./config", &config); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	elev.LoadConfig(&config.Elevator)
	network.LoadConfig(&config.Network)

	elev.Init()
	
	node := network.NewNode()
	node.Start()
	
	panel := NewPanel()
	panel.Start()

	elevator := NewElevator(panel)
	elevator.Start()

	msgs := make(chan *network.Message)
	go receiveLoop(node, msgs)
	
	for {
		select {
		case req := <-panel.Requests:
			fmt.Printf("Request: floor %v, direction %v\n", req.Floor, req.Direction)
			sendData(node, PANEL, &panelData{
				floor: req.Floor,
				button: btnFromDir(req.Direction),
				on: true,
			})
			fmt.Println("Panel data sent.")
		case msg := <-msgs:
			var pd panelData
			unpackData(msg.Data, &pd)
			panel.Set(pd.button, pd.floor, pd.on)
			fmt.Println("Panel data received.")
			node.ForwardMessage(msg)
		}
	}
	
	os.Exit(0)
}

func receiveLoop(node *network.Node, msgs chan *network.Message) {
	for {
		msg := node.ReceiveMessage()
		msgs <- msg
	}
}

const (
	PANEL network.MsgType = 0x10
)

type panelData struct {
	floor     int
	button    elev.Button
	on        bool
}

func packData(p []byte, data interface{}) int {
	var n int
	switch d := data.(type) {
	case *panelData:
		binary.BigEndian.PutUint32(p[:], uint32(d.floor))
		binary.BigEndian.PutUint32(p[4:], uint32(d.button))
		if d.on {
			binary.BigEndian.PutUint32(p[8:], 1)
		} else {
			binary.BigEndian.PutUint32(p[8:], 0)
		}
		n = 12
	}
	return n
}

func unpackData(p []byte, data interface{}) {
	switch d := data.(type) {
	case *panelData:
		d.floor = int(binary.BigEndian.Uint32(p[:]))
		d.button = elev.Button(binary.BigEndian.Uint32(p[4:]))
		val := binary.BigEndian.Uint32(p[8:])
		if val == 1 {
			d.on = true
		}
	}
}

// sendData sends a message with the data and returns the message ID.
// The data struct to be sent must be implmented in packData/unpackData.
func sendData(node *network.Node, dtype network.MsgType, data interface{}) uint32 {
	var buf [network.MaxDataLength]byte

	n := packData(buf[:], data)
	msg := network.NewMessage(dtype, buf[:n])

	node.SendMessage(msg)
	return msg.ID
}
		


