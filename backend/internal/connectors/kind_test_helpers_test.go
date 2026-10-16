package connectors

import (
	"context"
	"fmt"

	"github.com/soren/backend/internal/llm"

	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var runSeq int

// kindFixture wires a fake upstream for one Baizhi connector kind granted to bot "a".
func kindFixture(t *testing.T, kind string, h http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()
	s := fixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key-1" && !strings.HasPrefix(r.URL.Path, "/public/") {
			w.WriteHeader(401)
			return
		}
		h(w, r)
	}))
	t.Cleanup(server.Close)
	s.APIBase = map[string]string{kind: server.URL}
	c := Connection{ID: "k1", Kind: kind, Name: kind, AllowedBots: []string{"a"}, Enabled: true, Version: 1}
	if e := s.DB.Create(&c).Error; e != nil {
		t.Fatal(e)
	}
	if e := s.Vault.Set("k1", Credential{APIKey: "key-1"}); e != nil {
		t.Fatal(e)
	}
	return s, server
}

func call(t *testing.T, s *Service, action, args string) (string, error) {
	t.Helper()
	return loopResult(t, s, &scripted{name: "c_k1_" + action, args: args})
}

func loopResult(t *testing.T, s *Service, p *scripted) (string, error) {
	t.Helper()
	runSeq++
	run := fmt.Sprintf("kr%d", runSeq)
	s.DB.Exec("INSERT INTO runs VALUES (?, 'a', 'running')", run)
	if _, e := s.Loop(context.Background(), p, "a", run, 0, []llm.Message{{Role: "user", Content: "task"}}, &llm.Options{Model: "test"}); e != nil {
		return "", e
	}
	var inv Invocation
	if e := s.DB.Order("created_at DESC").First(&inv).Error; e != nil {
		t.Fatal(e)
	}
	if inv.Status != "succeeded" {
		return "", errors.New(inv.Status + ": " + inv.Result)
	}
	return inv.Result, nil
}

func assertApprovalRequired(t *testing.T, s *Service, approval map[string]bool) {
	t.Helper()
	catalog, e := s.catalog("a")
	if e != nil {
		t.Fatal(e)
	}
	for action, want := range approval {
		b, ok := catalog["c_k1_"+action]
		if !ok {
			t.Fatalf("missing tool %s", action)
		}
		if b.Approval != want {
			t.Fatalf("%s approval = %v, want %v", action, b.Approval, want)
		}
	}
	if _, ok := catalog["c_k1_"+"nope"]; ok {
		t.Fatal("unexpected tool")
	}
	if b, e := s.catalog("b"); e != nil || len(b) != len(catalog) {
		t.Fatal("workspace connection missing from other bot", len(b))
	}
}

func mustFile(t *testing.T, result, want string) string {
	t.Helper()
	var out struct {
		Path string `json:"path"`
	}
	if e := decode(result, &out); e != nil || out.Path == "" {
		t.Fatal(result)
	}
	b, e := os.ReadFile(out.Path)
	if e != nil || string(b) != want {
		t.Fatalf("file %s = %q", out.Path, b)
	}
	return out.Path
}

func newTLSStorage(t *testing.T, body string) *httptest.Server {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("API key forwarded to storage")
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTLSRedirect(t *testing.T, to string) *httptest.Server {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key-1" {
			w.WriteHeader(401)
			return
		}
		http.Redirect(w, r, to, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}
