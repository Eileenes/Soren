package templatesclient

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/soren/backend/internal/agent"
	"github.com/soren/backend/internal/botconfig"
	"github.com/soren/backend/internal/bottemplate"
	"github.com/soren/backend/internal/connectors"
	"github.com/soren/backend/internal/templateserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { conn.Close() })
	return db
}
func testClient(t *testing.T) *Client {
	t.Helper()
	a, e := agent.New(testDB(t), nil, nil, "", true)
	if e != nil {
		t.Fatal(e)
	}
	c, e := New(a, &connectors.MemoryVault{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func configFixture() bottemplate.Config {
	return bottemplate.Config{Format: bottemplate.Format, SchemaVersion: 1, Name: "短视频助手", Avatar: "robo", Description: "选题与分镜", Instructions: "先写脚本，再询问是否生成视频。", Definition: botconfig.Definition{CanDo: []string{"策划选题"}, CannotDo: []string{"不发送邮件"}}, PolicyTemplate: &botconfig.Policy{Actions: map[string]string{"video.generate": "confirm", "workspace_create": "allow", "gmail.send": "deny"}}, MemoryEnabled: true, RequiredCapabilities: []string{"video.generate", "workspace_create"}}
}

func TestPublishImportAcrossIndependentInstallations(t *testing.T) {
	ctx := context.Background()
	token := strings.Repeat("a", 43)
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	service, e := templateserver.New(testDB(t), server.URL, []templateserver.Publisher{{ID: "alice", Name: "Alice", TokenHash: fmt.Sprintf("%x", sha256.Sum256([]byte(token)))}})
	if e != nil {
		t.Fatal(e)
	}
	server.Config.Handler = service.Handler()
	author, recipient := testClient(t), testClient(t)
	if _, e = author.Save(SettingsInput{URL: server.URL, Token: token}); e != nil {
		t.Fatal(e)
	}
	if _, e = recipient.Save(SettingsInput{URL: server.URL}); e != nil {
		t.Fatal(e)
	}
	settings, _ := author.Get()
	raw, _ := json.Marshal(settings)
	if strings.Contains(string(raw), token) {
		t.Fatal("credential leaked")
	}
	if identity, e := author.Identity(ctx); e != nil || identity["name"] != "Alice" {
		t.Fatalf("identity: %v %v", identity, e)
	}
	input := bottemplate.PublishRequest{RequestID: uuid.NewString(), Config: configFixture()}
	first, e := author.Publish(ctx, "", input)
	if e != nil {
		t.Fatal(e)
	}
	retry, e := author.Publish(ctx, "", input)
	if e != nil || retry.ID != first.ID || retry.Revision != 1 {
		t.Fatal("publish retry created duplicate", e)
	}
	input.Config.Instructions = "变更后的脚本规则"
	if _, e = author.Publish(ctx, "", input); e == nil {
		t.Fatal("idempotency key accepted different config")
	}
	input.RequestID = uuid.NewString()
	second, e := author.Publish(ctx, first.ID, input)
	if e != nil || second.Revision != 2 {
		t.Fatal("revision", e)
	}
	preview, e := recipient.Preview(ctx, first.URL)
	if e != nil {
		t.Fatal(e)
	}
	if preview.Published.Config.Instructions == second.Config.Instructions {
		t.Fatal("old link silently changed")
	}
	var count int64
	recipient.DB.Model(&agent.Bot{}).Count(&count)
	if count != 1 {
		t.Fatal("preview created bot")
	}
	bot, e := recipient.Import(ctx, preview.ID, "我的视频助手")
	if e != nil {
		t.Fatal(e)
	}
	if bot.ID == agent.DefaultBotID || bot.Model != "" || bot.CollaborationEnabled || bot.CollaborationAll || bot.TemplateSource == nil || bot.Policy == nil {
		t.Fatalf("unsafe imported defaults: %+v", bot)
	}
	for _, permission := range bot.Policy.Actions {
		if permission != "deny" {
			t.Fatal("template automatically granted tool access")
		}
	}
	again, e := recipient.Import(ctx, preview.ID, bot.Name)
	if e != nil || again.ID != bot.ID {
		t.Fatal("confirm retry duplicated bot", e)
	}
	if again.Definition.CannotDo[0] != "不发送邮件" || again.Policy.Actions["gmail.send"] != "deny" {
		t.Fatal("definition/policy not persisted")
	}
	duplicate, e := recipient.Preview(ctx, first.URL)
	if e != nil || duplicate.ExistingBotID != bot.ID {
		t.Fatal("duplicate notice absent", e)
	}
	disabled := true
	if e = author.Manage(ctx, first.ID, &disabled); e != nil {
		t.Fatal(e)
	}
	if _, e = recipient.Preview(ctx, first.URL); e == nil {
		t.Fatal("disabled template downloadable")
	}
	if _, e = recipient.Import(ctx, duplicate.ID, "another"); e == nil {
		t.Fatal("disabled template imported after preview")
	}
	if _, e = recipient.Agents.GetBot(bot.ID); e != nil {
		t.Fatal("disable revoked imported copy")
	}
	disabled = false
	_ = author.Manage(ctx, first.ID, &disabled)
	if e = author.Manage(ctx, first.ID, nil); e != nil {
		t.Fatal(e)
	}
	if _, e = author.Publish(ctx, first.ID, input); e == nil {
		t.Fatal("retry resurrected deleted revision")
	}
	if rows, e := author.List(ctx); e != nil || len(rows) != 0 {
		t.Fatal("deleted template listed", e)
	}
}

func TestNoSecretsInPortableExport(t *testing.T) {
	bot := &agent.Bot{ID: "private-bot", Name: "Author", Avatar: "robo", Instructions: "Role", Model: "private-model-service", Collaborators: []string{"private-collaborator"}, CollaborationEnabled: true, TemplateSource: &agent.TemplateSource{Service: "https://private.example", ID: "old"}}
	raw, _ := json.Marshal(Export(bot))
	for _, private := range []string{"private-bot", "private-model-service", "private-collaborator", "private.example", "collaborators", "templateSource"} {
		if strings.Contains(string(raw), private) {
			t.Fatal("private metadata leaked", private)
		}
	}
}

func TestLinkValidationAndCredentialIsolation(t *testing.T) {
	id := uuid.NewString()
	base := "https://templates.example"
	for _, link := range []string{"https://evil.example/t/" + id + "/v/1", base + "/t/" + id + "/v/1?url=http://127.0.0.1", "soren://templates/import?id=" + id + "&revision=1&service=https://evil.example", "soren://templates/import?id=" + id + "&id=another&revision=1&service=https://templates.example", base + "/t/../../secret/v/1"} {
		if _, _, e := parseLink(base, link); e == nil {
			t.Fatal("accepted unsafe link", link)
		}
	}
	if _, n, e := parseLink(base, "soren://templates/import?id="+id+"&revision=2&service=https%3A%2F%2Ftemplates.example"); e != nil || n != 2 {
		t.Fatal("valid native link rejected", e)
	}
	c := testClient(t)
	_, _ = c.Save(SettingsInput{URL: base, Token: strings.Repeat("a", 43)})
	old, _ := c.settings()
	_, e := c.Save(SettingsInput{URL: "https://other.example"})
	if e != nil {
		t.Fatal(e)
	}
	current, _ := c.Get()
	if current.Configured {
		t.Fatal("credential reused at different server")
	}
	if _, e = c.Vault.Get(old.CredentialID); e == nil {
		t.Fatal("old credential retained")
	}
}

func TestUntrustedResponsesCannotRedirectOrForgeContent(t *testing.T) {
	ctx := context.Background()
	c := testClient(t)
	id := uuid.NewString()
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("followed redirect") }))
	defer forbidden.Close()
	mode := "redirect"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("download carried publishing credential")
		}
		if mode == "redirect" {
			http.Redirect(w, r, forbidden.URL, 302)
			return
		}
		if mode == "oversize" {
			_, _ = io.WriteString(w, strings.Repeat("x", bottemplate.MaxBytes+1))
			return
		}
		_ = json.NewEncoder(w).Encode(bottemplate.Published{ID: id, Revision: 1, Digest: "forged", Config: configFixture()})
	}))
	defer server.Close()
	_, _ = c.Save(SettingsInput{URL: server.URL, Token: strings.Repeat("b", 43)})
	for _, value := range []string{"redirect", "oversize", "digest"} {
		mode = value
		if _, e := c.Preview(ctx, server.URL+"/t/"+id+"/v/1"); e == nil {
			t.Fatal("accepted " + value)
		}
	}
}
