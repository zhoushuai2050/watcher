package geo

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Info struct {
	IP       string  `json:"ip"`
	Family   string  `json:"family"`
	Private  bool    `json:"private"`
	Country  string  `json:"country"`
	Region   string  `json:"region"`
	City     string  `json:"city"`
	ISP      string  `json:"isp"`
	Org      string  `json:"org"`
	ASN      string  `json:"asn"`
	Timezone string  `json:"timezone"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Location string  `json:"location"`
	Source   string  `json:"source"`
}

type cacheEntry struct {
	info Info
	exp  time.Time
}

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
	cacheMu    sync.Mutex
	cache      = map[string]cacheEntry{}
	cacheTTL   = 12 * time.Hour
)

func Lookup(raw string) (Info, error) {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return Info{}, fmt.Errorf("无效 IP")
	}
	norm := ip.String()
	if v4 := ip.To4(); v4 != nil {
		norm = v4.String()
	}
	info := Info{IP: norm, Family: "IPv4"}
	if ip.To4() == nil {
		info.Family = "IPv6"
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
		info.Private = true
		info.Location = privateLabel(ip)
		return info, nil
	}

	cacheMu.Lock()
	if e, ok := cache[norm]; ok && time.Now().Before(e.exp) {
		cacheMu.Unlock()
		return e.info, nil
	}
	cacheMu.Unlock()

	out, err := lookupIPAPI(norm)
	if err != nil {
		out, err = lookupIPWho(norm)
	}
	if err != nil {
		return info, fmt.Errorf("查询失败：%w", err)
	}
	out.IP = norm
	out.Family = info.Family
	out.Location = joinLocation(out.Country, out.Region, out.City)
	cacheMu.Lock()
	cache[norm] = cacheEntry{info: out, exp: time.Now().Add(cacheTTL)}
	cacheMu.Unlock()
	return out, nil
}

func privateLabel(ip net.IP) string {
	if ip.IsLoopback() {
		return "本机回环地址"
	}
	if ip.IsLinkLocalUnicast() {
		return "链路本地地址"
	}
	if ip.IsPrivate() {
		return "私网地址"
	}
	return "保留地址"
}

func joinLocation(parts ...string) string {
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "-" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		return "未知"
	}
	return strings.Join(out, " · ")
}

func lookupIPAPI(ip string) (Info, error) {
	url := "http://ip-api.com/json/" + ip + "?lang=zh-CN&fields=status,message,country,regionName,city,isp,org,as,query,lat,lon,timezone"
	var raw struct {
		Status     string  `json:"status"`
		Message    string  `json:"message"`
		Country    string  `json:"country"`
		RegionName string  `json:"regionName"`
		City       string  `json:"city"`
		ISP        string  `json:"isp"`
		Org        string  `json:"org"`
		AS         string  `json:"as"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
		Timezone   string  `json:"timezone"`
	}
	if err := getJSON(url, &raw); err != nil {
		return Info{}, err
	}
	if raw.Status != "success" {
		msg := raw.Message
		if msg == "" {
			msg = "ip-api 未命中"
		}
		return Info{}, fmt.Errorf("%s", msg)
	}
	return Info{
		Country: raw.Country, Region: raw.RegionName, City: raw.City,
		ISP: raw.ISP, Org: raw.Org, ASN: raw.AS, Timezone: raw.Timezone,
		Lat: raw.Lat, Lon: raw.Lon, Source: "ip-api",
	}, nil
}

func lookupIPWho(ip string) (Info, error) {
	var raw struct {
		Success   bool    `json:"success"`
		Message   string  `json:"message"`
		Country   string  `json:"country"`
		Region    string  `json:"region"`
		City      string  `json:"city"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timezone  struct {
			ID string `json:"id"`
		} `json:"timezone"`
		Connection struct {
			ISP string `json:"isp"`
			Org string `json:"org"`
			ASN int    `json:"asn"`
		} `json:"connection"`
	}
	if err := getJSON("https://ipwho.is/"+ip, &raw); err != nil {
		return Info{}, err
	}
	if !raw.Success {
		msg := raw.Message
		if msg == "" {
			msg = "ipwho 未命中"
		}
		return Info{}, fmt.Errorf("%s", msg)
	}
	asn := ""
	if raw.Connection.ASN > 0 {
		asn = fmt.Sprintf("AS%d", raw.Connection.ASN)
	}
	return Info{
		Country: raw.Country, Region: raw.Region, City: raw.City,
		ISP: raw.Connection.ISP, Org: raw.Connection.Org, ASN: asn,
		Timezone: raw.Timezone.ID, Lat: raw.Latitude, Lon: raw.Longitude, Source: "ipwho.is",
	}, nil
}

func getJSON(url string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "watcher-ip-lookup/1.0")
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return json.Unmarshal(body, dest)
}
