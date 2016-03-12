package main

import (
	"fmt"
	"os"
	"time"
	
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

	elev.Init()
	time.Sleep(time.Second)
	elev.SetFloorIndicator(0)
	time.Sleep(2*time.Second)
	elev.SetFloorIndicator(1)
	time.Sleep(2*time.Second)
	elev.SetFloorIndicator(2)
	time.Sleep(2*time.Second)

	select{}
	
	os.Exit(0)
}
