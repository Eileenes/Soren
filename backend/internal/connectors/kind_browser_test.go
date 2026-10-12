package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestBrowserConnectorFlow(t *testing.T) {
	created, stopped := 0, 0
	s, _ := kindFixture(t, "browser", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /browser/sessions":
			created++
			if len(r.Header.Get("Idempotency-Key")) < 8 {
				t.Error("missing Idempotency-Key")
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["max_seconds"] != float64(600) {
				t.Errorf("body %v", body)
			}
			w.WriteHeader(202)
			fmt.Fprint(w, `{"code":"ok","data":{"session_id":"ses_1","status":"starting"}}`)
		case "GET /browser/sessions":
			if r.URL.Query().Get("page_size") != "5" {
				t.Error("page_size not forwarded")
			}
			fmt.Fprint(w, `{"code":"ok","data":{"items":[]}}`)
		case "DELETE /browser/sessions/ses_1":
			stopped++
			w.WriteHeader(202)
			fmt.Fprint(w, `{"code":"ok","data":{"status":"stopping"}}`)
		case "GET /browser/sessions/ses_1/downloads/dl_1/content":
			w.Header().Set("Content-Disposition", `attachment; filename="../report.csv"`)
			w.Header().Set("X-Content-SHA256", "8e0c5aa5ee4bc0b6d8f5a3d6d1b2b5e8a8e46b0b9f2c0f1d07d0a2c6a1e7f4b3")
			w.Write([]byte("a,b\n"))
		case "GET /browser/sessions/ses_1/downloads/dl_ok/content":
			w.Header().Set("Content-Disposition", `attachment; filename="../report.csv"`)
			w.Write([]byte("a,b\n"))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	assertApprovalRequired(t, s, map[string]bool{"create_session": true, "stop_session": true, "delete_profile": true, "get_session": false, "list_sessions": false, "begin_handoff": false, "list_downloads": false, "download_file": false, "list_profiles": false, "get_profile": false, "create_profile": false, "pricing": false, "usage": false})
	k, ctx := kinds["browser"], context.Background()
	if out, e := s.executeKind(ctx, k, "create_session", "key-1", map[string]string{"max_seconds": "600"}); e != nil || !strings.Contains(out, "ses_1") || created != 1 {
		t.Fatal(out, e)
	}
	if _, e := s.executeKind(ctx, k, "create_session", "key-1", map[string]string{"max_seconds": "9000"}); e == nil {
		t.Fatal("max_seconds over limit accepted")
	}
	if _, e := call(t, s, "list_sessions", `{"page_size":"5"}`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.executeKind(ctx, k, "stop_session", "key-1", map[string]string{"session_id": "ses_1"}); e != nil || stopped != 1 {
		t.Fatal(e)
	}
	if _, e := s.executeKind(ctx, k, "download_file", "key-1", map[string]string{"session_id": "ses_1", "download_id": "dl_1"}); e == nil {
		t.Fatal("sha256 mismatch accepted")
	}
	out, e := s.executeKind(ctx, k, "download_file", "key-1", map[string]string{"session_id": "ses_1", "download_id": "dl_ok"})
	if e != nil {
		t.Fatal(e)
	}
	if p := mustFile(t, out, "a,b\n"); !strings.HasSuffix(p, "/connectors/browser/report.csv") {
		t.Fatal(p)
	}
}
