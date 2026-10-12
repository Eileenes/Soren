package connectors

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDesignConnectorFlow(t *testing.T) {
	s, _ := kindFixture(t, "design", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/visual-styles":
			if r.URL.Query().Get("context") != "prototype" || r.URL.Query().Get("q") != "minimal" {
				t.Error(r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":"STYLE_ID"}],"total":1}}`)
		case "/design-systems/linear.app":
			fmt.Fprint(w, `{"code":0,"data":{"id":"linear.app","content":"---"}}`)
		case "/catalog/templates/tpl1/download":
			http.Redirect(w, r, "/public/zip", http.StatusFound)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	assertApprovalRequired(t, s, map[string]bool{"search_templates": false, "get_template": false, "download_template": false, "search_visual_styles": false, "get_visual_style": false, "search_directions": false, "search_design_systems": false, "get_design_system": false})
	k, ctx := kinds["design"], context.Background()
	if out, e := call(t, s, "search_visual_styles", `{"context":"prototype","q":"minimal"}`); e != nil || !strings.Contains(out, "STYLE_ID") {
		t.Fatal(out, e)
	}
	if _, e := s.executeKind(ctx, k, "search_visual_styles", "key-1", map[string]string{"context": "poster"}); e == nil {
		t.Fatal("bad context accepted")
	}
	if out, e := s.executeKind(ctx, k, "get_design_system", "key-1", map[string]string{"id": "linear.app"}); e != nil || !strings.Contains(out, "linear.app") {
		t.Fatal(out, e)
	}
	// Redirects to plain HTTP are refused; only HTTPS signed storage URLs are followed.
	if _, e := s.executeKind(ctx, k, "download_template", "key-1", map[string]string{"id": "tpl1"}); e == nil {
		t.Fatal("non-https redirect followed")
	}
}

func TestDownloadFollowsHTTPSRedirectWithoutKey(t *testing.T) {
	s := fixture(t)
	storage := newTLSStorage(t, "PK-zip")
	s.HTTP = storage.Client()
	api := newTLSRedirect(t, storage.URL+"/obj.zip")
	s.APIBase = map[string]string{"design": api.URL}
	out, e := s.executeKind(context.Background(), kinds["design"], "download_template", "key-1", map[string]string{"id": "tpl1"})
	if e != nil {
		t.Fatal(e)
	}
	if p := mustFile(t, out, "PK-zip"); !strings.HasSuffix(p, "template-tpl1.zip") {
		t.Fatal(p)
	}
}
