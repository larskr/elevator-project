package main 

import (
	"fmt"
	"time"
	"network"
	"sync"
)

func main() {
	var ping_channel = make(chan network.Frame)
	var commands_channel = make(chan network.Frame)
	var remote_addr string
	var port int
	var mutexInit = &sync.Mutex{}
	//initializing dispacher
	go func (){
		mutexInit.Lock()
		fmt.Println("Enter remote address.")
		fmt.Scanln(&remote_addr)
		fmt.Println("Enter port number.")
		fmt.Scanln(&port)
		err := network.Init(port,  ping_channel, commands_channel)
		// timerIni := time.NewTimer(time.Second*10)
		// <- timerIni.C
		// fmt.Println("timerIni expired")
		if err != nil {
			fmt.Println("Dispacher initialization failed.")
			fmt.Println(err)
		} else {
			fmt.Println("Dispacher initialization successful.")
		}
		mutexInit.Unlock()
	}()
	timerInit := time.NewTimer(time.Second)
	<- timerInit.C
	//fmt.Println("timerInit expired")
	mutexInit.Lock()
	fmt.Println("Dispacher is set up with following parameters:")
	fmt.Printf("Remote address:					" + remote_addr + "\n")
	fmt.Printf("Port number:					%d\n", port)
	mutexInit.Unlock()
	//send commands
	fmt.Println("Control point 1")
	go func () {
		for {
			var command string
			fmt.Println("Enter command.")
			fmt.Scanln(&command)
			fmt.Println(command)
			// var commMessage network.Frame
			// commMessage = network.Frame{
			// 	RemoteAddr: remote_addr,
			// 	Data: []byte(command),
			// }
			//commands_channel <- commMessage
			fmt.Println("Command \"" + command + "\" sent.")
		}
	}()
	fmt.Println("Control point 2")
	for {}
	fmt.Println("End of program.")
}