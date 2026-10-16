package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestImageConnectorFlow(t *testing.T) {
	var serverURL string
	s, srv := kindFixture(t, "image", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /text_to_image":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["prompt"] != "猫" || body["image_count"] != float64(2) || body["ratio"] != "1:1" {
				t.Errorf("body %v", body)
			}
			fmt.Fprint(w, `{"code":"ok","message":"","data":{"task_id":"task_01abc","status":"processing"}}`)
		case "GET /tasks/task_01abc":
			fmt.Fprintf(w, `{"code":"ok","message":"","data":{"status":"completed","image_urls":["%s/public/out-1.png","%s/public/out-2.png"]}}`, serverURL, serverURL)
		case "GET /tasks/task_02":
			fmt.Fprint(w, `{"code":"ok","message":"","data":{"status":"processing","image_urls":[]}}`)
		case "GET /public/out-1.png", "GET /public/out-2.png":
			if r.Header.Get("Authorization") != "" {
				t.Error("API key sent to image host")
			}
			w.Write([]byte("PNG" + r.URL.Path))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	serverURL = srv.URL
	assertApprovalRequired(t, s, map[string]bool{"text_to_image": true, "edit_image": true, "retry_task": true, "get_task": false, "pricing": false})
	k := kinds["image"]
	ctx := context.Background()
	out, e := s.executeKind(ctx, k, "text_to_image", "key-1", map[string]string{"prompt": "猫", "image_count": "2", "ratio": "1:1"})
	if e != nil || !strings.Contains(out, "task_01abc") {
		t.Fatal(out, e)
	}
	out, e = call(t, s, "get_task", `{"task_id":"task_01abc"}`)
	if e != nil {
		t.Fatal(e)
	}
	var res struct {
		Status string           `json:"status"`
		Files  []map[string]any `json:"files"`
	}
	if e = json.Unmarshal([]byte(out), &res); e != nil || res.Status != "completed" || len(res.Files) != 2 {
		t.Fatal(out)
	}
	if _, ok := res.Files[1]["path"]; ok {
		t.Fatal("server path exposed")
	}
	var attachment FileArtifact
	if e = s.DB.First(&attachment, "id = ?", res.Files[1]["fileId"]).Error; e != nil {
		t.Fatal(e)
	}
	_, file, e := s.OpenFile(attachment.RunID, attachment.ID)
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(file)
	file.Close()
	if e != nil || string(data) != "PNG/public/out-2.png" {
		t.Fatal("download snapshot mismatch", e)
	}
	if out, e = s.executeKind(ctx, k, "get_task", "key-1", map[string]string{"task_id": "task_02"}); e != nil || strings.Contains(out, "files") {
		t.Fatal(out, e)
	}
	for _, bad := range []map[string]string{{"prompt": "x", "image_count": "9"}, {"prompt": "x", "resolution": "8K"}, {"prompt": ""}} {
		if _, e = s.executeKind(ctx, k, "text_to_image", "key-1", bad); e == nil {
			t.Fatal("accepted", bad)
		}
	}
	if _, e = s.executeKind(ctx, k, "edit_image", "key-1", map[string]string{"prompt": "x", "image_urls": "http://10.0.0.1/a.png"}); e == nil {
		t.Fatal("non-https reference accepted")
	}
	if _, e = s.executeKind(ctx, k, "get_task", "key-1", map[string]string{"task_id": "../me"}); e == nil {
		t.Fatal("traversal accepted")
	}
}
