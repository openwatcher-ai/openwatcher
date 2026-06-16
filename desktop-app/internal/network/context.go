package network

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type InterfaceOption struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	IP          string `json:"ip"`
	Recommended bool   `json:"recommended"`
}

type Context struct {
	Interfaces     []InterfaceOption `json:"interfaces"`
	RecommendedIP  string            `json:"recommendedIp"`
	RecommendedTag string            `json:"recommendedTag"`
}

func DetectContext() Context {
	type candidate struct {
		option InterfaceOption
		score  int
	}

	var candidates []candidate
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			if shouldSkipInterface(iface.Name) {
				continue
			}
			addrs, addrErr := iface.Addrs()
			if addrErr != nil {
				continue
			}
			for _, addr := range addrs {
				ip := extractIPv4(addr)
				if ip == nil || !isUsableLANIP(ip) {
					continue
				}
				ipText := ip.String()
				candidates = append(candidates, candidate{
					option: InterfaceOption{
						Name:  iface.Name,
						Label: formatInterfaceLabel(iface.Name, ipText),
						IP:    ipText,
					},
					score: scoreInterface(iface.Name, ip),
				})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].option.Name != candidates[j].option.Name {
			return candidates[i].option.Name < candidates[j].option.Name
		}
		return candidates[i].option.IP < candidates[j].option.IP
	})

	options := make([]InterfaceOption, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		key := item.option.Name + "|" + item.option.IP
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		options = append(options, item.option)
	}

	if len(options) == 0 {
		fallback := InterfaceOption{
			Name:        "loopback",
			Label:       "本机回环 (127.0.0.1)",
			IP:          "127.0.0.1",
			Recommended: true,
		}
		return Context{
			Interfaces:     []InterfaceOption{fallback},
			RecommendedIP:  fallback.IP,
			RecommendedTag: fallback.Label,
		}
	}

	options[0].Recommended = true
	return Context{
		Interfaces:     options,
		RecommendedIP:  options[0].IP,
		RecommendedTag: options[0].Label,
	}
}

func NormalizeMode(raw string) Mode {
	switch strings.TrimSpace(raw) {
	case string(ModePublicURL), "public":
		return ModePublicURL
	case string(ModeManagedBeta), "tunnel":
		return ModeManagedBeta
	case string(ModeLAN):
		return ModeLAN
	default:
		return ModeLAN
	}
}

func NormalizePort(raw string) string {
	port := strings.TrimSpace(raw)
	if _, err := strconv.Atoi(port); err != nil || port == "" {
		return "8787"
	}
	return port
}

func BuildListen(ip, port string, bindAll bool) string {
	port = NormalizePort(port)
	if bindAll {
		return net.JoinHostPort("0.0.0.0", port)
	}
	if strings.TrimSpace(ip) == "" {
		ip = "127.0.0.1"
	}
	return net.JoinHostPort(ip, port)
}

func BuildLANBaseURL(ip, port string) string {
	port = NormalizePort(port)
	if strings.TrimSpace(ip) == "" {
		ip = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s", net.JoinHostPort(ip, port))
}

func NormalizePublicURL(raw string) string {
	parsed, err := ValidatePublicURL(raw, false)
	if err != nil {
		return "https://openwatcher.example.com"
	}
	return parsed
}

func ValidatePublicURL(raw string, requireHTTPS bool) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("请填写自定义公网 URL")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("自定义公网 URL 格式不合法")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("自定义公网 URL 只支持 http 或 https")
	}
	if requireHTTPS && scheme != "https" {
		return "", fmt.Errorf("自定义公网 URL 仅支持 https；局域网调试请改用局域网模式")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("自定义公网 URL 缺少主机名")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("自定义公网 URL 不能携带 query 或 fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func BuildManagedBetaURL(rawCode string) string {
	code := sanitizeTunnelCode(rawCode)
	if code == "" {
		code = "example"
	}
	return "https://" + code + ".beta.openwatcher.example.com"
}

func InterfaceLabelForIP(ctx Context, ip string) string {
	for _, option := range ctx.Interfaces {
		if option.IP == ip {
			return option.Label
		}
	}
	return ""
}

func extractIPv4(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP.To4()
	case *net.IPAddr:
		return value.IP.To4()
	default:
		return nil
	}
}

func isUsableLANIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Equal(net.IPv4zero) {
		return false
	}
	return true
}

func shouldSkipInterface(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return true
	}
	for _, marker := range []string{
		"docker",
		"br-",
		"bridge",
		"vbox",
		"vmnet",
		"utun",
		"tailscale",
		"zerotier",
		"awdl",
		"llw",
		"anpi",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func formatInterfaceLabel(name, ip string) string {
	switch {
	case strings.HasPrefix(name, "en"):
		return fmt.Sprintf("Wi-Fi (%s) — %s", name, ip)
	case strings.HasPrefix(name, "eth"):
		return fmt.Sprintf("Ethernet (%s) — %s", name, ip)
	case strings.HasPrefix(name, "wlan"):
		return fmt.Sprintf("WLAN (%s) — %s", name, ip)
	default:
		return fmt.Sprintf("%s — %s", name, ip)
	}
}

func scoreInterface(name string, ip net.IP) int {
	score := 100
	if isPrivateRFC1918(ip) {
		score += 80
	}
	if strings.HasPrefix(name, "en0") || strings.HasPrefix(name, "wlan0") {
		score += 40
	}
	if strings.HasPrefix(name, "en") || strings.HasPrefix(name, "wlan") {
		score += 20
	}
	if ip[0] == 192 && ip[1] == 168 {
		score += 15
	}
	return score
}

func isPrivateRFC1918(ip net.IP) bool {
	if len(ip) < 4 {
		return false
	}
	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	default:
		return false
	}
}

func sanitizeTunnelCode(raw string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
		}
	}
	return strings.Trim(builder.String(), "-")
}
