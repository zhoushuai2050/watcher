package main

import (
	"context"
	"log"
	"os"

	"watcher/internal/api"
	"watcher/internal/auth"
	"watcher/internal/collect"
	"watcher/internal/config"
	"watcher/internal/detect"
	"watcher/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()
	st, err := store.Open(cfg.DBPath())
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	au := auth.New(st, cfg)
	if err := au.Bootstrap(cfg.AdminUser, cfg.AdminPassword); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	eng := collect.New(st, cfg)
	det := detect.New(st, cfg, eng)
	eng.OnEvent = det.OnEvent

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if os.Getenv("WATCHER_NO_COLLECT") != "1" {
		go eng.Start(ctx)
		go det.Start(ctx)
	}

	srv := api.New(cfg, st, au, eng, det)
	if err := srv.Listen(); err != nil {
		log.Fatal(err)
	}
}
