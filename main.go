package main

import (
	"fmt"
	"os"
	"os/signal"

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

	err = elev.Init()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//node := network.NewNode()
	//node.Start()

	panel := NewPanel()
	panel.Start()

	elevator := NewElevator(panel)
	elevator.Start()

	// msgsFromOther := make(chan *network.Message)
	// msgsFromThis := make(chan *network.Message)
	// go receiveMsgs(node, msgsFromOther)
	// go receiveMyMsgs(node, msgsFromThis)

	// Stop the elevator with Ctrl+C.
	interruptc := make(chan os.Signal)
	signal.Notify(interruptc, os.Interrupt)

	for {
		select {
		case req := <-panel.Requests:
			fmt.Printf("Request: floor %v, direction %v\n", req.Floor, req.Direction)
			elevator.AddRequest(req)
			fmt.Println(elevator.hasWork())
			// if node.IsConnected() {
			// 	sendData(node, PANEL, &panelData{
			// 		floor:  req.Floor,
			// 		button: btnFromDir(req.Direction),
			// 		on:     true,
			// 	})
			// 	sendData(node, COST, &costData{
			// 		elevator: node.Addr(),
			// 		req: req,
			// 		cost: 1.0,
			// 	})
			// 	fmt.Println("Panel data sent.")
			// }
		case <-interruptc:
			elev.SetMotorDirection(elev.Stop)
			os.Exit(1)
			// case msg := <-msgsFromOther:
			// 	switch msg.Type {
			// 	case PANEL:
			// 		var pd panelData
			// 		unpackData(msg.Data, &pd)
			// 		panel.SetLamp(pd.button, pd.floor, pd.on)
			// 		fmt.Println("Panel data received.")
			// 	case COST:
			// 		var cd costData
			// 		unpackData(msg.Data, &cd)
			// 		cost := elevator.SimulateCost(cd.req)
			// 		if cost < cd.cost {
			// 			cd.elevator = node.Addr()
			// 			cd.cost = cost
			// 		}
			// 		packData(msg.Data, &cd)
			// 		fmt.Println("Cost message forwarded.")
			// 	case ASSIGN:
			// 		var ad assignData
			// 		unpackData(msg.Data, &ad)
			// 		if ad.elevator == node.Addr() {
			// 			elevator.Add(ad.req)
			// 			ad.taken = true
			// 		}
			// 		packData(msg.Data, &ad)
			// 	}
			// 	node.ForwardMessage(msg)
			// case msg := <-msgsFromThis:
			// 	switch msg.Type {
			// 	case COST:
			// 		var cd costData
			// 		unpackData(msg.Data, &cd)
			// 		fmt.Printf("Lowest cost %v for %v\n", cd.cost, net.IP(cd.elevator[:]))
			// 		sendData(node, ASSIGN, &assignData{
			// 			elevator: cd.elevator,
			// 			req: cd.req,
			// 		})
			// 	case ASSIGN:
			// 		var ad assignData
			// 		unpackData(msg.Data, &ad)
			// 		fmt.Printf("Got assign message back with taken = %v\n", ad.taken)
			// 	}
		}
	}

	os.Exit(0)
}

func receiveMsgs(node *network.Node, msgs chan *network.Message) {
	for {
		msg := node.ReceiveMessage()
		msgs <- msg
	}
}

func receiveMyMsgs(node *network.Node, msgs chan *network.Message) {
	for {
		msg := node.ReceiveMyMessage()
		msgs <- msg
	}
}
