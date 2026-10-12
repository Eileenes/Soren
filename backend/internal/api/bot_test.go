package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBotScopeProtectsHistoryMemoryAndPrompt(t *testing.T) {
	_, mem, db := setupMemoryRouter(t)
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatal(err)
	}
	provider := &stubProvider{reply: "收到"}
	s, err := agent.New(db, provider, mem, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	a := agent.Bot{Name: "A", Avatar: "robo", Instructions: "ROLE_ALPHA", MemoryEnabled: true}
	b := agent.Bot{Name: "B", Avatar: "tako", Instructions: "ROLE_BETA", MemoryEnabled: true}
	s.SaveBot(&a, true)
	s.SaveBot(&b, true)
	if err := mem.StoreFact(context.Background(), agent.Owner(a.ID), "SECRET_ALPHA"); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	g := router.Group("/api")
	g.Use(BotContext(s))
	RegisterAgents(g, s)
	ch := NewChatHandler(repository.NewConversationRepository(db), provider, mem).WithAgents(s)
	mh := NewMemoryHandler(mem)
	g.POST("/chat", ch.SendMessage)
	g.GET("/conversations", ch.GetConversations)
	g.GET("/conversations/:id", ch.GetConversation)
	g.DELETE("/conversations/:id", ch.DeleteConversation)
	g.GET("/memories", mh.ListMemories)
	g.DELETE("/memories/:id", mh.DeleteMemory)
	call := func(bot, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Bot-ID", bot)
		req.Header.Set("X-User-ID", agent.Owner(a.ID))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	first := call(a.ID, "POST", "/api/chat", `{"message":"测试"}`)
	if first.Code != 200 {
		t.Fatal(first.Body.String())
	}
	var answer ChatResponse
	json.Unmarshal(first.Body.Bytes(), &answer)
	for _, method := range []string{"GET", "DELETE"} {
		res := call(b.ID, method, "/api/conversations/"+answer.ConversationID, "")
		if res.Code != 404 {
			t.Fatalf("cross bot %s allowed: %d", method, res.Code)
		}
	}
	if res := call(b.ID, "POST", "/api/chat", fmt.Sprintf(`{"conversationId":%q,"message":"越界"}`, answer.ConversationID)); res.Code != 404 {
		t.Fatal("cross bot append allowed")
	}
	list := call(b.ID, "GET", "/api/conversations", "")
	if list.Body.String() != "[]" {
		t.Fatal("history leaked", list.Body.String())
	}
	memories := call(b.ID, "GET", "/api/memories", "")
	if strings.Contains(memories.Body.String(), "SECRET_ALPHA") {
		t.Fatal("memory leaked through forged user header")
	}
	owned, _, _ := mem.List(context.Background(), memory.ListOptions{UserID: agent.Owner(a.ID), Limit: 100})
	if len(owned) == 0 {
		t.Fatal("missing seeded memory")
	}
	for _, method := range []string{"DELETE", "PUT"} {
		res := call(b.ID, method, fmt.Sprintf("/api/memories/%d", owned[0].ID), `{"content":"overwrite"}`)
		if res.Code != 404 {
			t.Fatal("cross bot memory modification allowed")
		}
	}
	if res := call(b.ID, "POST", "/api/chat", `{"message":"SECRET_ALPHA"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	all := ""
	for _, m := range provider.received {
		if m.Role == "system" {
			all += m.Content
		}
	}
	if strings.Contains(all, "SECRET_ALPHA") || !strings.Contains(all, "ROLE_BETA") {
		t.Fatal("prompt scope failed", all)
	}
	a.MemoryEnabled = false
	s.SaveBot(&a, false)
	call(a.ID, "POST", "/api/chat", `{"message":"SECRET_ALPHA"}`)
	for _, m := range provider.received {
		if m.Role == "system" && strings.Contains(m.Content, "SECRET_ALPHA") {
			t.Fatal("disabled memory still recalled")
		}
	}
}
