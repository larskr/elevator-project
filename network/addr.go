package network

import (
	"net"
)

func IPToUint32(IP net.IP) uint32 {
	var u uint32
	IP = IP.To4()
	u |= uint32(IP[0]) << 24
	u |= uint32(IP[1]) << 16
	u |= uint32(IP[2]) << 8
	u |= uint32(IP[3]) << 0
	return u
}
func Uint32ToIP(u uint32) net.IP {
	return net.IPv4(byte(u>>24), byte(u>>16), byte(u>>8), byte(u>>0))
}

func NetworkAddr() uint32 {
	ifi, err := net.InterfaceByName("eth0")
	if err != nil {
		return 0
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return 0
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if IPv4 := ipnet.IP.To4(); IPv4 != nil {
				return IPToUint32(IPv4)
			}
		}
	}

	return 0
}

func BroadcastAddr() uint32 {
	ifi, err := net.InterfaceByName("eth0")
	if err != nil {
		return 0
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return 0
	}

	for _, addr := range addrs {
		ipnet, isIPNet := addr.(*net.IPNet)
		isBroadcast := (ifi.Flags&net.FlagBroadcast != 0)
		isIPv4 := (ipnet.IP.To4() != nil)
		if isIPNet && isIPv4 && isBroadcast {
			ip := IPToUint32(ipnet.IP)
			mask := IPToUint32(net.IP(ipnet.Mask))
			return ip | ^mask
		}
	}

	return 0
}
