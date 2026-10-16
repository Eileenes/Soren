package connectors

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentSnapshotsArePrivateImmutableAndConfined(t *testing.T) {
	s := fixture(t)
	dir := filepath.Join(s.DataDir, "connectors", "music")
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "song.mp3")
	os.WriteFile(path, []byte("first"), 0600)
	raw := jsonText(map[string]any{"path": path, "bytes": 5, "sha256": "test"})
	result, e := s.captureFiles("a", "r", raw)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(result, path) {
		t.Fatal("server path exposed")
	}
	var metadata map[string]any
	json.Unmarshal([]byte(result), &metadata)
	id := metadata["fileId"].(string)
	os.WriteFile(path, []byte("later"), 0600)
	_, f, e := s.OpenFile("r", id)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "first" {
		t.Fatal("download changed with source")
	}
	if _, _, e = s.OpenFile("other-run", id); e == nil {
		t.Fatal("cross-run download")
	}
	outside := filepath.Join(s.DataDir, "vault-data")
	os.WriteFile(outside, []byte("private"), 0600)
	link := filepath.Join(dir, "escape")
	os.Symlink(outside, link)
	if _, e = s.captureFiles("a", "r", jsonText(map[string]any{"path": link, "bytes": 7, "sha256": "test"})); e == nil {
		t.Fatal("symlink escaped connector root")
	}
	if got, e := s.captureFiles("a", "r", `{"path":"remote/api/path"}`); e != nil || got != `{"path":"remote/api/path"}` {
		t.Fatal("ordinary remote path treated as local file")
	}
}
