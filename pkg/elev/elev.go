package elev

import (
	"errors"
	"net"
	"sync"
	"strconv"
)

// Hardware constants
const (
	NumFloors = 4
)

type Direction int

const (
	Down Direction = -1
	Stop Direction = 0
	Up   Direction = 1
)

type Button int

const (
	CallUp   Button = 0
	CallDown Button = 1
	Command  Button = 2
)

var (
	lampMatrix = [NumFloors][3]int{
		{LIGHT_UP1, LIGHT_DOWN1, LIGHT_COMMAND1},
		{LIGHT_UP2, LIGHT_DOWN2, LIGHT_COMMAND2},
		{LIGHT_UP3, LIGHT_DOWN3, LIGHT_COMMAND3},
		{LIGHT_UP4, LIGHT_DOWN4, LIGHT_COMMAND4},
	}

	buttonMatrix = [NumFloors][3]int{
		{BUTTON_UP1, BUTTON_DOWN1, BUTTON_COMMAND1},
		{BUTTON_UP2, BUTTON_DOWN2, BUTTON_COMMAND2},
		{BUTTON_UP3, BUTTON_DOWN3, BUTTON_COMMAND3},
		{BUTTON_UP4, BUTTON_DOWN4, BUTTON_COMMAND4},
	}
)

var sim struct {
	mu   sync.Mutex
	conn *net.TCPConn
	buf  [4]byte
}

type Config struct {
	MotorSpeed    int
	UseSimulator  bool
	SimulatorPort int
	SimulatorIP   string
}

var config Config

func LoadConfig(conf map[string]string) {
	config.MotorSpeed, _ = strconv.Atoi(conf["elevator.motor_speed"])
	config.SimulatorPort, _ = strconv.Atoi(conf["elevator.simulator_port"])
	config.SimulatorIP = conf["elevator.simulator_ip"]
	if conf["elevator.use_simulator"] == "true" {
		config.UseSimulator = true
	}
}

func Init() error {
	if config.UseSimulator {
		addr := &net.TCPAddr{
			IP:   net.ParseIP(config.SimulatorIP),
			Port: config.SimulatorPort,
		}
		c, err := net.DialTCP("tcp4", nil, addr)
		if err != nil {
			return err
		}
		sim.mu.Lock()
		sim.conn = c
		//sim.conn.Write(sim.buf[:])
		sim.mu.Unlock()
		return nil
	}

	ret := InitIO()
	if ret == 0 {
		return errors.New("Unable to initalize elevator hardware.")
	}

	for f := 0; f < NumFloors; f++ {
		SetButtonLamp(CallUp, f, 0)
		SetButtonLamp(CallDown, f, 0)
		SetButtonLamp(Command, f, 0)
	}

	SetStopLamp(0)
	SetDoorOpenLamp(0)
	SetFloorIndicator(0)

	return nil
}

func SetMotorDirection(dir Direction) {
	if config.UseSimulator {
		switch dir {
		case Down:
			sim.conn.Write([]byte{1, 255, 0, 0})
		case Stop:
			sim.conn.Write([]byte{1, 0, 0, 0})
		case Up:
			sim.conn.Write([]byte{1, 1, 0, 0})
		}
		return
	}

	switch dir {
	case Stop:
		WriteAnalog(MOTOR, 0)
	case Up:
		ClearBit(MOTORDIR)
		WriteAnalog(MOTOR, config.MotorSpeed)
	case Down:
		SetBit(MOTORDIR)
		WriteAnalog(MOTOR, config.MotorSpeed)

	}
}

func SetButtonLamp(b Button, floor int, val int) {
	if config.UseSimulator {
		sim.conn.Write([]byte{2, byte(b), byte(floor), byte(val)})
		return
	}

	if val == 1 {
		SetBit(lampMatrix[floor][int(b)])
	} else {
		ClearBit(lampMatrix[floor][int(b)])
	}
}

func SetFloorIndicator(floor int) {
	if config.UseSimulator {
		sim.conn.Write([]byte{3, byte(floor), 0, 0})
		return
	}

	if floor&0x02 != 0 {
		SetBit(LIGHT_FLOOR_IND1)
	} else {
		ClearBit(LIGHT_FLOOR_IND1)
	}
	if floor&0x01 != 0 {
		SetBit(LIGHT_FLOOR_IND2)
	} else {
		ClearBit(LIGHT_FLOOR_IND2)
	}
}

func SetDoorOpenLamp(val int) {
	if config.UseSimulator {
		sim.conn.Write([]byte{4, byte(val), 0, 0})
		return
	}

	if val == 1 {
		SetBit(LIGHT_DOOR_OPEN)
	} else {
		ClearBit(LIGHT_DOOR_OPEN)
	}
}

func SetStopLamp(val int) {
	if config.UseSimulator {
		sim.conn.Write([]byte{5, byte(val), 0, 0})
		return
	}

	if val == 1 {
		SetBit(LIGHT_STOP)
	} else {
		ClearBit(LIGHT_STOP)
	}
}

func ReadButton(b Button, floor int) int {
	if config.UseSimulator {
		sim.mu.Lock()
		sim.conn.Write([]byte{6, byte(b), byte(floor), 0})
		sim.conn.Read(sim.buf[:])
		defer sim.mu.Unlock()
		return int(sim.buf[1])
	}

	return ReadBit(buttonMatrix[floor][int(b)])
}

func ReadFloorSensor() int {
	if config.UseSimulator {
		sim.mu.Lock()
		sim.conn.Write([]byte{7, 0, 0, 0})
		sim.conn.Read(sim.buf[:])
		defer sim.mu.Unlock()
		if sim.buf[1] == 1 {
			return int(sim.buf[2])
		}
		return -1
	}

	switch {
	case ReadBit(SENSOR_FLOOR1) == 1:
		return 0
	case ReadBit(SENSOR_FLOOR2) == 1:
		return 1
	case ReadBit(SENSOR_FLOOR3) == 1:
		return 2
	case ReadBit(SENSOR_FLOOR4) == 1:
		return 3
	default:
		return -1
	}
}

func ReadStopButton() int {
	if config.UseSimulator {
		sim.mu.Lock()
		sim.conn.Write([]byte{8, 0, 0, 0})
		sim.conn.Read(sim.buf[:])
		sim.mu.Unlock()
		return int(sim.buf[1])
	}

	return ReadBit(STOP)
}

func ReadObstruction() int {
	if config.UseSimulator {
		sim.mu.Lock()
		sim.conn.Write([]byte{9, 0, 0, 0})
		sim.conn.Read(sim.buf[:])
		sim.mu.Unlock()
		return int(sim.buf[1])
	}

	return ReadBit(OBSTRUCTION)
}
