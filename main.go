package main

import (
	"fmt"
	"os"
	"time"
	//"os/signal"

	"elevator-project/elev"
	"elevator-project/network"
)

type ServiceMode int

const (
	Online ServiceMode = 0x1
	Local  ServiceMode = 0x2
	Broken ServiceMode = 0x3
)

type BackupHandler struct {
	backups map[network.Addr]*backupData
	this    network.Addr
}

func (b *BackupHandler) create(e *Elevator) *backupData {
	var bd = &backupData{
		elevator: b.this,
		created:  time.Now(),
		requests: make([]Request, 0, maxRequests),
		dest:     e.dest,
	}

	for floor := 0; floor < elev.NumFloors; floor++ {
		if e.requests[floor][indexOfDir(elev.Up)] {
			bd.requests = append(bd.requests, Request{floor, elev.Up})
		} else if e.requests[floor][indexOfDir(elev.Down)] {
			bd.requests = append(bd.requests, Request{floor, elev.Down})
		}
	}

	b.backups[b.this] = bd
	return bd
}

func (b *BackupHandler) update(bd *backupData) {
	b.backups[bd.elevator] = bd
}

func (b *BackupHandler) changed(e *Elevator) bool {
	backup := b.backups[b.this]

	var reqs [elev.NumFloors][2]bool
	for _, req := range backup.requests {
		reqs[req.Floor][indexOfDir(req.Direction)] = true
	}

	return !(e.requests == reqs && e.dest == backup.dest)
}

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

	node := network.NewNode()
	node.Start()

	panel := NewPanel()
	panel.Start()

	elevator := NewElevator(panel)
	elevator.Start()

	msgsFromOther := make(chan *network.Message)
	msgsFromThis := make(chan *network.Message)
	go receiveMsgs(node, msgsFromOther)
	go receiveMyMsgs(node, msgsFromThis)

	var backup = &BackupHandler{make(map[network.Addr]*backupData), node.Addr()}
	backup.create(elevator)

	var mode ServiceMode = Local
	reqch := panel.Requests

	for {
		if node.IsConnected() {
			mode = Online
		} else {
			mode = Local
		}

		if mode == Online {

			// if backup.changed(elevator) {
			// 	bd := backup.create(elevator)
			// 	sendData(node, BACKUP, bd)
			// }

			select {
			case req := <-reqch:
				sendData(node, PANEL, &panelData{
					floor:  req.Floor,
					button: btnFromDir(req.Direction),
					on:     true,
				})

				fmt.Printf("PANEL message sent\n")

				sendData(node, COST, &costData{
					elevator: node.Addr(),
					req:      req,
					cost:     elevator.SimulateCost(req),
				})

				fmt.Printf("COST message sent\n")

				reqch = nil // handle only Request at a time.

			case msg := <-msgsFromOther:
				switch msg.Type {
				case PANEL:
					var pd panelData
					unpackData(msg.Data, &pd)
					panel.SetLamp(pd.button, pd.floor, pd.on)
					fmt.Printf("PANEL message received and forwarded\n")

				case COST:
					var cd costData
					unpackData(msg.Data, &cd)
					cost := elevator.SimulateCost(cd.req)
					if cost < cd.cost {
						cd.elevator = node.Addr()
						cd.cost = cost
					}
					packData(msg.Data, &cd)

					fmt.Printf("COST message received and forwarded\n")

				case ASSIGN:
					var ad assignData
					unpackData(msg.Data, &ad)
					if ad.elevator == node.Addr() {
						elevator.AddRequest(ad.req)
						ad.taken = true
						fmt.Printf("ASSIGN message taken\n")
					}
					packData(msg.Data, &ad)

					fmt.Printf("ASSIGN message received and forwarded\n")

					// case BACKUP:
					// 	bd := &backupData{}
					// 	unpackData(msg.Data, bd)
					// 	backup.update(bd)
				}
				node.ForwardMessage(msg)

			case msg := <-msgsFromThis:
				switch msg.Type {
				case COST:
					var cd costData
					unpackData(msg.Data, &cd)
					sendData(node, ASSIGN, &assignData{
						elevator: cd.elevator,
						req:      cd.req,
					})
					fmt.Printf("COST message received from myself\n")
				case ASSIGN:
					var ad assignData
					unpackData(msg.Data, &ad)
					reqch = panel.Requests
					fmt.Printf("ASSIGN message received from myself with taken = %v\n", ad.taken)
					if !ad.taken {
						elevator.AddRequest(ad.req)
						fmt.Printf("Took my own ASSIGN message\n")
					}
				}
			default:
			}

		} else if mode == Local {

			select {
			case req := <-reqch:
				elevator.AddRequest(req)
			default:
			}

		} else if mode == Broken {
		}

	}

	// var bd1 = backupData{
	// 	elevator: node.Addr(),
	// 	created:  time.Now(),
	// 	requests: []Request{Request{1, elev.Down}},
	// }
	// fmt.Println(bd1.created)

	// buf, _ := bd1.MarshalBinary()
	// fmt.Println(buf)

	// var bd2 backupData
	// bd2.UnmarshalBinary(buf)
	// fmt.Println(bd2)
	// fmt.Println(bd2.created)

	// msgsFromOther := make(chan *network.Message)
	// msgsFromThis := make(chan *network.Message)
	// go receiveMsgs(node, msgsFromOther)
	// go receiveMyMsgs(node, msgsFromThis)

	// Stop the elevator with Ctrl+C.
	//interruptc := make(chan os.Signal)
	//signal.Notify(interruptc, os.Interrupt)

	//reqch := panel.Requests

	//for {
	//if mode == online {

	//	select {
	//	case req := <-reqch:
	//		fmt.Printf("Request: floor %v, direction %v\n", req.Floor, req.Direction)
	//		elevator.AddRequest(req)
	//		fmt.Println(elevator.hasWork())
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
	//	case <-interruptc:
	//		elev.SetMotorDirection(elev.Stop)
	//		os.Exit(1)
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
	//	}

	//} else if mode == local {

	//}
	//}

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
