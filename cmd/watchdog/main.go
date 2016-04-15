// Simple watchdog process which monitors the elevator process and
// stores a backup of the elevator state.
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"elevator-project/pkg/config"
)

func main() {
	conf, err := config.LoadFile("./config")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	addr := &net.UnixAddr{conf["watchdog.socket"], "unixgram"}
	os.Remove(conf["watchdog.socket"])
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var wd = &Watchdog{
		conn: conn,
		elevator: &net.UnixAddr{conf["watchdog.elev_socket"], "unixgram"},
		shutdown: make(chan chan struct{}),
	}
	wd.proto = exec.Command("./bin/elevator")
	wd.proto.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	wd.proto.Stderr = os.Stderr
	wd.proto.Stdout = os.Stdout

	go wd.Run()

	interrupted := make(chan os.Signal)
	quit := make(chan os.Signal)
	signal.Notify(interrupted, syscall.SIGINT)
	signal.Notify(quit, syscall.SIGQUIT)

	for {
		select {
		case <-interrupted:
			if wd.cmd.Process != nil {
				wd.cmd.Process.Signal(syscall.SIGINT)
				wd.cmd.Wait()
				fmt.Printf("\nWATCHDOG: sent SIGINT to elevator process pid %v\n",
					wd.cmd.Process.Pid)
			}
		case <-quit:
			fmt.Printf("\nWATCHDOG: shutting down.\n")
			waitc := make(chan struct{})
			wd.shutdown <- waitc
			<-waitc
			os.Exit(0)
		}
	}	
}

const (
	aliveTime = 3 * time.Second
)

type Watchdog struct {
	conn   *net.UnixConn
	proto  *exec.Cmd
	cmd    *exec.Cmd
	backup [256]byte

	elevator *net.UnixAddr

	shutdown chan chan struct{}
}

func (wd *Watchdog) Restart() {
	// Start process
	wd.cmd = exec.Command(wd.proto.Path)
	*wd.cmd = *wd.proto
	wd.cmd.Start()

	fmt.Printf("WATCHDOG: elevator process pid is %v\n", wd.cmd.Process.Pid)

	

	var buf [16]byte
	wd.conn.SetReadDeadline(time.Time{})
	_, _, err := wd.conn.ReadFromUnix(buf[:])
	if err != nil {
		fmt.Println(err)
	}
	
	_, err = wd.conn.WriteToUnix(wd.backup[:], wd.elevator)
	if err != nil {
		fmt.Println(err)
	}
}

func (wd *Watchdog) Run() {
	wd.Restart()

	for {
		select {
		case c := <-wd.shutdown:
			if wd.cmd.Process != nil {
				wd.cmd.Process.Signal(syscall.SIGINT)
				wd.cmd.Wait()
			}
			wd.conn.Close()
			c <- struct{}{}
			return
		default:
			wd.conn.SetReadDeadline(time.Now().Add(aliveTime))
			
			_, _, err := wd.conn.ReadFromUnix(wd.backup[:])
			if err != nil {
				if wd.cmd.Process != nil {
					wd.cmd.Process.Signal(syscall.SIGINT)
					wd.cmd.Wait()
				}
				fmt.Printf("WATCHDOG: elevator process has crashed\n")
			
				wd.Restart()
			}
		}
	}
}
