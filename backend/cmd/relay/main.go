package main

import (
	"context"
	"github.com/soren/backend/internal/relay"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := os.Getenv("RELAY_LISTEN")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	s := relay.New(strings.Split(os.Getenv("RELAY_ALLOWED_ORIGINS"), ","))
	go s.Cleanup(context.Background())
	server := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 90 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Printf("Soren relay listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
