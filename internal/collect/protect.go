package collect

import (
	"net"
	"strings"
	"sync"
	"time"
)

var protectedNets []*net.IPNet

func init() {
	cidrs := []string{
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
		"2400:cb00::/32",
		"2606:4700::/32",
		"2803:f800::/32",
		"2405:b500::/32",
		"2405:8100::/32",
		"2a06:98c0::/29",
		"2c0f:f248::/32",
	}
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		protectedNets = append(protectedNets, n)
	}
}

func NormalizeIP(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")
	ip := net.ParseIP(s)
	if ip == nil {
		return "", false
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String(), true
	}
	return ip.String(), true
}

func IsLoopbackIP(ip string) bool {
	p := net.ParseIP(ip)
	return p != nil && p.IsLoopback()
}

func IsExemptIP(ip string, extra []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return true
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsUnspecified() || parsed.IsLinkLocalUnicast() || parsed.IsMulticast() {
		return true
	}
	for _, n := range protectedNets {
		if n.Contains(parsed) {
			return true
		}
	}
	if isLocalInterfaceIP(parsed) {
		return true
	}
	for _, spec := range extra {
		if matchIgnore(parsed, spec) {
			return true
		}
	}
	return false
}

func matchIgnore(ip net.IP, spec string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	if strings.Contains(spec, "/") {
		_, n, err := net.ParseCIDR(spec)
		return err == nil && n.Contains(ip)
	}
	other := net.ParseIP(spec)
	return other != nil && other.Equal(ip)
}

var (
	localIPMu   sync.Mutex
	localIPs    []net.IP
	localIPWhen time.Time
)

func isLocalInterfaceIP(ip net.IP) bool {
	localIPMu.Lock()
	defer localIPMu.Unlock()
	if time.Since(localIPWhen) > 30*time.Second || localIPs == nil {
		var next []net.IP
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				n, ok := a.(*net.IPNet)
				if !ok || n.IP == nil {
					continue
				}
				next = append(next, n.IP)
			}
		}
		localIPs = next
		localIPWhen = time.Now()
	}
	for _, a := range localIPs {
		if a.Equal(ip) {
			return true
		}
	}
	return false
}
