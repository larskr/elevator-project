package main 

import (
	"fmt"
	"time"
	"network"
)

func main() {
	var ping_channel = make(chan network.Frame)
	var commands_channel = make(chan network.Frame)
	var remote_addr string
	var port int
	var current_floor int 
	var elevator_num int
	//initializing dispacher
	func InitializeElevator(){
		fmt.Println("Enter remote address.")
		fmt.Scanln(&remote_addr)
		fmt.Println("Enter port number.")
		fmt.Scanln(&port)
		fmt.Println("Enter elevator number.")
		fmt.Scanln(&elevator_num)
		network.Init(port,  ping_channel, commands_channel)
	}

	//pinging on every 3s
	go func (){
		for {
			timer = time.NewTimer(time.Second*3)
			<-timer.C
			//current_floor = elev_get_floor_sensor_signal()
			//var ping_message = "ping," + elevator_num + "," + current_floor
			var frame = Frame{
				raddr: remote_addr,
				Data:[]byte("ping message")
			}
			ping_channel <- frame
			fmt.Println("Ping message sent.")
		}
	}
	//pinging if floor has changed
	go func () {
		for {
			var new_floor = elev_get_floor_sensor_signal()
			if new_floor != current_floor {
				//var ping_message = "ping," + elevator_num + "," + current_floor
				var Frame = Frame(raddr: remote_addr, Data:"ping message", length:len(remote_addr) + len("ping message"/*ping_message*/))
				ping_channel <- Frame
				fmt.Println("Ping message sent.")
			}
		}
	}
	go func receive_commands() {
		for {
			var command <- commands_channel
			var command_type string;
			func resolve_command_type() {
				
			}
			switch command_type
			{
			case "stop":
				//stop elevator
				Elev_set_motor_direction(tag_elev_motor_direction.DIRN_STOP)
				Elev_set_door_open_lamp(floor)
				fmt.Println("Elevator has stoped. Waiting for commands.")
			case "go_up":
				//go up
				int floor
				Elev_set_motor_direction(tag_elev_motor_direction.DIRN_UP)
				Elev_set_floor_indicator(floor)
				fmt.Println("Elevator is going down - to the " + floor + ". floor.")
			case "go_down":
				//go down
				int floor
				Elev_set_motor_direction(tag_elev_motor_direction.DIRN_DOWN)
				Elev_set_floor_indicator(floor)
				fmt.Println("Elevator is going down - to the " + floor + ". floor.")
			}
		}
	}

}