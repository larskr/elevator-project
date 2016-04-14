package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"elevator-project/elev"
	"elevator-project/network"
)

type config struct {
	elevator elev.Config
	network  network.Config
	elevatorSocket string
	watchdogSocket string
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
			for ; line[i] != ']'; i++ {
			}
			prefix = line[1:i]
		} else {
			i := 0
			for ; line[i] != ' ' && i < len(line); i++ {
			}
			field := line[:i]
			for ; line[i] != '=' && i < len(line); i++ {
			}
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
		case "watchdog.elevator_socket":
			c.elevatorSocket = valstr
		case "watchdog.watchdog_socket":
			c.watchdogSocket = valstr
		}
	}

	return c, nil
}
