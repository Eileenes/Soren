package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const cid, mid, gid = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", "33333333-3333-3333-3333-333333333333"

func TestMusicConnectorFlow(t *testing.T) {
	generated := 0
	s, _ := kindFixture(t, "music", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /creations":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["message"] != "温暖的钢琴" || body["guide"].(map[string]any)["genre"] != "jazz" {
				t.Errorf("body %v", body)
			}
			fmt.Fprintf(w, `{"creation_id":%q,"message_id":%q,"status":"succeeded","brief":{"title":"T"}}`, cid, mid)
		case "POST /creations/" + cid + "/generations":
			generated++
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), mid) {
				t.Errorf("generation body %s", b)
			}
			fmt.Fprintf(w, `{"id":%q,"status":"queued"}`, gid)
		case "GET /generations/" + gid + "/audio":
			if r.URL.Query().Get("format") != "mp3" {
				t.Error("format not mp3")
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Header().Set("X-Audio-Duration", "61.5")
			w.Write([]byte("ID3fake"))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	assertApprovalRequired(t, s, map[string]bool{"start_creation": false, "continue_creation": false, "get_creation": false, "get_generation": false, "list_generations": false, "get_audio": false, "generate_music": true, "retry_plan": true, "retry_generation": true, "delete_generation": true})

	out, e := call(t, s, "start_creation", `{"message":"温暖的钢琴","genre":"jazz"}`)
	if e != nil || !strings.Contains(out, mid) {
		t.Fatal(out, e)
	}
	// Paid generation must wait for approval and not reach upstream.
	p := &scripted{name: "c_k1_generate_music", args: fmt.Sprintf(`{"creation_id":%q,"message_id":%q}`, cid, mid)}
	if _, e = loop(s, p); !errors.Is(e, ErrApproval) || generated != 0 {
		t.Fatal(e, generated)
	}
	inv := pending(t, s)
	s.DB.Exec("UPDATE runs SET status='waiting_approval'")
	if e = s.Decide(inv.ID, "a", true); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec("UPDATE runs SET status='running'")
	if _, e = loop(s, p); e != nil || generated != 1 {
		t.Fatal(e, generated)
	}

	res, e := s.executeKind(context.Background(), kinds["music"], "get_audio", "key-1", map[string]string{"generation_id": gid})
	if e != nil {
		t.Fatal(e)
	}
	path := mustFile(t, res, "ID3fake")
	if !strings.HasSuffix(path, gid+".mp3") || !strings.Contains(res, `"durationSeconds":61.5`) {
		t.Fatal(res)
	}
	if _, e = s.executeKind(context.Background(), kinds["music"], "get_generation", "key-1", map[string]string{"generation_id": "../../x"}); e == nil {
		t.Fatal("path traversal accepted")
	}
	if _, e = s.executeKind(context.Background(), kinds["music"], "start_creation", "key-1", map[string]string{"message": "x", "genre": "metal"}); e == nil {
		t.Fatal("bad genre accepted")
	}
}

func TestKindKeyValidation(t *testing.T) {
	s, _ := kindFixture(t, "music", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"items":[]}`) })
	if e := s.CheckKey(context.Background(), "music", "key-1"); e != nil {
		t.Fatal(e)
	}
	if e := s.CheckKey(context.Background(), "music", "wrong"); e == nil || !strings.Contains(e.Error(), "无效") {
		t.Fatal(e)
	}
	if e := s.CheckKey(context.Background(), "music", ""); e == nil {
		t.Fatal("empty key accepted")
	}
}
