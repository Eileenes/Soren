package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/soren/backend/internal/group"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/models"
	"gorm.io/gorm"
)

func TestGroupRemovedMemberDoesNotReceiveLaterTurns(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil), createGroupBot(t, h, "设计", nil)}
	id := createGroup(t, h, "成员变化", ids, "auto", 2)
	provider.reply = func(call int) (string, error) {
		if call == 0 {
			updated := callGroup(t, h, http.MethodPatch, "/api/groups/"+id, map[string]any{"botIds": ids[:2]})
			if updated.Code != http.StatusOK {
				t.Errorf("update: %s", updated.Body.String())
			}
		}
		return fmt.Sprintf("reply%d", call), nil
	}
	result := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", uuid.NewString(), nil, "auto"))
	if result.Code != http.StatusOK {
		t.Fatalf("speak: %s", result.Body.String())
	}
	if provider.callCount() != 4 {
		t.Fatalf("removed member called: got %d calls, want 4", provider.callCount())
	}
	for _, msg := range storedMessages(t, h, id) {
		if msg.SpeakerBotID == ids[2] {
			t.Fatal("removed member left a reply")
		}
	}
}

func TestGroupHistoryAttributesSelfAndPeerReplies(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "身份", ids, "auto", 2)
	result := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", uuid.NewString(), nil, "auto"))
	if result.Code != http.StatusOK {
		t.Fatalf("speak: %s", result.Body.String())
	}
	for _, test := range []struct {
		call             int
		text, name, role string
	}{{1, "发言1", "产品", "user"}, {2, "发言1", "产品", "assistant"}, {2, "发言2", "研发", "user"}} {
		found := false
		for _, msg := range provider.callMessages(test.call) {
			if strings.Contains(msg.Content, test.text) {
				found = true
				if !strings.Contains(msg.Content, test.name) || msg.Role != test.role {
					t.Errorf("wrong identity: %+v", msg)
				}
			}
		}
		if !found {
			t.Errorf("missing history: %s", test.text)
		}
	}
}

func TestGroupIdempotencyIncludesMentionsAndModelOptions(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "幂等", ids, "mention", 1)
	key := uuid.NewString()
	send := func(mentions []string, config *llm.Options) int {
		body := speakBody("question", key, mentions, "mention")
		body["config"] = config
		return callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", body).Code
	}
	if got := send(ids, nil); got != 200 {
		t.Fatalf("first: %d", got)
	}
	if got := send([]string{ids[1], " " + ids[0] + " ", ids[1]}, &llm.Options{Temperature: 0.7, MaxTokens: 2000}); got != 200 {
		t.Errorf("normalized replay: %d", got)
	}
	for _, test := range []struct {
		name     string
		mentions []string
		config   *llm.Options
	}{
		{"mentions", ids[:1], nil},
		{"model", ids, &llm.Options{Model: "other-model"}},
		{"temperature", ids, &llm.Options{Temperature: 0.2}},
		{"tokens", ids, &llm.Options{MaxTokens: 100}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := send(test.mentions, test.config); got != 409 {
				t.Errorf("changed parameters: got %d, want 409", got)
			}
		})
	}
	if provider.callCount() != 2 {
		t.Fatalf("replayed model calls: %d", provider.callCount())
	}
}

func TestGroupDeleteSerializesWithAdmission(t *testing.T) {
	provider := &groupTestProvider{}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "删除竞态", ids, "mention", 1)
	reached, release := make(chan struct{}), make(chan struct{})
	var entered atomic.Bool
	if err := h.db.Callback().Query().After("gorm:query").Register("test:pause-members", func(tx *gorm.DB) {
		if tx.Statement.Table == "group_members" && entered.CompareAndSwap(false, true) {
			close(reached)
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer h.db.Callback().Query().Remove("test:pause-members")
	speak := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", uuid.NewString(), nil, "mention"))
	select {
	case <-reached:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("admission not reached")
	}
	deleted := callGroupAsync(h, http.MethodDelete, "/api/groups/"+id, nil)
	// 删除不能越过正在读取成员的发言启动阶段。
	select {
	case result := <-deleted:
		close(release)
		<-speak
		t.Fatalf("deletion bypassed admission: %d", result.Code)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case result := <-deleted:
		if result.Code != 200 {
			t.Fatalf("delete: %s", result.Body.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("delete blocked")
	}
	<-speak
	for _, model := range []any{&models.GroupChat{}, &models.GroupMember{}, &models.GroupTurn{}, &models.Conversation{}, &models.Message{}} {
		var count int64
		if err := h.db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("orphan %T: %d", model, count)
		}
	}
	if response := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("again", uuid.NewString(), nil, "mention")); response.Code != 404 {
		t.Errorf("deleted group accepted new speak: %d", response.Code)
	}
}

func TestGroupDeleteTimeoutPreservesData(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	provider := &groupTestProvider{reply: func(call int) (string, error) {
		if call == 0 {
			close(entered)
			<-release
		}
		return "reply", nil
	}}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "等待中止", ids, "auto", 1)
	done := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", uuid.NewString(), nil, "auto"))
	<-entered
	err := h.orch.Delete(context.Background(), uuid.MustParse(id), time.Millisecond)
	var ge *group.Error
	if !errors.As(err, &ge) || ge.Code != 409 {
		t.Errorf("want conflict, got %v", err)
	}
	if _, err := h.groups.GetByID(context.Background(), uuid.MustParse(id)); err != nil {
		t.Errorf("group removed before stop: %v", err)
	}
	close(release)
	<-done
	if err := h.orch.Delete(context.Background(), uuid.MustParse(id), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestGroupDetailRestoresRunningAndTerminalState(t *testing.T) {
	provider := &groupTestProvider{release: make(chan struct{}), blockFrom: 1}
	h := setupGroupHarness(t, provider, false)
	ids := []string{createGroupBot(t, h, "产品", nil), createGroupBot(t, h, "研发", nil)}
	id := createGroup(t, h, "恢复", ids, "auto", 1)
	key := uuid.NewString()
	done := callGroupAsync(h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", key, nil, "auto"))
	waitFor(t, "second call", func() bool { return provider.callCount() == 2 })
	detail := func() GroupDetailView {
		var d GroupDetailView
		response := callGroup(t, h, http.MethodGet, "/api/groups/"+id, nil)
		if response.Code != 200 {
			t.Fatalf("detail: %s", response.Body.String())
		}
		decodeGroup(t, response, &d)
		return d
	}
	running := detail()
	if running.LatestTurn == nil || running.LatestTurn.Status != "running" || running.LatestTurn.RequestID != key {
		t.Fatalf("missing running state: %+v", running.LatestTurn)
	}
	replay := callGroup(t, h, http.MethodPost, "/api/groups/"+id+"/messages", speakBody("question", key, nil, "auto"))
	var result group.SpeakResult
	decodeGroup(t, replay, &result)
	if result.Status != "running" {
		t.Errorf("replay: %+v", result)
	}
	close(provider.release)
	<-done
	finished := detail()
	if finished.LatestTurn.Status != "done" || len(finished.Messages) != 4 {
		t.Errorf("missing final history/state: %+v", finished)
	}
}
