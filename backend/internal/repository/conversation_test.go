package repository_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/soren/backend/internal/config"
	"github.com/soren/backend/internal/memory"
	"github.com/soren/backend/internal/models"
	"github.com/soren/backend/internal/repository"
	"github.com/soren/backend/internal/store"
	"gorm.io/gorm"
)

func TestDeleteConversationRemovesItsMessagesAndRetainsMemories(t *testing.T) {
	db, err := store.OpenDB(&config.Config{
		StorageDriver: config.StorageDriverSQLite,
		SQLitePath:    filepath.Join(t.TempDir(), "delete.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	tables := append([]interface{}{&models.Conversation{}, &models.Message{}}, memory.AllModels()...)
	if err := db.AutoMigrate(tables...); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	repo := repository.NewConversationRepository(db)
	target := &models.Conversation{Title: "delete this conversation"}
	other := &models.Conversation{Title: "keep this conversation"}
	for _, conversation := range []*models.Conversation{target, other} {
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddMessage(ctx, &models.Message{
			ConversationID: conversation.ID, Role: "user", Content: "hello",
		}); err != nil {
			t.Fatal(err)
		}
	}
	preference := &memory.Memory{
		UserID: memory.DefaultUserID, SessionID: target.ID.String(),
		ContentType: memory.MemoryTypePreference, Content: "请用中文回答",
	}
	if err := db.Create(preference).Error; err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, target.ID); err != nil {
		t.Fatalf("conversation with messages must be deletable: %v", err)
	}
	if _, err := repo.GetByID(ctx, target.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted conversation still exists: %v", err)
	}
	if messages, err := repo.GetMessages(ctx, target.ID, 0); err != nil || len(messages) != 0 {
		t.Fatalf("deleted conversation left messages: %v, %v", messages, err)
	}
	if preserved, err := repo.GetByID(ctx, other.ID); err != nil || len(preserved.Messages) != 1 {
		t.Fatalf("another conversation was affected: %v, %v", preserved, err)
	}
	var remaining memory.Memory
	if err := db.First(&remaining, preference.ID).Error; err != nil || remaining.Content != preference.Content {
		t.Fatalf("independent long-term memory must be retained: %v, %v", remaining, err)
	}
}

func TestReplyMovesExistingConversationToTop(t *testing.T) {
	db, err := store.OpenDB(&config.Config{StorageDriver: config.StorageDriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "recents.db")})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Conversation{}, &models.Message{}); err != nil {
		t.Fatal(err)
	}
	repo := repository.NewConversationRepository(db)
	ctx := context.Background()
	old := &models.Conversation{Title: "old", BotID: "soren", UpdatedAt: time.Now().Add(-time.Hour)}
	recent := &models.Conversation{Title: "recent", BotID: "soren"}
	for _, c := range []*models.Conversation{old, recent} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.AddMessage(ctx, &models.Message{ConversationID: old.ID, Role: "user", Content: "continue"}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.GetAllForBot(ctx, "soren")
	if err != nil || len(list) != 2 || list[0].ID != old.ID {
		t.Fatal("replied conversation did not move to top", err)
	}
}
