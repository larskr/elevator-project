package main

import (
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
	if _, err := toml.DecodeFile("./config.toml", &config); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	elev.LoadConfig(&config.Elevator)
	network.LoadConfig(&config.Network)

	elev.Init()
	
	//node := network.NewNode()
	//node.Start()
	
	panel := new(Panel)
	panel.Start()

	elevator := NewElevator(panel)
	elevator.Start()

	//for !node.IsConnected() {
	//	time.Sleep(time.Second)
	//}

	//node.SendMessage(network.NewMessage(16, []byte("Hello, world.")))

	//node.ReceiveMyMessage()
	//fmt.Println("Got my message.")
	
	for {
		select {
		case req := <-panel.Requests:
			fmt.Printf("Request: floor %v, direction %v\n", req.Floor, req.Direction)
			elevator.Add(req)
		}
	}
	
	os.Exit(0)
}
