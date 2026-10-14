package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/models"
)

func TestGroupMessageDeterminesRoundsAndWaitsForContinuation(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	// A legacy auto group with a six-round setting must still default to one reply.
	id := createGroup(t, h, "讨论", ids, "auto", 6)
	speak := func(content, key string, mentions []string) group.SpeakResult {
		t.Helper()
		response := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody(content, key, mentions, ""))
		if response.Code != 200 {
			t.Fatalf("speak: %s", response.Body.String())
		}
		var result group.SpeakResult
		decodeGroup(t, response, &result)
		return result
	}
	first := speak("大家觉得这个方案怎么样？", uuid.NewString(), nil)
	if first.Mode != "mention" || provider.callCount() != 2 || len(first.Messages) != 3 {
		t.Fatalf("ordinary question should call each bot once: %+v calls=%d", first, provider.callCount())
	}
	speak("@产品 请给我建议", uuid.NewString(), ids[:1])
	if provider.callCount() != 3 {
		t.Fatal("mention must only invite one bot")
	}
	key := uuid.NewString()
	discussion := speak("请大家讨论一下这个方案", key, nil)
	if discussion.Mode != "auto" || provider.callCount() != 9 {
		t.Fatalf("discussion must stop at three rounds: calls=%d", provider.callCount())
	}
	last := discussion.Messages[len(discussion.Messages)-1]
	if last.Role != "system" || !strings.Contains(last.Content, "要继续讨论吗") {
		t.Fatal("missing continuation invitation", last)
	}
	if !provider.hasContent(8, "第 3/3 轮") {
		t.Fatal("last bot did not receive round budget")
	}
	replay := speak("请大家讨论一下这个方案", key, nil)
	if provider.callCount() != 9 || replay.Messages[len(replay.Messages)-1].ID != last.ID {
		t.Fatal("replay repeated model calls or continuation notice")
	}
	speak("继续", uuid.NewString(), nil)
	if provider.callCount() != 15 {
		t.Fatal("explicit continuation should run one bounded batch")
	}
	speak("不要讨论，大家只回答一轮", uuid.NewString(), nil)
	if provider.callCount() != 17 {
		t.Fatal("single round request did not stop discussion")
	}
	speak("讨论会什么时候开始？", uuid.NewString(), nil)
	if provider.callCount() != 19 {
		t.Fatal("mentioning discussion should not start one")
	}
	speak("大家讨论的结果是什么？", uuid.NewString(), nil)
	if provider.callCount() != 21 {
		t.Fatal("asking about discussion results should only call each bot once")
	}
}

func TestBotCreationAndFirstReplyPersistResponsibilitiesWithoutModelCall(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	routes := h.router.Group("/api")
	routes.Use(BotContext(h.agents))
	RegisterAgents(routes, h.agents)
	chat := NewChatHandler(h.convRepo, provider, nil).WithAgents(h.agents)
	routes.POST("/chat", chat.SendMessage)
	call := func(bot, method, path string, input any) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(input)
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Bot-ID", bot)
		response := httptest.NewRecorder()
		h.router.ServeHTTP(response, req)
		return response
	}
	created := call("soren", "POST", "/api/bots", map[string]any{"name": "资料助手", "avatar": "robo", "memoryEnabled": true})
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	var bot agent.Bot
	json.Unmarshal(created.Body.Bytes(), &bot)
	if bot.Instructions != "" || !bot.CollaborationEnabled || !bot.CollaborationAll || bot.Model != "" {
		t.Fatalf("wrong creation defaults: %+v", bot)
	}
	// Failed initialization must leave no orphan conversation or role.
	bad := call(bot.ID, "POST", "/api/chat", map[string]any{"message": strings.Repeat("字", 8001)})
	if bad.Code != 400 {
		t.Fatal("oversize role accepted")
	}
	var count int64
	h.db.Model(&models.Conversation{}).Where("bot_id = ?", bot.ID).Count(&count)
	if count != 0 {
		t.Fatal("failed onboarding created an orphan conversation")
	}
	role := "负责整理客户反馈，每周交付一份简洁的分析报告。"
	answer := call(bot.ID, "POST", "/api/chat", map[string]any{"message": role})
	if answer.Code != 200 {
		t.Fatal(answer.Body.String())
	}
	var result ChatResponse
	json.Unmarshal(answer.Body.Bytes(), &result)
	saved, err := h.agents.GetBot(bot.ID)
	if err != nil || saved.Instructions != role || saved.Version != 2 {
		t.Fatalf("role not persisted: %+v %v", saved, err)
	}
	if provider.callCount() != 0 {
		t.Fatal("role setup wasted a model call")
	}
	var history []models.Message
	h.db.Where("conversation_id = ?", result.ConversationID).Order("timestamp ASC").Find(&history)
	if len(history) != 3 || !strings.Contains(history[0].Content, "我的职责是什么") || history[1].Content != role {
		t.Fatalf("missing onboarding history: %+v", history)
	}
	next := call(bot.ID, "POST", "/api/chat", map[string]any{"message": "先整理今天的反馈", "conversationId": result.ConversationID})
	if next.Code != 200 || provider.callCount() != 1 || !provider.hasContent(0, role) {
		t.Fatal("subsequent chat did not use saved role", next.Body.String())
	}
}
