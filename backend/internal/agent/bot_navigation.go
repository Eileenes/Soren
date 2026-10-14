package agent

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// BotNavigationPatch updates only sidebar metadata; stale forms cannot overwrite
// the Bot's instructions, permissions, avatar, or model configuration.
type BotNavigationPatch struct {
	Version int     `json:"version"`
	Name    *string `json:"name"`
	Pinned  *bool   `json:"pinned"`
	Archive bool    `json:"archive"`
}

func (s *Service) UpdateBotNavigation(id string, patch BotNavigationPatch) (*Bot, error) {
	if patch.Version < 1 || (patch.Name == nil && patch.Pinned == nil && !patch.Archive) {
		return nil, errors.New("请提供有效版本和要更新的字段")
	}
	var bot Bot
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&bot, "id = ?", id).Error; err != nil {
			return err
		}
		if bot.Version != patch.Version {
			return ErrConflict
		}
		if patch.Name != nil {
			bot.Name = *patch.Name
		}
		if patch.Pinned != nil {
			bot.Pinned = *patch.Pinned
			if !bot.Pinned {
				bot.PinnedAt = nil
			} else if bot.PinnedAt == nil {
				now := time.Now()
				bot.PinnedAt = &now
			}
		}
		if patch.Archive {
			bot.Status = "archived"
			bot.Pinned, bot.PinnedAt = false, nil
		}
		// Collaborator lookup must reuse this transaction's connection.
		if err := validateBot(tx, &bot); err != nil {
			return err
		}
		bot.Version++
		bot.UpdatedAt = time.Now()
		result := tx.Model(&Bot{}).Where("id = ? AND version = ?", id, patch.Version).
			Select("name", "status", "pinned", "pinned_at", "version", "updated_at").Updates(&bot)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		snapshot, err := json.Marshal(bot)
		if err != nil {
			return err
		}
		return tx.Create(&BotVersion{BotID: bot.ID, Version: bot.Version, Snapshot: string(snapshot)}).Error
	})
	return &bot, err
}
