package driver

import (
	"testing"
	"fmt"
	"os"
)

func TestMain(m *testing.M) {
    Elev_init();
    fmt.Println("Press STOP button to stop elevator and exit program.\n");
    Elev_set_motor_direction(DIRN_UP);
    i := 0

    for i<1 {
        // Change direction when we reach top/bottom floor
        if Elev_get_floor_sensor_signal() == N_FLOORS - 1 {
           Elev_set_motor_direction(DIRN_DOWN)
        } else if Elev_get_floor_sensor_signal() == 0 {
            Elev_set_motor_direction(DIRN_UP)
        }

        // Stop elevator and exit program if the stop button is pressed
        if Elev_get_stop_signal()!=0 {
            Elev_set_motor_direction(DIRN_STOP)
        }
    }

	os.Exit(0)
}
