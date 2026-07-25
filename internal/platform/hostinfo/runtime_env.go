package hostinfo

import (
	"log"
	"net"
	"os"
	"strings"
)

// GetNetworkIPs returns private IPv4 addresses reachable from the current host.
func GetNetworkIPs() []string {
	networkIPs := make([]string, 0)
	ips, err := net.InterfaceAddrs()
	if err != nil {
		log.Println(err)
		return networkIPs
	}

	for _, address := range ips {
		ipNet, ok := address.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
			continue
		}

		ip := ipNet.IP.String()
		if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") || strings.HasPrefix(ip, "192.168.") {
			networkIPs = append(networkIPs, ip)
		}
	}
	return networkIPs
}

// IsRunningInContainer reports whether the current process appears to be containerized.
func IsRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "kubepods") ||
			strings.Contains(content, "/lxc/") {
			return true
		}
	}

	for _, envVar := range []string{"KUBERNETES_SERVICE_HOST", "DOCKER_CONTAINER", "container"} {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		comm := strings.TrimSpace(string(data))
		if comm != "init" && comm != "systemd" {
			if strings.Contains(comm, "docker") ||
				strings.Contains(comm, "containerd") ||
				strings.Contains(comm, "runc") {
				return true
			}
		}
	}

	return false
}
