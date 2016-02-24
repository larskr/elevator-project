package driver


type elev_motor_direction int

const (
    DIRN_DOWN elev_motor_direction     = -1 
    DIRN_STOP                          =  0   
    DIRN_UP                            =  1   
)

type elev_button_type int

const (
    BUTTON_CALL_UP elev_button_type     = 0 
    BUTTON_CALL_DOWN                    = 1   
    BUTTON_COMMAND                      = 2   
)

const (
	MOTOR_SPEED = 2800 
        N_FLOORS    = 4
        N_BUTTONS   = 3
	)
var lamp_channel_matrix = [N_FLOORS][N_BUTTONS]int{
    {LIGHT_UP1, LIGHT_DOWN1, LIGHT_COMMAND1},
    {LIGHT_UP2, LIGHT_DOWN2, LIGHT_COMMAND2},
    {LIGHT_UP3, LIGHT_DOWN3, LIGHT_COMMAND3},
    {LIGHT_UP4, LIGHT_DOWN4, LIGHT_COMMAND4},
};


var button_channel_matrix = [N_FLOORS][N_BUTTONS]int {
    {BUTTON_UP1, BUTTON_DOWN1, BUTTON_COMMAND1},
    {BUTTON_UP2, BUTTON_DOWN2, BUTTON_COMMAND2},
    {BUTTON_UP3, BUTTON_DOWN3, BUTTON_COMMAND3},
    {BUTTON_UP4, BUTTON_DOWN4, BUTTON_COMMAND4},
};



func Elev_init() {
    //init_success := Init()
    Init()
    //assert(init_success && "Unable to initialize elevator hardware!");
    var b elev_button_type = 0
    for f := 0; f < N_FLOORS; f++ {
        for b = 0; b < N_BUTTONS; b++{
            Elev_set_button_lamp(b, f, 0)
        }
    }

    Elev_set_stop_lamp(0)
    Elev_set_door_open_lamp(0)
    Elev_set_floor_indicator(0)
}


func Elev_set_motor_direction(dirn elev_motor_direction) {
    if dirn == 0{
        Write_analog(MOTOR, 0)
    } else if dirn > 0 {
        Clear_bit(MOTORDIR)
        Write_analog(MOTOR, MOTOR_SPEED)
    } else if dirn < 0 {
        Set_bit(MOTORDIR)
        Write_analog(MOTOR, MOTOR_SPEED)
    }
}


func Elev_set_button_lamp(button elev_button_type,floor int,value int) {
//    assert(floor >= 0);
//    assert(floor < N_FLOORS);
//    assert(button >= 0);
//    assert(button < N_BUTTONS);

    if value!=0 {
        Set_bit(lamp_channel_matrix[floor][button])
    } else {
        Clear_bit(lamp_channel_matrix[floor][button])
    }
}


func Elev_set_floor_indicator(floor int) {
//    assert(floor >= 0);
//    assert(floor < N_FLOORS);

    // Binary encoding. One light must always be on.
    if floor != 0 & 0x02 {
        Set_bit(LIGHT_FLOOR_IND1)
    } else {
        Clear_bit(LIGHT_FLOOR_IND1)
    }    

    if floor != 0 & 0x01 {
        Set_bit(LIGHT_FLOOR_IND2)
    } else {
        Clear_bit(LIGHT_FLOOR_IND2)
    }   
}


func Elev_set_door_open_lamp(value int) {
    if value != 0 {
        Set_bit(LIGHT_DOOR_OPEN)
    } else {
        Clear_bit(LIGHT_DOOR_OPEN)
    }
}


func Elev_set_stop_lamp(value int) {
    if value != 0 {
        Set_bit(LIGHT_STOP)
    } else {
        Clear_bit(LIGHT_STOP)
    }
}



func Elev_get_button_signal(button elev_button_type,floor int) int {
//    assert(floor >= 0);
//    assert(floor < N_FLOORS);
//    assert(button >= 0);
//    assert(button < N_BUTTONS);


    if Read_bit(button_channel_matrix[floor][button])!=0 {
        return 1
    } else {
        return 0
    }    
}


func Elev_get_floor_sensor_signal() int {
    if Read_bit(SENSOR_FLOOR1) !=0 {
        return 0
    } else if Read_bit(SENSOR_FLOOR2) !=0 {
        return 1
    } else if Read_bit(SENSOR_FLOOR3) !=0 {
        return 2
    } else if Read_bit(SENSOR_FLOOR4) !=0 {
        return 3
    } else {
        return -1
    }
}


func Elev_get_stop_signal() int {
    return Read_bit(STOP);
}


func Elev_get_obstruction_signal() int {
    return Read_bit(OBSTRUCTION);
}


