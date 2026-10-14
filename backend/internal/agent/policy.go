package agent

import (
	"fmt"
	"github.com/soren/backend/internal/botconfig"
)

func (s *Service) actionPolicy(bot, run, action string) (bool, string, error) {
	b, e := s.GetBot(bot)
	if e != nil {
		return false, "", e
	}
	confirm, e := botconfig.Decision(b.Policy, action)
	if e != nil {
		return false, "", e
	}
	revision := fmt.Sprintf("%s:%d", b.ID, b.Version)
	if run != "" {
		var r Run
		if e = s.DB.First(&r, "id = ?", run).Error; e != nil {
			return false, "", e
		}
		if r.OwnerBotID != bot {
			owner, e := s.GetBot(r.OwnerBotID)
			if e != nil {
				return false, "", e
			}
			other, e := botconfig.Decision(owner.Policy, action)
			if e != nil {
				return false, "", e
			}
			confirm = confirm || other
			revision += fmt.Sprintf(":%s:%d", owner.ID, owner.Version)
		}
	}
	return confirm, revision, nil
}
