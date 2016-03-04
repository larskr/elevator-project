package driver

import (
	"fmt"
	"os"
	"testing"
	"time"
)

type Orders struct {
	N      int
	oType  [10]elev_button_type
	oFloor [10]int
}

func addOrder(button elev_button, orders *Orders) {
	orders.oFloor[orders.N] = button.floor
	orders.oType[orders.N] = button.bType
	orders.N++
	fmt.Printf("Added Floor %v  Button %v \n", button.floor, button.bType)
}

func removeOrder(orders *Orders) {
	for i := 0; i < orders.N; i++ {
		orders.oFloor[i] = orders.oFloor[i+1]
		orders.oType[i] = orders.oType[i+1]
	}
	orders.N--
}

func readElevatorInput(orders *Orders) {
	//Read buttons
	var prev = [N_FLOORS][N_BUTTONS]int{}
	m := 0
	button := elev_button{0, 0}

	for {
		for button.floor = 0; button.floor < N_FLOORS; button.floor++ {
			for button.bType = 0; button.bType < N_BUTTONS; button.bType++ {
				m = Elev_get_button_signal(button.bType, button.floor)
				if m != 0 && m != prev[button.floor][button.bType] {
					addOrder(button, orders)
					//fmt.Printf("Floor %v  Button %v", button.floor, button.bType)
				}
				prev[button.floor][button.bType] = m
			}
		}
	}
}

func TestMain(m *testing.M) {
	Elev_init()
	fmt.Println("Press STOP button to stop elevator and exit program.")
	orders := Orders{}
	orders.N = 0
	var requestedFloor int
	var floor int = 0
	// go startElev()
	// for {}

	Elev_set_motor_direction(DIRN_UP)
	time.Sleep(1 * time.Second)

	/* go func() {
		var signal, floor int
		for {
			signal = Elev_get_floor_sensor_signal()
			if signal != -1 && signal != floor {
				floor = signal
				fmt.Printf("I am on floor %v!\n", floor)
				if floor == 0 {
					time.Sleep(250*time.Millisecond)
					Elev_set_motor_direction(DIRN_STOP)
				}
			}
			time.Sleep(25*time.Millisecond)
		}
	}()*/

	go readElevatorInput(&orders)
	for {
		time.Sleep(25 * time.Millisecond)
		requestedFloor = orders.oFloor[0]
		var signal int = Elev_get_floor_sensor_signal()
		signal = Elev_get_floor_sensor_signal()
		if signal != -1 {
			floor = signal
		}
		if orders.N != 0 {
			if floor > requestedFloor {
				Elev_set_motor_direction(DIRN_DOWN)
			} else if floor < requestedFloor {
				Elev_set_motor_direction(DIRN_UP)
			} else if floor == requestedFloor {
				fmt.Printf("Reached floor %v \n", requestedFloor)
				removeOrder(&orders)
			}
		} else {
			Elev_set_motor_direction(DIRN_STOP)
		}

		/*// Change direction when we reach top/bottom floor
		  if Elev_get_floor_sensor_signal() == N_FLOORS - 1 {
		     Elev_set_motor_direction(DIRN_DOWN)
		  } else if Elev_get_floor_sensor_signal() == 0 {
		      Elev_set_motor_direction(DIRN_UP)
		  }
		*/
		// Stop elevator and exit program if the stop button is pressed
		time.Sleep(25 * time.Millisecond)
		if Elev_get_stop_signal() != 0 {
			Elev_set_motor_direction(DIRN_STOP)
		}
	}
	os.Exit(0)
}
