package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthOriginsOfflineAndLimits(t *testing.T) {
	s := New([]string{"https://mobile.example.com"})
	cases := []struct {
		method, path, origin, auth, body string
		status                           int
	}{
		{"POST", "/v1/hosts/" + RandomID() + "/exchange", "", "", "{}", 503},
		{"GET", "/v1/hosts/" + RandomID() + "/poll", "", "wrong", "", 401},
		{"POST", "/v1/hosts/" + RandomID() + "/exchange", "https://evil.example", "", "{}", 403},
		{"OPTIONS", "/v1/hosts/" + RandomID() + "/exchange", "https://mobile.example.com", "", "", 204},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		r.Header.Set("Origin", c.origin)
		r.Header.Set("Authorization", "Bearer "+c.auth)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != c.status {
			t.Fatalf("%s: %d", c.path, w.Code)
		}
	}
	token := RandomID()
	id := HostID(token)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "/v1/hosts/"+id+"/poll", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+token)
	s.ServeHTTP(httptest.NewRecorder(), r)
	payload := append([]byte(`{"padding":"`), bytes.Repeat([]byte("a"), MaxEnvelope)...)
	payload = append(payload, []byte(`"}`)...)
	r = httptest.NewRequest("POST", "/v1/hosts/"+id+"/exchange", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatal("oversized body accepted", w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if bytes.Contains(body, []byte(token)) {
		t.Fatal("token exposed")
	}
}
