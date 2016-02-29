package driver

import (
	"testing"
	"fmt"
	"os"
)

func readElevatorInput() {
    i:= 0
    for i<1 {
        var k elev_button_type = 0
        for j:=0; j<N_FLOORS; j++{
	    for k=0;k<N_BUTTONS; k++{
                fmt.Println("Floor " +j+ " Button" +k+ "\n")   
            }
	}    
    }
}

func TestMain(m *testing.M) {
    Elev_init();
    fmt.Println("Press STOP button to stop elevator and exit program.\n");
    readElevatorInput()


/*    Elev_set_motor_direction(DIRN_UP);
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
*/
	os.Exit(0)
}
