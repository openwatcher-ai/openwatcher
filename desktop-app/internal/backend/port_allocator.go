package backend

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ResolveLoopbackListen keeps Desktop on the preferred loopback port when possible
// and otherwise walks upward until it finds a free local port.
func ResolveLoopbackListen(preferred string) (string, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(preferred))
	if err != nil {
		return "", err
	}
	if !isLoopbackHost(host) {
		return preferred, nil
	}

	startPort, err := strconv.Atoi(portText)
	if err != nil || startPort <= 0 || startPort > 65535 {
		return "", fmt.Errorf("invalid loopback port: %q", portText)
	}

	for port := startPort; port <= 65535; port++ {
		listen := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := net.Listen("tcp", listen)
		if err == nil {
			_ = ln.Close()
			return listen, nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "address already in use") {
			continue
		}
		return "", err
	}
	return "", fmt.Errorf("no available loopback port from %s", preferred)
}

func isLoopbackHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	if trimmed == "localhost" {
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}
