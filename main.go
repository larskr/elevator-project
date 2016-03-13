package main

import (
	"fmt"
	"os"
	
	"github.com/BurntSushi/toml"

	"ring-network/elev"
	"ring-network/network"
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

	node := network.NewNode()
	node.Start()
	
	//e := new(Elevator)
	//e.Start()

	select { }
	
	os.Exit(0)
}
