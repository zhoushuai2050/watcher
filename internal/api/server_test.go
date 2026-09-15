package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"watcher/internal/auth"
	"watcher/internal/collect"
	"watcher/internal/config"
	"watcher/internal/detect"
	"watcher/internal/store"
)

func TestLoginAndOverview(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	cfg.Listen = "127.0.0.1:0"
	au := auth.New(st, cfg)
	if err := au.Bootstrap("admin", "watcher"); err != nil {
		t.Fatal(err)
	}
	eng := collect.New(st, cfg)
	det := detect.New(st, cfg, eng)
	srv := New(cfg, st, au, eng, det)
	h := srv.Handler()

	body, _ := json.Marshal(map[string]string{"name": "admin", "password": "watcher"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || env.Data.Token == "" {
		t.Fatalf("%s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/Watcher/api/v1/overview", nil)
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("overview %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("want 401 unauth, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/Watcher/favicon.svg", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("favicon %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("content-type %s", rec.Header().Get("Content-Type"))
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("<svg")) {
		t.Fatal("favicon is not svg")
	}

	defer collect.SetRunNft(func(args ...string) (string, error) { return "", nil })()
	banBody, _ := json.Marshal(map[string]string{"ip": "203.0.113.8", "reason": "test"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/security/bans", bytes.NewReader(banBody))
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("ban %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/security/bans", nil)
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("203.0.113.8")) {
		t.Fatalf("list bans %d %s", rec.Code, rec.Body.String())
	}
	unbanBody, _ := json.Marshal(map[string]string{"ip": "203.0.113.8"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/security/unban", bytes.NewReader(unbanBody))
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("unban %d %s", rec.Code, rec.Body.String())
	}
}

func TestAllowlistAPI(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	au := auth.New(st, cfg)
	if err := au.Bootstrap("admin", "watcher"); err != nil {
		t.Fatal(err)
	}
	defer collect.SetRunNft(func(args ...string) (string, error) { return "", nil })()
	eng := collect.New(st, cfg)
	det := detect.New(st, cfg, eng)
	h := New(cfg, st, au, eng, det).Handler()
	body, _ := json.Marshal(map[string]string{"name": "admin", "password": "watcher"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env struct {
		Data struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Data.Token == "" {
		t.Fatalf("login %s", rec.Body.String())
	}
	authz := "Bearer " + env.Data.Token
	addBody, _ := json.Marshal(map[string]string{"spec": "203.0.113.9", "note": "home"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/security/allow", bytes.NewReader(addBody))
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("add %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/security/allow", nil)
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("203.0.113.9")) {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	banBody, _ := json.Marshal(map[string]string{"ip": "203.0.113.9"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/security/bans", bytes.NewReader(banBody))
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("ban whitelist want 400 got %d %s", rec.Code, rec.Body.String())
	}
}

func TestLookupPrivateIP(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	au := auth.New(st, cfg)
	if err := au.Bootstrap("admin", "watcher"); err != nil {
		t.Fatal(err)
	}
	eng := collect.New(st, cfg)
	det := detect.New(st, cfg, eng)
	h := New(cfg, st, au, eng, det).Handler()
	body, _ := json.Marshal(map[string]string{"name": "admin", "password": "watcher"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env struct {
		Data struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Data.Token == "" {
		t.Fatalf("login %s", rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/network/ip?ip=127.0.0.1", nil)
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte("本机回环地址")) {
		t.Fatalf("lookup %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/network/ip?ip=bad", nil)
	req.Header.Set("Authorization", "Bearer "+env.Data.Token)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("bad ip want 400 got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAlertBatchAckAndResolve(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Defaults()
	au := auth.New(st, cfg)
	if err := au.Bootstrap("admin", "watcher"); err != nil {
		t.Fatal(err)
	}
	if err := st.RaiseAlert("ssh.bruteforce", "203.0.113.1", "high", "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.RaiseAlert("web.probe", "203.0.113.2", "medium", "b"); err != nil {
		t.Fatal(err)
	}
	open, err := st.OpenAlerts("")
	if err != nil || len(open) != 2 {
		t.Fatalf("open=%d err=%v", len(open), err)
	}
	eng := collect.New(st, cfg)
	det := detect.New(st, cfg, eng)
	h := New(cfg, st, au, eng, det).Handler()

	body, _ := json.Marshal(map[string]string{"name": "admin", "password": "watcher"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env struct {
		Data struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Data.Token == "" {
		t.Fatalf("login %s", rec.Body.String())
	}
	authz := "Bearer " + env.Data.Token

	ackBody, _ := json.Marshal(map[string]any{"ids": []int64{open[0].ID, open[1].ID}})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/ack", bytes.NewReader(ackBody))
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"updated":2`)) {
		t.Fatalf("ack %d %s", rec.Code, rec.Body.String())
	}

	resBody, _ := json.Marshal(map[string]any{"ids": []int64{open[0].ID, open[1].ID}})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/resolve", bytes.NewReader(resBody))
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(`"updated":2`)) {
		t.Fatalf("resolve %d %s", rec.Code, rec.Body.String())
	}

	left, err := st.OpenAlerts("")
	if err != nil || len(left) != 0 {
		t.Fatalf("left=%+v err=%v", left, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/alerts/ack", bytes.NewReader([]byte(`{"ids":[]}`)))
	req.Header.Set("Authorization", authz)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("empty ids want 400 got %d %s", rec.Code, rec.Body.String())
	}
}
