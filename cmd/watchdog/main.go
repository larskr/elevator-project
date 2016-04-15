// Simple watchdog process which monitors the elevator process and
// stores a backup of the elevator state.
package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"elevator-project/pkg/config"
	"elevator-project/pkg/elev"
)

var infolog *log.Logger
var errorlog *log.Logger

func init() {
	infolog = log.New(os.Stdout, "\x1b[35mWATCHDOG\x1b[m: ", 0)
	errorlog = log.New(os.Stdout, "\x1b[31mERROR\x1b[m: ", 0)
}

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
		conn:           conn,
		elevator:       &net.UnixAddr{conf["watchdog.elev_socket"], "unixgram"},
		backupfilepath: conf["watchdog.backupfile"],
		shutdown:       make(chan chan struct{}),
	}
	wd.proto = exec.Command("./bin/elevator")
	wd.proto.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	wd.proto.Stderr = os.Stderr
	wd.proto.Stdout = os.Stdout

	go wd.Run()

	interrupted := make(chan os.Signal, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT)
	signal.Notify(quit, syscall.SIGQUIT)

	for {
		select {
		case <-interrupted:
			if wd.cmd.Process != nil {
				wd.cmd.Process.Signal(syscall.SIGINT)
				wd.cmd.Wait()
				infolog.Printf("sent SIGINT to elevator process pid %v\n",
					wd.cmd.Process.Pid)
			}
		case <-quit:
			infolog.Printf("shutting down.\n")
			waitc := make(chan struct{})
			wd.shutdown <- waitc
			<-waitc
			os.Exit(0)
		}
	}
}

const (
	aliveTime  = 250 * time.Millisecond
	backupSize = 34 + 3*elev.NumFloors
)

type Watchdog struct {
	conn   *net.UnixConn
	proto  *exec.Cmd
	cmd    *exec.Cmd
	backup [backupSize]byte

	backupfile     *os.File
	backupfilepath string

	elevator *net.UnixAddr

	shutdown chan chan struct{}
}

func (wd *Watchdog) Restart() {
	// Start process
	wd.cmd = exec.Command(wd.proto.Path)
	*wd.cmd = *wd.proto
	wd.cmd.Start()

	infolog.Printf("elevator process pid is %v\n", wd.cmd.Process.Pid)

	// Wait for ready message.
	var buf [16]byte
	wd.conn.SetReadDeadline(time.Time{})
	_, _, err := wd.conn.ReadFromUnix(buf[:])
	if err != nil {
		fmt.Println(err)
	}

	// Send backup to elevator.
	_, err = wd.conn.WriteToUnix(wd.backup[:], wd.elevator)
	if err != nil {
		fmt.Println(err)
	}
}

func (wd *Watchdog) Flush() error {
	wd.backupfile.Seek(0, os.SEEK_SET)

	_, err := wd.backupfile.Write(wd.backup[:])
	if err != nil {
		return nil
	}

	wd.backupfile.Sync()

	return nil
}

func (wd *Watchdog) Run() error {
	fd, err := os.OpenFile(wd.backupfilepath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	wd.backupfile = fd

	fi, err := fd.Stat()
	if err != nil {
		return err
	}

	if fi.Size() != 0 {
		n, err := fd.Read(wd.backup[:])
		if err != nil && n != backupSize {
			fmt.Println(err)
			return err
		}
	}

	wd.Restart()

	var buf [backupSize]byte
	for {
		select {
		case c := <-wd.shutdown:
			if wd.cmd.Process != nil {
				wd.cmd.Process.Signal(syscall.SIGINT)
				wd.cmd.Wait()
			}
			wd.conn.Close()
			c <- struct{}{}
			return nil
		default:
			wd.conn.SetReadDeadline(time.Now().Add(aliveTime))

			_, _, err := wd.conn.ReadFromUnix(buf[:])
			if err != nil {
				if wd.cmd.Process != nil {
					wd.cmd.Process.Signal(syscall.SIGINT)
					wd.cmd.Wait()
				}
				infolog.Printf("elevator process has crashed\n")

				wd.Restart()
				continue
			}

			if !bytes.Equal(wd.backup[32:], buf[32:]) {
				wd.backup = buf
				wd.Flush()
			}
		}
	}
}
