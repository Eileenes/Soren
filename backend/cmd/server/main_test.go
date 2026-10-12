package main

import (
	"github.com/soren/backend/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func TestAppCORSAllowsOnlyTrustedOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(cors.New(appCORSConfig()))
	router.GET("/api/health", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, test := range []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:1420", true},
		{"http://127.0.0.1:1420", true},
		{"tauri://localhost", true},
		{"http://tauri.localhost", true},
		{"https://tauri.localhost", true},
		{"https://untrusted.example", false},
		{"http://localhost:1420.untrusted.example", false},
		{"http://127.0.0.1:1421", false},
		{"null", false},
	} {
		t.Run(test.origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", "GET")
			request.Header.Set("Access-Control-Request-Headers", "X-User-ID")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if test.allowed {
				if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != test.origin {
					t.Fatalf("trusted app origin rejected: status=%d headers=%v", response.Code, response.Header())
				}
			} else if response.Code != http.StatusForbidden || response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatalf("untrusted origin allowed: status=%d headers=%v", response.Code, response.Header())
			}
		})
	}
}

func TestDefaultModelUsesDesktopSelection(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		cfg := &config.Config{Model: "my-provider-model"}
		if provider == "anthropic" {
			cfg.AnthropicAPIKey = "test-key"
		}
		if got := defaultModel(cfg); got != cfg.Model {
			t.Fatalf("selected model ignored: %s", got)
		}
	}
}
