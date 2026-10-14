package agent

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/llm"
	"github.com/soren/backend/internal/store"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu    sync.Mutex
	calls [][]llm.Message
	block chan struct{}
}

func (p *recorder) Chat(ctx context.Context, m []llm.Message, o *llm.Options) (*llm.Response, error) {
	p.mu.Lock()
	p.calls = append(p.calls, m)
	p.mu.Unlock()
	if p.block != nil {
		select {
		case <-p.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &llm.Response{Content: "可核验的测试成果"}, nil
}
func (p *recorder) StreamChat(context.Context, []llm.Message, *llm.Options, chan<- llm.StreamChunk) error {
	return nil
}
func fixture(t *testing.T, p *recorder) *Service {
	t.Helper()
	db, err := store.OpenDB(&config.Config{StorageDriver: "sqlite", SQLitePath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(db, p, nil, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.mu.Lock()
		for _, c := range s.cancellations {
			c()
		}
		s.mu.Unlock()
	})
	return s
}
func addBot(t *testing.T, s *Service, name string) *Bot {
	t.Helper()
	b := &Bot{Name: name, Avatar: "robo", Instructions: "独立测试职责", MemoryEnabled: true}
	if err := s.SaveBot(b, true); err != nil {
		t.Fatal(err)
	}
	return b
}
func waitRun(t *testing.T, s *Service, id, state string) Run {
	t.Helper()
	for i := 0; i < 300; i++ {
		var r Run
		s.DB.First(&r, "id = ?", id)
		if r.Status == state {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run did not reach %s", state)
	return Run{}
}
func TestVersionsAndAvatarValidation(t *testing.T) {
	s := fixture(t, &recorder{})
	b := addBot(t, s, "工作")
	stale := *b
	b.Name = "工作更新"
	if err := s.SaveBot(b, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBot(&stale, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	b.Avatar = "../../other"
	if s.SaveBot(b, false) == nil {
		t.Fatal("accepted unknown avatar")
	}
}
func TestCollaborationAuthorizationAndSharedIntersection(t *testing.T) {
	p := &recorder{}
	s := fixture(t, p)
	a := addBot(t, s, "工作")
	b := addBot(t, s, "创作")
	// Legacy explicit allowlists remain enforced; new Bots default to collaboration.
	a.CollaborationAll = false
	b.CollaborationAll = false
	if err := s.SaveBot(a, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBot(b, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(a.ID, uuid.NewString(), "写方案", []string{b.ID}); err == nil {
		t.Fatal("unapproved collaboration allowed")
	}
	a.CollaborationEnabled = true
	a.Collaborators = []string{b.ID}
	b.CollaborationEnabled = true
	b.Collaborators = []string{a.ID}
	if err := s.SaveBot(a, false); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveBot(b, false); err != nil {
		t.Fatal(err)
	}
	s.DB.Create(&SharedNote{ID: uuid.NewString(), Title: "共同资料", Content: "APPROVED_CONTEXT", AllowedBots: []string{a.ID, b.ID}})
	s.DB.Create(&SharedNote{ID: uuid.NewString(), Title: "仅工作", Content: "DO_NOT_SHARE", AllowedBots: []string{a.ID}})
	key := uuid.NewString()
	r, err := s.Start(a.ID, key, "写方案", []string{b.ID})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Start(a.ID, key, "写方案", []string{b.ID})
	if err != nil || again.ID != r.ID {
		t.Fatal("retry created duplicate")
	}
	if _, err = s.Start(a.ID, key, "换一个任务", []string{b.ID}); err == nil {
		t.Fatal("reused request accepted")
	}
	result := waitRun(t, s, r.ID, "succeeded")
	if len(result.Steps) != 2 || result.Result == "" {
		t.Fatal("missing result")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.calls) != 2 {
		t.Fatalf("calls=%d", len(p.calls))
	}
	for _, call := range p.calls {
		all := ""
		for _, m := range call {
			all += m.Content
		}
		if strings.Contains(all, "DO_NOT_SHARE") || !strings.Contains(all, "APPROVED_CONTEXT") {
			t.Fatal("shared visibility failed")
		}
	}
	if !strings.Contains(p.calls[1][1].Content, "可核验的测试成果") {
		t.Fatal("downstream did not receive predecessor result")
	}
}
func TestCancellationCannotBeOverwrittenByCompletion(t *testing.T) {
	p := &recorder{block: make(chan struct{})}
	s := fixture(t, p)
	b := addBot(t, s, "测试")
	r, err := s.Start(b.ID, uuid.NewString(), "任务", nil)
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, s, r.ID, "running")
	if s.Cancel(r.ID, "soren") == nil {
		t.Fatal("cross bot cancellation allowed")
	}
	if err := s.Cancel(r.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	close(p.block)
	time.Sleep(50 * time.Millisecond)
	waitRun(t, s, r.ID, "cancelled")
}
func TestRevokedCollaborationStopsFurtherSteps(t *testing.T) {
	p := &recorder{block: make(chan struct{})}
	s := fixture(t, p)
	a := addBot(t, s, "A")
	b := addBot(t, s, "B")
	a.CollaborationEnabled = true
	a.Collaborators = []string{b.ID}
	b.CollaborationEnabled = true
	b.Collaborators = []string{a.ID}
	s.SaveBot(a, false)
	s.SaveBot(b, false)
	r, err := s.Start(a.ID, uuid.NewString(), "任务", []string{b.ID})
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, s, r.ID, "running")
	b.CollaborationEnabled = false
	if err := s.SaveBot(b, false); err != nil {
		t.Fatal(err)
	}
	close(p.block)
	waitRun(t, s, r.ID, "failed")
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.calls) > 1 {
		t.Fatal("continued after revocation")
	}
}
func TestRestartPausesInsteadOfRepeatingWork(t *testing.T) {
	s := fixture(t, &recorder{})
	r := Run{ID: uuid.NewString(), RequestID: uuid.NewString(), OwnerBotID: "soren", Goal: "任务", Status: "running", Steps: []Step{{BotID: "soren", Status: "running"}}}
	if err := s.DB.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	next, err := New(s.DB, &recorder{}, nil, "test", false)
	if err != nil {
		t.Fatal(err)
	}
	waitRun(t, next, r.ID, "paused")
	if err = next.Resume(r.ID, "soren"); err != nil {
		t.Fatal(err)
	}
	waitRun(t, next, r.ID, "succeeded")
}
