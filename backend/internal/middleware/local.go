package middleware

import (
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
	"strings"
)

// LocalOnly is a local-owner boundary, not cloud authentication. In particular,
// binding a GitHub identity does not make this API safe to expose remotely.
func LocalOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		if h, _, e := net.SplitHostPort(host); e == nil {
			host = h
		}
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		origin := c.GetHeader("Origin")
		allowedOrigin := origin == "" || origin == "http://localhost:1420" || origin == "http://127.0.0.1:1420" || origin == "tauri://localhost" || origin == "http://tauri.localhost" || origin == "https://tauri.localhost"
		if (host != "localhost" && host != "127.0.0.1" && host != "::1") || !net.ParseIP(ip).IsLoopback() || !allowedOrigin {
			c.AbortWithStatusJSON(403, gin.H{"error": "仅允许本机可信界面访问"})
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "OPTIONS" && c.GetHeader("X-Bot-ID") == "" && !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			c.AbortWithStatus(http.StatusUnsupportedMediaType)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
