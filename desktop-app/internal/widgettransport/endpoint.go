// Package widgettransport validates the private loopback endpoint shared by
// the Desktop process, sidecar supervisor, and Widget helper.
package widgettransport

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var ErrInvalidEndpoint = errors.New("悬浮球服务地址无效")

// ParseEndpoint accepts only a plain HTTP origin backed by a numeric loopback
// address and an explicit non-zero port. It returns a canonical origin without
// a trailing slash.
func ParseEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrInvalidEndpoint
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme != "http" || u.Opaque != "" || u.User != nil ||
		u.Host == "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
		u.RawFragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return "", ErrInvalidEndpoint
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" || port == "" {
		return "", ErrInvalidEndpoint
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", ErrInvalidEndpoint
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", ErrInvalidEndpoint
	}
	return "http://" + net.JoinHostPort(ip.String(), strconv.Itoa(portNumber)), nil
}
