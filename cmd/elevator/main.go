package main

import (
	"fmt"
	"net"
	"os"
	"time"
	//"os/signal"

	"elevator-project/pkg/config"
	"elevator-project/pkg/elev"
	"elevator-project/pkg/network"
)

type ServiceMode int

const (
	Online ServiceMode = 0x1
	Local  ServiceMode = 0x2
	Broken ServiceMode = 0x3
)

type BackupHandler struct {
	backups map[network.Addr]*backupData
	addr    network.Addr
	synced  bool
}

func (b *BackupHandler) create(e *Elevator) *backupData {
	var bd = &backupData{
		elevator: b.addr,
		created:  time.Now(),
		requests: e.requests,
		dest:     e.dest,
	}
	b.backups[b.addr] = bd
	return bd
}

func (b *BackupHandler) get() *backupData {
	return b.backups[b.addr]
}

func (b *BackupHandler) update(bd *backupData) {
	b.backups[bd.elevator] = bd
}

func (b *BackupHandler) changed(e *Elevator) bool {
	backup := b.backups[b.addr]
	return !(e.requests == backup.requests && e.dest == backup.dest)
}

type WatchdogHandler struct {
	conn     *net.UnixConn
	watchdog *net.UnixAddr
	addr     *net.UnixAddr
	timer    Timer
}

const watchdogResendInterval = 100 * time.Millisecond

func (wd *WatchdogHandler) start() ([]byte, error) {
	// unlink socket
	os.Remove(wd.addr.Name)

	var err error
	wd.conn, err = net.ListenUnixgram("unixgram", wd.addr)
	if err != nil {
		return nil, err
	}

	// Let the watchdog process know that the elevator is ready to receive.
	_, err = wd.conn.WriteToUnix([]byte("ready"), wd.watchdog)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 256)
	n, _, err := wd.conn.ReadFromUnix(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

func (wd *WatchdogHandler) writeBackup(bd *backupData) error {
	data, _ := bd.MarshalBinary()

	_, err := wd.conn.WriteToUnix(data, wd.watchdog)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	conf, _ := config.LoadFile("./config")
	elev.LoadConfig(conf)
	network.LoadConfig(conf)

	watchdog := &WatchdogHandler{
		watchdog: &net.UnixAddr{conf["watchdog.socket"], "unixgram"},
		addr:     &net.UnixAddr{conf["watchdog.elev_socket"], "unixgram"},
	}
	watchdog.start()

	err := elev.Init()
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

	var backup = &BackupHandler{make(map[network.Addr]*backupData), node.Addr(), false}
	backup.create(elevator)

	var mode ServiceMode = Local
	reqch := panel.Requests

	for {
		// TODO: check node.IsConnected()
		
		if watchdog.timer.HasTimedOut() {
			watchdog.writeBackup(backup.get())
			watchdog.timer.Reset(watchdogResendInterval)
		}
		
		if mode == Online {

			if backup.synced && backup.changed(elevator) {
				bd := backup.create(elevator)
				backup.synced = false
				sendData(node, BACKUP, bd)
			}

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

				case BACKUP:
					bd := &backupData{}
					unpackData(msg.Data, bd)
					backup.update(bd)

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

				case BACKUP:
					backup.synced = true

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

	// Stop the elevator with Ctrl+C.
	//interruptc := make(chan os.Signal)
	//signal.Notify(interruptc, os.Interrupt)

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
