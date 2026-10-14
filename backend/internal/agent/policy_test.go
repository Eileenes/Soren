package agent

import (
	"encoding/json"
	"testing"

	"github.com/soren/backend/internal/botconfig"
)

func TestRestrictedCreationAndDelegationPolicy(t *testing.T) {
	s := fixture(t, &recorder{})
	owner := &Bot{Name: "Owner", Avatar: "robo", Instructions: "Role", Definition: botconfig.Definition{CannotDo: []string{"不能发邮件"}}, Policy: &botconfig.Policy{Actions: map[string]string{"gmail.send": "deny", "workspace_create": "confirm"}}}
	if e := s.SaveBot(owner, true); e != nil {
		t.Fatal(e)
	}
	loaded, _ := s.GetBot(owner.ID)
	if loaded.CollaborationEnabled || loaded.CollaborationAll || loaded.Policy.Actions["gmail.send"] != "deny" {
		t.Fatal("restricted creation lost settings")
	}
	worker := addBot(t, s, "Worker")
	run := Run{ID: "policy-run", RequestID: "policy-request", OwnerBotID: owner.ID}
	if e := s.DB.Create(&run).Error; e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.actionPolicy(worker.ID, run.ID, "gmail.send"); e == nil {
		t.Fatal("delegation bypassed owner denial")
	}
	confirm, revision, e := s.actionPolicy(worker.ID, run.ID, "workspace_create")
	if e != nil || !confirm || revision == "" {
		t.Fatal("owner confirmation not inherited")
	}
	if _, _, e = s.actionPolicy(owner.ID, "", "new_tool"); e == nil {
		t.Fatal("new tool automatically granted")
	}
	owner.Definition.CannotDo = nil
	if e = s.SaveBot(owner, false); e != nil {
		t.Fatal(e)
	}
	loaded, _ = s.GetBot(owner.ID)
	if len(loaded.Definition.CannotDo) != 0 || loaded.Policy.Actions["gmail.send"] != "deny" {
		t.Fatal("deleting prose changed permissions")
	}
	var versions []BotVersion
	s.DB.Where("bot_id = ?", owner.ID).Find(&versions)
	if len(versions) != 2 {
		t.Fatal("missing rule history")
	}
	var snapshot Bot
	if e = json.Unmarshal([]byte(versions[0].Snapshot), &snapshot); e != nil || len(snapshot.Definition.CannotDo) != 1 {
		t.Fatal("history changed")
	}
}
