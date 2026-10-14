package templateserver

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/soren/backend/internal/bottemplate"
)

// Keep these bundled icons in sync with frontend/public/bot-avatars and soren-mark.svg.
// Embedding lets both standalone and Docker deployments serve the same Bot identity.
//
//go:embed web
var previewAssets embed.FS

var previewPage = template.Must(template.ParseFS(previewAssets, "web/preview.html"))

var avatarFiles = map[string]bool{
	"soren.svg": true, "angler.png": true, "flamingo.png": true,
	"hydrant.png": true, "iceberg.png": true, "pudin.png": true,
	"rex.png": true, "robo.png": true, "saboten.png": true,
	"tako.png": true, "yurei.png": true,
}

func serveAvatar(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !avatarFiles[name] {
		http.NotFound(w, r)
		return
	}
	raw, err := previewAssets.ReadFile("web/avatars/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType := "image/png"
	if strings.HasSuffix(name, ".svg") {
		contentType = "image/svg+xml"
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(raw)
}

func serveImportScript(w http.ResponseWriter, r *http.Request) {
	raw, err := previewAssets.ReadFile("web/import.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(raw)
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	v, err := s.load(r)
	if err != nil {
		http.Error(w, "模板不存在、已停用或已删除。请联系分享者。", 404)
		return
	}
	link := "soren://templates/import?" + url.Values{"id": {v.ID}, "revision": {strconv.Itoa(v.Revision)}, "service": {s.origin}}.Encode()
	avatar := v.Config.Avatar + ".png"
	if !avatarFiles[avatar] {
		avatar = "soren.svg"
	}
	data := struct {
		bottemplate.Published
		DeepLink template.URL
		Avatar   string
		Service  string
	}{v, template.URL(link), "/assets/avatars/" + avatar, s.origin}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Only our static launch script runs; template content never enters script context.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; script-src 'self'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	_ = previewPage.Execute(w, data)
}
