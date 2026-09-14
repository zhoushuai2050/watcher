package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"watcher/internal/auth"
	"watcher/internal/collect"
	"watcher/internal/config"
	"watcher/internal/detect"
	"watcher/internal/store"
	"watcher/web"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	auth      *auth.Service
	engine    *collect.Engine
	detect    *detect.Detector
	loginMu   sync.Mutex
	loginHits map[string][]time.Time
}

func New(cfg config.Config, st *store.Store, au *auth.Service, eng *collect.Engine, det *detect.Detector) *Server {
	return &Server{
		cfg: cfg, store: st, auth: au, engine: eng, detect: det,
		loginHits: map[string][]time.Time{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("GET /api/v1/me", s.authed(s.me))
	mux.HandleFunc("POST /api/v1/me/password", s.authed(s.changePassword))
	mux.HandleFunc("GET /api/v1/overview", s.authed(s.overview))
	mux.HandleFunc("GET /api/v1/metrics", s.authed(s.metrics))
	mux.HandleFunc("GET /api/v1/processes", s.authed(s.processes))
	mux.HandleFunc("GET /api/v1/processes/history", s.authed(s.processHistory))
	mux.HandleFunc("GET /api/v1/network", s.authed(s.network))
	mux.HandleFunc("GET /api/v1/security/events", s.authed(s.events))
	mux.HandleFunc("GET /api/v1/security/ips", s.authed(s.ips))
	mux.HandleFunc("GET /api/v1/security/bans", s.authed(s.bans))
	mux.HandleFunc("POST /api/v1/security/bans", s.authed(s.postBan))
	mux.HandleFunc("POST /api/v1/security/unban", s.authed(s.postUnban))
	mux.HandleFunc("GET /api/v1/alerts", s.authed(s.alerts))
	mux.HandleFunc("POST /api/v1/alerts/ack", s.authed(s.ackAlerts))
	mux.HandleFunc("POST /api/v1/alerts/resolve", s.authed(s.resolveAlerts))
	mux.HandleFunc("POST /api/v1/alerts/{id}/ack", s.authed(s.ackAlert))
	mux.HandleFunc("POST /api/v1/alerts/{id}/resolve", s.authed(s.resolveAlert))
	mux.HandleFunc("GET /api/v1/settings", s.authed(s.getSettings))
	mux.HandleFunc("PUT /api/v1/settings", s.authed(s.putSettings))
	mux.HandleFunc("GET /api/v1/collectors", s.authed(s.collectors))
	mux.HandleFunc("/", s.ui)
	return s.stripPrefix(mux)
}

func (s *Server) Listen() error {
	h := s.Handler()
	log.Printf("watcher listening on http://%s%s/", s.cfg.Listen, s.cfg.PublicPath)
	return http.ListenAndServe(s.cfg.Listen, h)
}

func (s *Server) stripPrefix(next http.Handler) http.Handler {
	prefix := strings.TrimRight(s.cfg.PublicPath, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if prefix != "" && (r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/")) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		next.ServeHTTP(w, r)
	})
}

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status, code int, msg string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Code: code, Message: msg, Data: data})
}

func ok(w http.ResponseWriter, data any) { writeJSON(w, 200, 0, "ok", data) }

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, status, msg, nil)
}

func (s *Server) authed(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			fail(w, 401, "未登录")
			return
		}
		id, err := s.auth.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil || id == 0 {
			fail(w, 401, "登录已过期")
			return
		}
		u, err := s.store.GetUserByID(id)
		if err != nil || u == nil {
			fail(w, 401, "用户不存在")
			return
		}
		r.Header.Set("X-User-Id", strconv.FormatInt(u.ID, 10))
		r.Header.Set("X-User-Name", u.Name)
		fn(w, r)
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	ok(w, map[string]any{"name": "watcher", "db": "sqlite"})
}

func (s *Server) allowLogin(ip string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := time.Now()
	hits := s.loginHits[ip]
	kept := hits[:0]
	for _, t := range hits {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 10 {
		s.loginHits[ip] = kept
		return false
	}
	s.loginHits[ip] = append(kept, now)
	return true
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	if !s.allowLogin(ip) {
		fail(w, 429, "尝试过于频繁")
		return
	}
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	tok, user, err := s.auth.Login(strings.TrimSpace(body.Name), body.Password)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if user == nil || tok == "" {
		fail(w, 401, "用户名或密码错误")
		return
	}
	ok(w, map[string]any{"access_token": tok, "user": map[string]any{"id": user.ID, "name": user.Name}})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{"id": r.Header.Get("X-User-Id"), "name": r.Header.Get("X-User-Name")})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.New) < 6 {
		fail(w, 400, "新密码至少 6 位")
		return
	}
	id, _ := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
	if err := s.auth.UpdatePassword(id, body.Old, body.New); err != nil {
		if auth.IsWrongPassword(err) {
			fail(w, 400, err.Error())
			return
		}
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true})
}

func (s *Server) overview(w http.ResponseWriter, _ *http.Request) {
	host, procs, groups, _ := s.engine.Snapshot()
	alerts, _ := s.store.ListAlerts("active", 8)
	events, _ := s.store.QueryEvents("", "", time.Now().Unix()-86400, 10)
	metrics, _ := s.store.QueryMetrics(time.Now().Unix()-3600, 5)
	top := procs
	if len(top) > 5 {
		top = top[:5]
	}
	ok(w, map[string]any{
		"host":          host,
		"alerts_open":   len(alerts),
		"alerts":        alerts,
		"recent_events": events,
		"top_processes": top,
		"groups":        groups,
		"sparkline":     metrics,
		"collectors":    s.engine.Status(),
	})
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	rng := r.URL.Query().Get("range")
	now := time.Now().Unix()
	from, step := now-3600, 5
	switch rng {
	case "6h":
		from, step = now-6*3600, 60
	case "24h":
		from, step = now-86400, 60
	case "7d":
		from, step = now-7*86400, 300
	}
	rows, err := s.store.QueryMetrics(from, step)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) processes(w http.ResponseWriter, _ *http.Request) {
	_, procs, groups, _ := s.engine.Snapshot()
	if len(procs) == 0 {
		var err error
		procs, err = s.store.LatestProcesses()
		if err != nil {
			fail(w, 500, err.Error())
			return
		}
	}
	ok(w, map[string]any{"items": procs, "groups": groups})
}

func (s *Server) processHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := time.Now().Unix() - 3600
	if v := q.Get("range"); v == "6h" {
		from = time.Now().Unix() - 6*3600
	} else if v == "24h" {
		from = time.Now().Unix() - 86400
	}
	rows, err := s.store.ProcessHistory(q.Get("name"), q.Get("container"), from)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) network(w http.ResponseWriter, _ *http.Request) {
	_, _, _, net := s.engine.Snapshot()
	settings := s.store.LoadSettings(s.cfg.Settings)
	allow := map[string]bool{}
	for _, spec := range settings.ListenAllow {
		allow[strings.ToLower(spec)] = true
	}
	for i := range net.Listen {
		key := net.Listen[i].Proto + ":" + strconv.Itoa(net.Listen[i].Port)
		net.Listen[i].Allowed = allow[key]
	}
	ok(w, net)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := time.Now().Unix() - 7*86400
	if v := q.Get("from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			from = n
		}
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	rows, err := s.store.QueryEvents(q.Get("source"), q.Get("ip"), from, limit)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) ips(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.store.QueryIPAgg(time.Now().Unix() - 7*86400)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) bans(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.store.ListBans()
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) postBan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP     string `json:"ip"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "手动永久封禁"
	}
	if err := s.engine.BanPermanent(body.IP, reason, false); err != nil {
		fail(w, 400, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true, "ip": strings.TrimSpace(body.IP)})
}

func (s *Server) postUnban(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if err := s.engine.UnbanPermanent(body.IP); err != nil {
		fail(w, 400, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true})
}

func (s *Server) alerts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	rows, err := s.store.ListAlerts(status, 200)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, rows)
}

func (s *Server) ackAlert(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.SetAlertStatus(id, "acked"); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true})
}

func (s *Server) resolveAlert(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.store.ResolveAlert(id); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true})
}

func readAlertIDs(r *http.Request) ([]int64, error) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(body.IDs))
	for _, id := range body.IDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if len(out) >= 200 {
			break
		}
	}
	return out, nil
}

func (s *Server) ackAlerts(w http.ResponseWriter, r *http.Request) {
	ids, err := readAlertIDs(r)
	if err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if len(ids) == 0 {
		fail(w, 400, "未选择告警")
		return
	}
	n, err := s.store.SetAlertStatusMany(ids, "acked")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true, "updated": n})
}

func (s *Server) resolveAlerts(w http.ResponseWriter, r *http.Request) {
	ids, err := readAlertIDs(r)
	if err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if len(ids) == 0 {
		fail(w, 400, "未选择告警")
		return
	}
	n, err := s.store.SetAlertStatusMany(ids, "resolved")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"ok": true, "updated": n})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	st := s.store.LoadSettings(s.cfg.Settings)
	defaultPwd := false
	if u, err := s.store.GetUserByName(s.cfg.AdminUser); err == nil && u != nil {
		defaultPwd = auth.CheckPassword(u.PasswordHash, "watcher")
	}
	ok(w, map[string]any{"settings": st, "collectors": s.engine.Status(), "default_password": defaultPwd})
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	st := s.store.LoadSettings(s.cfg.Settings)
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		fail(w, 400, "请求格式错误")
		return
	}
	if err := s.store.SaveSettings(st); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if s.detect != nil {
		go s.detect.Tick()
	}
	ok(w, st)
}

func (s *Server) collectors(w http.ResponseWriter, _ *http.Request) {
	ok(w, s.engine.Status())
}

func (s *Server) ui(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	sub, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
		if b, err := fs.ReadFile(sub, rel); err == nil {
			switch path.Ext(rel) {
			case ".svg":
				w.Header().Set("Content-Type", "image/svg+xml")
			case ".js":
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			case ".css":
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case ".ico":
				w.Header().Set("Content-Type", "image/x-icon")
			}
			_, _ = w.Write(b)
			return
		}
	}
	b, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(500)
			_, _ = w.Write([]byte("frontend not built"))
			return
		}
		fail(w, 500, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
