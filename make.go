// +build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	buildGoPath := os.Getenv("GOPATH")
	if buildGoPath == "" {
		fmt.Println("Cannot run make.go without GOPATH set.")
		os.Exit(1)
	}

	targets := []string{"./bin/elevator", "./bin/watchdog"}
	srcs := []string{"./cmd/elevator", "./cmd/watchdog"}

	for i := range targets {
		cmd := exec.Command("go", "build", "-o", targets[i], srcs[i])
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			os.Exit(1)
		}
	}

	os.Exit(0)
}
