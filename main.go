package main

import (
	"./driver"
	"fmt"
)

func main() {
	fmt.Println("Started")
	driver.Init()
	driver.Set_bit(driver.LIGHT_FLOOR_IND1)

	fmt.Println("Done.")

	// We wait to make sure the driver starts all its threads & connections
	select {}

}
