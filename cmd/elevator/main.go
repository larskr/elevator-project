package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"elevator-project/pkg/config"
	"elevator-project/pkg/elev"
	"elevator-project/pkg/network"
)

var (
	noWatchdog = flag.Bool("nowatchdog", false,
		"Set this to run without a watchdog process.")
)

var debug *log.Logger
var errorlog *log.Logger

func init() {
	debug = log.New(os.Stdout, "", 0)
	errorlog = log.New(os.Stderr, "\x1b[31mERROR\x1b[m: ", 0)
}

type ServiceMode int

const (
	Online  ServiceMode = 0x1
	Local   ServiceMode = 0x2
	Stopped ServiceMode = 0x3
)

// Stores backups for all connected elevators.
type BackupHandler struct {
	backups map[network.Addr]*backupData
	addr    network.Addr

	// An empty struct is sent on this channel when the latest backup
	// and the current elevator state is different.
	invalid chan struct{}
}

// Copy the elevator state into a backupData struct, and store in backup database.
func (b *BackupHandler) create(e *Elevator) *backupData {
	var bd = &backupData{
		elevator:  b.addr,
		created:   time.Now(),
		floor:     e.floor,
		direction: e.direction,
		requests:  e.requestsBuffer,
		dest:      e.destBuffer,
	}
	b.backups[b.addr] = bd
	return bd
}

// Get the latest backup of this elevator.
func (b *BackupHandler) get() *backupData {
	return b.backups[b.addr]
}

// Store backupData struct in database.
func (b *BackupHandler) update(bd *backupData) {
	b.backups[bd.elevator] = bd
}

// Check if current elevator state differs from latest backup. Runs in a goroutine.
func (b *BackupHandler) changed(e *Elevator) bool {
	for {
		backup := b.backups[b.addr]
		if !(e.requestsBuffer == backup.requests && e.destBuffer == backup.dest) {
			b.invalid <- struct{}{}
		}
	}
}

// Handles communication with the watchdog process over Unix domain sockets.
type WatchdogHandler struct {
	conn     *net.UnixConn
	watchdog *net.UnixAddr
	addr     *net.UnixAddr
	timer    *time.Timer
}

const watchdogResendInterval = 150 * time.Millisecond

// Connects to watchdog process and loads inital backup.
func (wd *WatchdogHandler) start() (*backupData, error) {
	if *noWatchdog {
		return &backupData{}, nil
	}

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

	bd := &backupData{}
	unpackData(buf[:n], bd)

	return bd, nil
}

// Send backupData to watchdog process.
func (wd *WatchdogHandler) writeBackup(bd *backupData) error {
	if *noWatchdog {
		return nil
	}

	data, _ := bd.MarshalBinary()
	_, err := wd.conn.WriteToUnix(data, wd.watchdog)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()

	var mode ServiceMode

	// Load configuration file.
	conf, _ := config.LoadFile("./config")
	elev.LoadConfig(conf)
	network.LoadConfig(conf)

	// Initialize WatchdogHandler and load elevator backup.
	watchdog := &WatchdogHandler{
		watchdog: &net.UnixAddr{conf["watchdog.socket"], "unixgram"},
		addr:     &net.UnixAddr{conf["watchdog.elev_socket"], "unixgram"},
		timer:    time.NewTimer(watchdogResendInterval),
	}
	wdbackup, _ := watchdog.start()

	// Initialize elavator hardware.
	err := elev.Init()
	if err != nil {
		debug.Println(err)
		os.Exit(1)
	}

	node := network.NewNode()
	panel := NewPanel()
	elevator := NewElevator(panel)

	// Load the backup from the watchdog process. This does nothing if
	// wdbackup has only nil-values.
	elevator.LoadBackup(wdbackup)
	panel.LoadBackup(wdbackup)

	// Setup channels.
	msgsFromOther := make(<-chan *network.Message)
	msgsFromThis := make(<-chan *network.Message)
	deadNode := make(<-chan network.Addr)

	// Setup signal handler
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT)

	// Start all threads. NOTE!! The order matters.
	node.Start()
	panel.Start()
	elevator.Start()

	// Start Goroutines that put all incomming messages onto channels.
	go receiveMsgs(node, msgsFromOther)
	go receiveMyMsgs(node, msgsFromThis)
	go getDeadNode(node, deadNode)

	// Initialize BackupHandler and store an initial backup.
	var backup = &BackupHandler{
		backups: make(map[network.Addr]*backupData),
		addr:    node.Addr(),
		invalid: make(chan struct{}),
	}
	backup.create(elevator)
	go backup.changed(elevator)

	// Start elevator in local mode.
	mode = Local

	// Channel containing all unassigned requests.
	unassigned := make(chan Request, 32)

	// When an unassigned request is being processed, reqch is set to nil. This means
	// we process only one request at a time.
	reqch := unassigned

	// The current unassigned request being processed.
	var req Request

	for {
		/*
		 * Update elevator service mode.
		 */
		if node.IsConnected() && elevator.IsRunning() {
			switch mode {
			case Local:
				sendData(node, SYNC, &syncData{})
			case Stopped:
			case Online:
			}

			mode = Online
		} else if node.IsConnected() && !elevator.IsRunning() {
			switch mode {
			case Online:
				restoreBackup(unassigned, backup.get())
			case Local:
				sendData(node, SYNC, &syncData{})
			case Stopped:
			}

			mode = Stopped
		} else {
			switch mode {
			case Stopped: fallthrough
			case Online:
				// Only local requests on panel.
				lightPanel(panel, backup.get(), nil)

				// Check if the elevator was disconnected in the middle of a transaction.
				if reqch == nil {
					unassigned <- req
					reqch = unassigned
				}
				
			case Local:
			}

			mode = Local
		}

		/*
		 * Process messages and handle backups.
		 */
		select {

		case <-watchdog.timer.C:
			watchdog.writeBackup(backup.get())
			watchdog.timer.Reset(watchdogResendInterval)

		case <-backup.invalid:
			bd := backup.create(elevator)
			watchdog.writeBackup(bd)

			if mode != Local {
				sendData(node, BACKUP, bd)
				debug.Printf("Sent backup message: \n\t%v\n", bd)
			}

		case r := <-panel.Requests:
			unassigned <- r

		case req = <-reqch:
			if mode == Online {
				var cd = costData{
					elevator: node.Addr(),
					req:      req,
					cost:     elevator.SimulateCost(req),
				}
				sendData(node, COST, &cd)
				debug.Printf("Sent cost message: \n\t%v\n", cd)

				reqch = nil // handle only Request at a time.
			} else if mode == Stopped {
				var cd = costData{
					elevator: node.Addr(),
					req:      req,
					cost:     9000.0,
				}
				sendData(node, COST, &cd)
				debug.Printf("Sent cost message: \n\t%v\n", cd)

				reqch = nil // handle only Request at a time.
			} else if mode == Local {
				elevator.AddRequest(req)
			}

		case msg := <-msgsFromOther:
			if mode == Stopped {
				node.ForwardMessage(msg)
			} else if mode == Local {
				break
			}

			switch msg.Type {
			case COST:
				var cd costData
				if err := unpackData(msg.Data, &cd); err != nil {
					errorlog.Println(err)
					break
				}

				debug.Printf("Received cost message: \n\t%v\n", cd)

				// Update cost message if our cost is lower.
				cost := elevator.SimulateCost(cd.req)
				if cost < cd.cost {
					cd.elevator = node.Addr()
					cd.cost = cost
				}

				debug.Printf("Forwarded cost message: \n\t%v\n", cd)

				packData(msg.Data, &cd)

			case ASSIGN:
				var ad assignData
				if err := unpackData(msg.Data, &ad); err != nil {
					errorlog.Println(err)
					break
				}

				debug.Printf("Received assign message: \n\t%v\n", ad)

				// Add request to elevator if the request was assign to this elevator.
				if ad.elevator == node.Addr() {
					elevator.AddRequest(ad.req)
					panel.SetLamp(btnFromDir(ad.req.direction), ad.req.floor, true)
					ad.taken = true
				}

				debug.Printf("Forwarded assign message: \n\t%v\n", ad)

				packData(msg.Data, &ad)

			case BACKUP:
				var bd backupData
				if err := unpackData(msg.Data, &bd); err != nil {
					errorlog.Println(err)
					break
				}

				debug.Printf("Forwarded backup message: \n\t%v\n", bd)

				old := backup.backups[bd.elevator]
				lightPanel(panel, &bd, old)
				backup.update(&bd)

			case SYNC:
				var sd syncData
				unpackData(msg.Data, &sd)
				syncBackup(&sd, backup.get())
				packData(msg.Data, &sd)

			}
			node.ForwardMessage(msg)

		case msg := <-msgsFromThis:
			if mode == Local {
				break
			}

			switch msg.Type {
			case COST:
				var cd costData
				if err := unpackData(msg.Data, &cd); err != nil {
					elevator.AddRequest(req)
					reqch = unassigned
					errorlog.Println(err)
					break
				}

				// Assign request to elevator with lowest cost value.
				var ad = assignData{
					elevator: cd.elevator,
					req:      cd.req,
				}
				sendData(node, ASSIGN, &ad)
				debug.Printf("Cost message returned: \n\t%v\n", cd)
				debug.Printf("Sent assign message: \n\t%v\n", ad)

			case ASSIGN:
				var ad assignData
				if err := unpackData(msg.Data, &ad); err != nil {
					elevator.AddRequest(req)
					reqch = unassigned
					errorlog.Println(err)
					break
				}

				debug.Printf("Assign message returned: \n\t%v\n", ad)
				if !ad.taken {
					if mode != Stopped {
						elevator.AddRequest(ad.req)
					} else {
						unassigned <- req
					}
				}

				// Transaction complete. Ready to process next request.
				reqch = unassigned

			case SYNC:
				var sd syncData
				unpackData(msg.Data, &sd)
				lightBackup(panel, sd.latest, nil)

			}

		case <-interrupt:
			elev.SetMotorDirection(elev.Stop)
			os.Exit(0)

		case dead := <-deadNode:
			debug.Printf("%v has been disconnected. Try to find backup.\n", dead)

			// Lookup backup for the disconnected elevator.
			if deadbackup, found := backup.backups[dead]; found {
				debug.Printf("Found backup of %v\n", dead)

				restoreBackup(unassigned, deadbackup)
			}
		default:
		}

	}

	os.Exit(0)
}

// Extracts request from backup into channel.
func restoreBackup(c chan Request, bd *backupData) {
	for floor := 0; floor < elev.NumFloors; floor++ {
		for _, dir := range []elev.Direction{elev.Down, elev.Up} {
			if bd.requests[floor][indexOfDir(dir)] {
				c <- Request{floor, dir}
			}
		}
	}
}

// ORs the backup into syncData.
func syncBackup(sd *syncData, bd *backupData) {
	for floor := 0; floor < elev.NumFloors; floor++ {
		for _, dir := range []elev.Direction{elev.Down, elev.Up} {
			if bd.requests[floor][indexOfDir(dir)] {
				sd.latest.requests[floor][indexOfDir(dir)] = true
			}
		}
	}
}

// Update panel by comparing old and new backups.
func lightPanel(panel *Panel, new, old *backupData) {
	var empty backupData
	if old == nil {
		old = &empty
	}

	for floor := 0; floor < elev.NumFloors; floor++ {
		for _, dir := range []elev.Direction{elev.Down, elev.Up} {
			if !old.requests[floor][indexOfDir(dir)] && new.requests[floor][indexOfDir(dir)] {
				panel.SetLamp(btnFromDir(dir), floor, true)
			} else if old.requests[floor][indexOfDir(dir)] && !new.requests[floor][indexOfDir(dir)] {
				panel.SetLamp(btnFromDir(dir), floor, false)
			}
		}
	}

}

// Listen for disconnected elevators.
func getDeadNode(node *network.Node, c chan network.Addr) {
	for {
		c <- node.GetDeadNode()
	}
}

// Listen for new messages from other elevators.
func receiveMsgs(node *network.Node, msgs chan *network.Message) {
	for {
		msgs <- node.ReceiveMessage()
	}
}

// Listen for new message from this elevator.
func receiveMyMsgs(node *network.Node, msgs chan *network.Message) {
	for {
		msgs <- node.ReceiveMyMessage()
	}
}
