package common

import (
	"fmt"
	"net"
	"net/http"
)

// GetServerIPAndVersion determines the server's IP and the IP version of the request
func GetServerIPAndVersion(r *http.Request) (string, string) {
	host, _, err := net.SplitHostPort(r.Host)
	if err == nil {
		ip := net.ParseIP(host)
		if ip != nil {
			if ip.To4() != nil {
				return ip.String(), "IPv4"
			}
			return ip.String(), "IPv6"
		}
	}

	// If unable to get IP from request, find local non-loopback address
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", "Unknown"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), "IPv4"
			}
			if ipNet.IP.To16() != nil {
				return ipNet.IP.String(), "IPv6"
			}
		}
	}
	return "", "Unknown"
}

// GetServerIPAndPort determines the server's IP and port
func GetServerIPAndPort() (string, string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()
	port := fmt.Sprintf("%d", localAddr.Port)
	return ip, port, nil
}
