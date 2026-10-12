package config

import "testing"

func TestServerDefaultsToLoopback(t *testing.T) {
	t.Setenv("SERVER_HOST", "")
	if got := Load().ServerHost; got != "127.0.0.1" {
		t.Fatalf("server must default to loopback, got %q", got)
	}
}

func TestServerHostCanBeConfigured(t *testing.T) {
	t.Setenv("SERVER_HOST", "::1")
	if got := Load().ServerHost; got != "::1" {
		t.Fatalf("explicit server host was ignored: %q", got)
	}
}

func TestModelComesFromDesktopEnvironment(t *testing.T) {
	t.Setenv("SOREN_MODEL", "  custom-chat-model  ")
	if got := Load().Model; got != "custom-chat-model" {
		t.Fatalf("model selection lost: %q", got)
	}
}
