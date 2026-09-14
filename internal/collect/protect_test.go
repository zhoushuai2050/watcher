package collect

import "testing"

func TestIsExemptIP(t *testing.T) {
	if !IsExemptIP("127.0.0.1", nil) {
		t.Fatal("loopback")
	}
	if !IsExemptIP("10.1.2.3", nil) {
		t.Fatal("private")
	}
	if !IsExemptIP("192.168.0.9", nil) {
		t.Fatal("rfc1918")
	}
	if !IsExemptIP("104.16.1.2", nil) {
		t.Fatal("cloudflare")
	}
	if IsExemptIP("8.8.8.8", nil) {
		t.Fatal("public dns should not be exempt")
	}
	if !IsExemptIP("203.0.113.10", []string{"203.0.113.10"}) {
		t.Fatal("explicit ignore")
	}
	if !IsExemptIP("198.51.100.20", []string{"198.51.100.0/24"}) {
		t.Fatal("ignore cidr")
	}
}

func TestNormalizeIP(t *testing.T) {
	ip, ok := NormalizeIP(" 1.2.3.4 ")
	if !ok || ip != "1.2.3.4" {
		t.Fatalf("%s %v", ip, ok)
	}
	if _, ok := NormalizeIP("not-an-ip"); ok {
		t.Fatal("expected invalid")
	}
}
