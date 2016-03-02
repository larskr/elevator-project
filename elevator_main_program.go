package main 

import (
	"fmt"
	"time"
	"network"
	"sync"
	"bytes"
	//"strconv"
	//"driver"
)

func main() {
	var ping_channel = make(chan network.Frame)
	var commands_channel = make(chan network.Frame)
	var remote_addr string
	var port int
	var current_floor int 
	var elevator_num int
	var mutexInit = &sync.Mutex{}
	//initializing dispacher
	go func (){
		mutexInit.Lock()
		fmt.Println("Enter remote address.")
		fmt.Scanln(&remote_addr)
		fmt.Println(remote_addr)
		fmt.Println("Enter port number.")
		fmt.Scanln(&port)
		fmt.Println(port)
		fmt.Println("Enter elevator number.")
		fmt.Scanln(&elevator_num)
		fmt.Println(elevator_num)
		err := network.Init(port,  ping_channel, commands_channel)
		if err != nil {
			fmt.Println("Elevator initialization failed.")
			fmt.Println(err)
		} else {
			fmt.Println("Elevator initialization successful.")
		}
		mutexInit.Unlock()
	}()
	timerInit := time.NewTimer(time.Second)
	<- timerInit.C
	mutexInit.Lock()
	fmt.Println("Elevator is set up with following parameters:")
	fmt.Printf("Remote address:					" + remote_addr + "\n")
	fmt.Printf("Port number:					%d\n", port)
	fmt.Printf("Elevator number:				%d\n", elevator_num)
	mutexInit.Unlock()
	//pinging on every 3s
	fmt.Println("Control point 1")
	go func (){
		fmt.Println("Regular pinging function")
		for {
			timer := time.NewTimer(time.Second*3)
			<- timer.C
			fmt.Println("Ping interval has expired(3s). Pinging...")
			//current_floor = elev_get_floor_sensor_signal()
			//var ping_message = "ping," + elevator_num + "," + current_floor
			var frame = network.Frame{
				RemoteAddr: remote_addr,
				Data: []byte("ping message"),
			}
			ping_channel <- frame
			fmt.Println("Ping message sent.")
		}
	}()
	fmt.Println("Control point 2")
	//pinging if floor has changed
	go func () {
		fmt.Println("Pinging function for changed floor")
		for {
			var new_floor = 0//Elev_get_floor_sensor_signal()
			if new_floor != current_floor {
				fmt.Println("Floor has changed, pinging...")
				//var ping_message = "ping," + elevator_num + "," + current_floor
				var Frame = network.Frame{
					RemoteAddr: remote_addr,
					Data: []byte("ping message"),
				}
				ping_channel <- Frame
				fmt.Println("Ping message sent.")
			}
		}
	}()
	fmt.Println("Control point 3")
	//receive commands
	go func () {
		fmt.Println("Receiving command...")
		for {
			var command network.Frame
			command = <- commands_channel
			var command_type string;
			
			n := bytes.IndexByte(command.Data, 0)
			command_type = string(command.Data[:n])

			switch
			{
			case command_type == "stop":
				//stop elevator
				{
					// Elev_set_motor_direction(driver.DIRN_STOP)
					// Elev_set_door_open_lamp(current_floor)
					fmt.Println("Elevator has stoped. Waiting for commands.")
				}
			case command_type == "go_up":
				//go up
				{
					//var floor int
					//Elev_set_motor_direction(driver.DIRN_UP)
					//Elev_set_floor_indicator(floor)
					//fmt.Println("Elevator is going up - to the " + strconv.Itoa(floor) + ". floor.")
					fmt.Println("Elevator is going up.")
				}
			case command_type == "go_down":
				//go down
				{
					//var floor int
					// Elev_set_motor_direction(driver.DIRN_DOWN)
					// Elev_set_floor_indicator(floor)
					// fmt.Println("Elevator is going down - to the " + strconv.Itoa(floor) + ". floor.")
					fmt.Println("Elevator is going down.")
				}
			}
		}
	}()
	for {}
	fmt.Println("End of program.")
}