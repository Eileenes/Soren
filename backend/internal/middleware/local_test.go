package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestLocalBoundaryRejectsRemoteAndCrossOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(LocalOnly())
	r.GET("/api/test", func(c *gin.Context) { c.Status(200) })
	for _, tt := range []struct {
		host, remote, origin string
		status               int
	}{
		{"localhost:8080", "127.0.0.1:4400", "http://localhost:1420", 200},
		{"localhost:8080", "127.0.0.1:4400", "https://evil.example", 403},
		{"evil.example:8080", "127.0.0.1:4400", "", 403},
		{"localhost:8080", "192.168.1.2:4400", "", 403},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "http://"+tt.host+"/api/test", nil)
		req.RemoteAddr = tt.remote
		req.Header.Set("Origin", tt.origin)
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		r.ServeHTTP(w, req)
		if w.Code != tt.status {
			t.Fatalf("%+v: %d", tt, w.Code)
		}
	}
}
