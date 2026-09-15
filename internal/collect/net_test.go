package collect

import "testing"

func TestIPFamily(t *testing.T) {
	if ipFamily(2, "0.0.0.0") != "ipv4" {
		t.Fatal("ipv4 wildcard")
	}
	if ipFamily(10, "::") != "ipv6" {
		t.Fatal("ipv6 wildcard")
	}
	if ipFamily(0, "2001:db8::1") != "ipv6" {
		t.Fatal("ipv6 from addr")
	}
	if displayAddr("0.0.0.0") != "*" || displayAddr("::") != "*" {
		t.Fatal("wildcard display")
	}
}
