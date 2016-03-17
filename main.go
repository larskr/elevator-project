package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	
	"elevator-project/elev"
	"elevator-project/network"
)

func main() {
	conf, err := loadConfig("./config")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	elev.LoadConfig(&conf.elevator)
	network.LoadConfig(&conf.network)

	elev.Init()
	
	node := network.NewNode()
	node.Start()
	
	panel := NewPanel()
	panel.Start()

	elevator := NewElevator(panel)
	elevator.Start()

	msgs := make(chan *network.Message)
	go receiveLoop(node, msgs)

	// Stop the elevator with Ctrl+C.
	interruptc := make(chan os.Signal)
	signal.Notify(interruptc, os.Interrupt)
	
	for {
		select {
		case req := <-panel.Requests:
			fmt.Printf("Request: floor %v, direction %v\n", req.Floor, req.Direction)
			//elevator.Add(req)
			if node.IsConnected() {
				sendData(node, PANEL, &panelData{
					floor: req.Floor,
					button: btnFromDir(req.Direction),
					on: true,
				})
				fmt.Println("Panel data sent.")
			}
		case <-interruptc:
			elev.SetMotorDirection(elev.Stop)
			os.Exit(1)
		case msg := <-msgs:
			var pd panelData
			unpackData(msg.Data, &pd)
			panel.SetLamp(pd.button, pd.floor, pd.on)
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

type config struct {
	elevator elev.Config
	network  network.Config
}

func loadConfig(filename string) (*config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	settings := make(map[string]string)
	
	r := bufio.NewReader(f)
	var line, prefix string
	for err != io.EOF {
		line, err = r.ReadString('\n')
		line = strings.TrimSpace(line)
		
		if len(line) == 0 {
			prefix = ""
			continue
		}
		
		if line[0] == '[' {
			i := 1
			for ; line[i] != ']'; i++ {}
			prefix = line[1:i]
		} else {
			i := 0
			for ; line[i] != ' ' && i < len(line); i++ {}
			field := line[:i]
			for ; line[i] != '=' && i < len(line); i++ {}
			val := strings.TrimSpace(line[i+1:])
			if prefix != "" {
				str := fmt.Sprintf("%v.%v", prefix, field)
				settings[str] = val
			} else {
				settings[field] = val
			}
		}
	}
	
	c := new(config)
	for field, valstr := range settings {
		switch field {
		case "elevator.motor_speed":
			c.elevator.MotorSpeed, _ = strconv.Atoi(valstr)
		case "elevator.use_simulator":
			if valstr == "true" {
				c.elevator.UseSimulator = true
			}
		case "elevator.simulator_port":
			c.elevator.SimulatorPort, _ = strconv.Atoi(valstr)
		case "elevator.simulator_ip":
			c.elevator.SimulatorIP = valstr
		case "network.interface":
			c.network.Interface = valstr
		case "network.protocol":
			c.network.Protocol = valstr
		}
	}	
	
	return c, nil
}

