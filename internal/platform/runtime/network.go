package runtime

import (
	"log"
	"net"
	"strings"
)

func GetIP() string {
	ips, err := net.InterfaceAddrs()
	if err != nil {
		log.Println(err)
		return ""
	}

	for _, address := range ips {
		ipNet, ok := address.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}

		ip := ipNet.IP.String()
		if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || strings.HasPrefix(ip, "192.168.") {
			return ip
		}
	}
	return ""
}
