package geo

import "testing"

func TestLookupPrivate(t *testing.T) {
	info, err := Lookup("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Private || info.Location != "本机回环地址" {
		t.Fatalf("%+v", info)
	}
	info, err = Lookup("192.168.1.20")
	if err != nil || !info.Private {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := Lookup("not-an-ip"); err == nil {
		t.Fatal("want invalid")
	}
}

func TestJoinLocation(t *testing.T) {
	if joinLocation("中国", "陕西", "西安") != "中国 · 陕西 · 西安" {
		t.Fatal(joinLocation("中国", "陕西", "西安"))
	}
	if joinLocation("美国", "美国", "") != "美国" {
		t.Fatal("dedupe")
	}
}
