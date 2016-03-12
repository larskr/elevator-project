package network

import (
	"net"
)

type Config struct {
	Interface string
	UseIPv6 bool
}

func LoadConfig(conf *Config) {
	config = *conf
}

// The Addr type is used to identify nodes and can be easliy converted
// to net.IP. It should always be 16 bytes long irregardless of IP version.
type Addr []byte

func NetworkAddr() Addr {
	ifi, err := net.InterfaceByName(config.Interface)
	if err != nil {
		return nil
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return Addr(ipnet.IP.To16())
			}
		}
	}

	return nil
}

func BroadcastAddr() Addr {
	ifi, err := net.InterfaceByName(config.Interface)
	if err != nil {
		return nil
	}

	addrs, err := ifi.Addrs()
	if err != nil {
		return nil
	}

	for _, addr := range addrs {
		ipnet, isIPNet := addr.(*net.IPNet)
		isBroadcast := (ifi.Flags&net.FlagBroadcast != 0)
		isIPv4 := (ipnet.IP.To4() != nil)
		if isIPNet && isIPv4 && isBroadcast {
			ip := ipnet.IP.To16()
			mask := net.IP(ipnet.Mask).To16()
			ip[12] |= ^mask[12]
			ip[13] |= ^mask[13]
			ip[14] |= ^mask[14]
			ip[15] |= ^mask[15]
			return Addr(ip) 
		}
	}

	return nil
}
