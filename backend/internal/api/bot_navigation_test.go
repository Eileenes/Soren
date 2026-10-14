package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
)

func TestBotSidebarActionsPersistAndPreserveConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "bots.db")})
	if err != nil {
		t.Fatal(err)
	}
	// Bound database waits so a nested connection lookup fails instead of hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	s, err := agent.New(db, llm.NewMockProvider(), nil, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	bot := agent.Bot{Name: "Writer", Avatar: "robo", Instructions: "Keep these instructions", Model: "my-model", MemoryEnabled: true,
		CollaborationEnabled: true, Collaborators: []string{agent.DefaultBotID}}
	if err := s.SaveBot(&bot, true); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	g := router.Group("/api")
	g.Use(BotContext(s))
	RegisterAgents(g, s)
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	patch := func(body any) agent.Bot {
		t.Helper()
		w := call("PATCH", "/api/bots/"+bot.ID, body)
		if w.Code != 200 {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
		var result agent.Bot
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Instructions != bot.Instructions || result.Model != bot.Model || result.Avatar != bot.Avatar || !result.MemoryEnabled {
			t.Fatal("sidebar action overwrote Bot configuration", result)
		}
		if !result.CollaborationEnabled || len(result.Collaborators) != 1 || result.Collaborators[0] != agent.DefaultBotID {
			t.Fatal("sidebar action overwrote collaborators", result)
		}
		return result
	}

	pinned := patch(map[string]any{"version": 1, "pinned": true, "instructions": "must be ignored", "pinnedAt": "2999-01-01T00:00:00Z"})
	if !pinned.Pinned || pinned.PinnedAt == nil || pinned.PinnedAt.Year() == 2999 {
		t.Fatal(pinned)
	}
	var ordered []agent.Bot
	if err := json.Unmarshal(call("GET", "/api/bots", nil).Body.Bytes(), &ordered); err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].ID != bot.ID {
		t.Fatal("pinned Bot is not first", ordered)
	}
	again := patch(map[string]any{"version": pinned.Version, "pinned": true})
	if !again.PinnedAt.Equal(*pinned.PinnedAt) {
		t.Fatal("repeat pin changed ordering timestamp")
	}
	renamed := patch(map[string]any{"version": again.Version, "name": "  新名字  "})
	if renamed.Name != "新名字" || !renamed.Pinned || !renamed.PinnedAt.Equal(*pinned.PinnedAt) {
		t.Fatal(renamed)
	}
	stale := call("PATCH", "/api/bots/"+bot.ID, map[string]any{"version": 1, "archive": true})
	if stale.Code != 409 {
		t.Fatal("stale version accepted", stale.Code)
	}
	invalid := call("PATCH", "/api/bots/"+bot.ID, map[string]any{"version": renamed.Version, "name": strings.Repeat("字", 41), "pinned": false})
	if invalid.Code != 400 {
		t.Fatal("overlong name accepted")
	}
	stored, _ := s.GetBot(bot.ID)
	if stored.Version != renamed.Version || !stored.Pinned {
		t.Fatal("invalid mutation was not rolled back")
	}

	// Older full-editor payloads must not undo pinning.
	renamed.Pinned, renamed.PinnedAt = false, nil
	w := call("PUT", "/api/bots/"+bot.ID, renamed)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	stored, _ = s.GetBot(bot.ID)
	if !stored.Pinned || stored.PinnedAt == nil {
		t.Fatal("full edit cleared pin")
	}
	unpinned := patch(map[string]any{"version": stored.Version, "pinned": false})
	if unpinned.Pinned || unpinned.PinnedAt != nil {
		t.Fatal("unpin failed")
	}
	archived := patch(map[string]any{"version": unpinned.Version, "archive": true})
	if archived.Status != "archived" {
		t.Fatal("Bot was not archived")
	}

	// Restart preserves the metadata; the regular editor can restore an archived Bot.
	restarted, err := agent.New(db, llm.NewMockProvider(), nil, "test", true)
	if err != nil {
		t.Fatal(err)
	}
	stored, err = restarted.GetBot(bot.ID)
	if err != nil || stored.Name != "新名字" || stored.Status != "archived" {
		t.Fatal(stored, err)
	}
	stored.Status = "active"
	if err := restarted.SaveBot(stored, false); err != nil {
		t.Fatal(err)
	}
	if w := call("PATCH", "/api/bots/soren", map[string]any{"version": 1, "archive": true}); w.Code != 400 {
		t.Fatal("default Bot archived")
	}
	if w := call("PATCH", "/api/bots/missing", map[string]any{"version": 1, "pinned": true}); w.Code != 404 {
		t.Fatal("missing Bot accepted")
	}
	if w := call("PATCH", "/api/bots/"+bot.ID, map[string]any{"version": stored.Version}); w.Code != 400 {
		t.Fatal("empty patch accepted")
	}
}
