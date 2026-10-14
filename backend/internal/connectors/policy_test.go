package connectors

import (
	"context"
	"errors"
	"testing"

	"github.com/soren/backend/internal/llm"
)

func TestPolicyRecheckedBeforeExecutionAndApprovalCannotSurviveChange(t *testing.T) {
	s := fixture(t)
	revision := "v1"
	denied := false
	s.Policy = func(bot, run, action string) (bool, string, error) {
		if denied {
			return false, "", errors.New("revoked")
		}
		return true, revision, nil
	}
	p := &scripted{name: "workspace_create", args: `{"title":"test","content":"data"}`}
	if _, e := loop(s, p); !errors.Is(e, ErrApproval) {
		t.Fatal("confirmation policy ignored", e)
	}
	inv := pending(t, s)
	if inv.PolicyRevision != "v1" {
		t.Fatal("approval missing policy snapshot")
	}
	s.DB.Model(&Invocation{}).Where("id = ?", inv.ID).Update("status", "approved")
	revision = "v2"
	catalog, e := s.catalog("a")
	if e != nil {
		t.Fatal(e)
	}
	call := llm.ToolCall{Function: llm.Function{Name: p.name, Arguments: p.args}}
	if _, e = s.invoke(context.Background(), catalog, "a", "r", inv.ID, call); e == nil {
		t.Fatal("old approval executed with new policy")
	}
	denied = true
	if _, e = s.invoke(context.Background(), catalog, "a", "r", "new-invocation", call); e == nil {
		t.Fatal("stale catalog bypassed revoked policy")
	}
	var count int64
	s.DB.Model(&Artifact{}).Count(&count)
	if count != 0 {
		t.Fatal("denied tool created artifact")
	}
}
