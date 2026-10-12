// Package relay routes opaque, end-to-end encrypted envelopes. It never receives
// model credentials, plaintext operations, or pairing secrets.
package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const MaxEnvelope = 2 << 20

func HostID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func RandomID() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}

type Packet struct {
	ID       string          `json:"id"`
	Envelope json.RawMessage `json:"envelope"`
}
type pending struct{ response chan json.RawMessage }
type host struct {
	queue   chan Packet
	pending map[string]*pending
	touched time.Time
	limit   *rate.Limiter
}
type Server struct {
	mu      sync.Mutex
	hosts   map[string]*host
	limit   *rate.Limiter
	Origins map[string]bool
	flight  chan struct{}
	queued  chan struct{}
}

func New(origins []string) *Server {
	s := &Server{flight: make(chan struct{}, 128), queued: make(chan struct{}, 128), hosts: map[string]*host{}, limit: rate.NewLimiter(100, 200), Origins: map[string]bool{}}
	for _, o := range origins {
		if o != "" {
			s.Origins[o] = true
		}
	}
	return s
}
func (s *Server) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, h := range s.hosts {
				if time.Since(h.touched) > 2*time.Minute && len(h.pending) == 0 {
					delete(s.hosts, id)
				drain:
					for {
						select {
						case <-h.queue:
							<-s.queued
						default:
							break drain
						}
					}
				}
			}
			s.mu.Unlock()
		}
	}
}
func out(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	out(w, status, map[string]string{"error": message})
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if origin := r.Header.Get("Origin"); origin != "" {
		if !s.Origins[origin] {
			fail(w, 403, "origin not allowed")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(204)
		return
	}
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		out(w, 200, map[string]string{"status": "ok"})
		return
	}
	if !s.limit.Allow() {
		fail(w, 429, "relay busy")
		return
	}
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(p) != 4 || p[0] != "v1" || p[1] != "hosts" || len(p[2]) != 64 {
		fail(w, 404, "not found")
		return
	}
	id, op := p[2], p[3]
	if _, err := hex.DecodeString(id); err != nil {
		fail(w, 400, "invalid host")
		return
	}
	if op != "exchange" {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(token) != 64 || HostID(token) != id {
			fail(w, 401, "unauthorized")
			return
		}
	}
	s.mu.Lock()
	h := s.hosts[id]
	if h == nil && op == "poll" && r.Method == "GET" && len(s.hosts) < 256 {
		h = &host{queue: make(chan Packet, 8), pending: map[string]*pending{}, limit: rate.NewLimiter(10, 20)}
		s.hosts[id] = h
	}
	if h == nil {
		s.mu.Unlock()
		fail(w, 503, "Mac 离线")
		return
	}
	if op == "poll" || op == "reply" {
		h.touched = time.Now()
	}
	s.mu.Unlock()
	switch {
	case op == "poll" && r.Method == "GET":
		timer := time.NewTimer(25 * time.Second)
		defer timer.Stop()
		select {
		case packet := <-h.queue:
			<-s.queued
			out(w, 200, packet)
		case <-timer.C:
			w.WriteHeader(204)
		case <-r.Context().Done():
		}
	case op == "reply" && r.Method == "POST":
		var packet Packet
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxEnvelope)).Decode(&packet) != nil || len(packet.Envelope) == 0 {
			fail(w, 400, "invalid envelope")
			return
		}
		s.mu.Lock()
		wait := h.pending[packet.ID]
		s.mu.Unlock()
		if wait == nil {
			w.WriteHeader(410)
			return
		}
		select {
		case wait.response <- packet.Envelope:
		default:
		}
		w.WriteHeader(204)
	case op == "exchange" && r.Method == "POST":
		select {
		case s.flight <- struct{}{}:
			defer func() { <-s.flight }()
		default:
			fail(w, 429, "relay busy")
			return
		}
		var envelope json.RawMessage
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&envelope) != nil || len(envelope) == 0 {
			fail(w, 400, "invalid envelope")
			return
		}
		packet := Packet{ID: RandomID(), Envelope: envelope}
		wait := &pending{response: make(chan json.RawMessage, 1)}
		s.mu.Lock()
		if time.Since(h.touched) > 35*time.Second {
			s.mu.Unlock()
			fail(w, 503, "Mac 离线")
			return
		}
		if len(h.pending) >= 8 || !h.limit.Allow() {
			s.mu.Unlock()
			fail(w, 429, "relay busy")
			return
		}
		h.pending[packet.ID] = wait
		s.mu.Unlock()
		defer func() { s.mu.Lock(); delete(h.pending, packet.ID); s.mu.Unlock() }()
		select {
		case s.queued <- struct{}{}:
		default:
			fail(w, 429, "relay busy")
			return
		}
		select {
		case h.queue <- packet:
		default:
			<-s.queued
			fail(w, 429, "relay busy")
			return
		}
		timer := time.NewTimer(80 * time.Second)
		defer timer.Stop()
		select {
		case response := <-wait.response:
			out(w, 200, response)
		case <-timer.C:
			fail(w, 504, "Mac 未返回结果，请刷新确认状态")
		case <-r.Context().Done():
		}
	default:
		fail(w, 405, "method not allowed")
	}
}
