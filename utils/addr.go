package utils

import (
	"net"
)

func NetworkAddr() net.IP {
	ifi, err := net.InterfaceByName("en0")
	if err != nil {
		panic(err)
	}
	
	addrs, err := ifi.Addrs()
	if err != nil {
		panic(err)
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if IPv4 := ipnet.IP.To4(); IPv4 != nil {
				return IPv4
			}
		}
	}

	return net.IPv4zero
}

func BroadcastAddr() net.IP {
	ifi, err := net.InterfaceByName("en0")
	if err != nil {
		panic(err)
	}
	
	addrs, err := ifi.Addrs()
	if err != nil {
		panic(err)
	}
	
	for _, addr := range addrs {
		ipnet, isIPNet := addr.(*net.IPNet)
		isBroadcast := (ifi.Flags & net.FlagBroadcast != 0)
		isIPv4 := (ipnet.IP.To4() != nil)
		if isIPNet && isIPv4 && isBroadcast {
			ip := ipnet.IP.To4()
			mask := ipnet.Mask

			var buf [4]byte
			buf[0] = ip[0] | ^mask[0]
			buf[1] = ip[1] | ^mask[1]
			buf[2] = ip[2] | ^mask[2]
			buf[3] = ip[3] | ^mask[3]
			bcast := net.IPv4(buf[0], buf[1], buf[2], buf[3])

			return bcast
		}
	}

	
	return net.IPv4zero
}
