package channel

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soren/backend/internal/config"
)

func TestWebhookRequiresAuthenticationBeforeStarting(t *testing.T) {
	feishu := NewFeishuChannel(config.FeishuConfig{AppID: "test", AppSecret: "test", Mode: config.ChannelModeWebhook})
	ding := NewDingTalkChannel(config.DingTalkConfig{ClientID: "test", ClientSecret: "test", Mode: config.ChannelModeWebhook})
	for _, ch := range []Channel{feishu, ding} {
		if err := ch.Start(context.Background()); err == nil {
			t.Fatal("unauthenticated webhook started")
		}
	}
	for _, handler := range []func(*httptest.ResponseRecorder){
		func(w *httptest.ResponseRecorder) {
			feishu.handleWebhook(w, httptest.NewRequest("POST", "/", strings.NewReader(`{}`)))
		},
		func(w *httptest.ResponseRecorder) {
			ding.handleWebhook(w, httptest.NewRequest("POST", "/", strings.NewReader(`{}`)))
		},
	} {
		w := httptest.NewRecorder()
		handler(w)
		if w.Code != 503 {
			t.Fatalf("unsafe webhook accepted request: %d", w.Code)
		}
	}
}
func TestDingTalkRejectsUntrustedReplyTargets(t *testing.T) {
	ch := NewDingTalkChannel(config.DingTalkConfig{})
	for _, target := range []string{"https://example.com/reply", "https://127.0.0.1/", "https://oapi.dingtalk.com.evil.example/", "https://oapi.dingtalk.com:8443/", "http://oapi.dingtalk.com/", "https://user@oapi.dingtalk.com/"} {
		if err := ch.postJSON(context.Background(), target, map[string]any{}); err == nil || !strings.Contains(err.Error(), "invalid session webhook") {
			t.Fatalf("untrusted target wasn't rejected: %v", err)
		}
	}
}
