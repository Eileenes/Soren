package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProvidersDoNotFollowRedirectsOrExposeErrorBodies(t *testing.T) {
	for _, kind := range []string{"openai", "anthropic"} {
		for _, status := range []int{302, 307, 401} {
			t.Run(kind+http.StatusText(status), func(t *testing.T) {
				var forwarded atomic.Int32
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Add(1) }))
				defer target.Close()
				const secret = "test-only-credential-should-not-escape"
				source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", target.URL)
					w.WriteHeader(status)
					_, _ = w.Write([]byte(secret))
				}))
				defer source.Close()
				var p Provider = NewOpenAIProvider(secret, source.URL)
				if kind == "anthropic" {
					p = NewAnthropicProvider(secret, source.URL)
				}
				_, err := p.Chat(context.Background(), []Message{{Role: "user", Content: "hello"}}, &Options{Model: "test", MaxTokens: 10})
				if err == nil || strings.Contains(err.Error(), secret) {
					t.Fatalf("unsafe error: %v", err)
				}
				if forwarded.Load() != 0 {
					t.Fatal("redirect forwarded a credential-bearing request")
				}
			})
		}
	}
}

func TestProviderResponseIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat(" ", maxResponseBytes+1)))
	}))
	defer server.Close()
	_, err := NewOpenAIProvider("test", server.URL).Chat(context.Background(), nil, &Options{})
	if err == nil || !strings.Contains(err.Error(), "大小限制") {
		t.Fatalf("unexpected result %v", err)
	}
}

func TestAnthropicAcceptsBaseURLWithVersionSuffix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("wrong endpoint: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()
	for _, base := range []string{server.URL, server.URL + "/", server.URL + "/v1", server.URL + "/v1/"} {
		res, err := NewAnthropicProvider("test", base).Chat(context.Background(), nil, &Options{})
		if err != nil || res.Content != "ok" {
			t.Fatalf("%v %v", res, err)
		}
	}
}
