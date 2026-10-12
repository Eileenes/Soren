package mobile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/soren/backend/internal/relay"
	"golang.org/x/crypto/nacl/box"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func fixture(t *testing.T) (*Service, *[32]byte, *[32]byte, *atomic.Int32) {
	t.Helper()
	dir := t.TempDir()
	db, e := gorm.Open(sqlite.Open(filepath.Join(dir, "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { sql.Close() })
	calls := &atomic.Int32{}
	s, e := New(db, dir, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}), "", "", true)
	if e != nil {
		t.Fatal(e)
	}
	s.state.Enabled = true
	pub, priv, _ := box.GenerateKey(rand.Reader)
	return s, pub, priv, calls
}
func seal(t *testing.T, s *Service, pub, priv *[32]byte, r Request) json.RawMessage {
	t.Helper()
	var n [24]byte
	rand.Read(n[:])
	plain, _ := json.Marshal(r)
	raw, _ := json.Marshal(Envelope{Key: b64(pub[:]), Nonce: b64(n[:]), Box: b64(box.Seal(nil, plain, &n, &s.state.Public, priv))})
	return raw
}
func openResponse(t *testing.T, s *Service, priv *[32]byte, raw []byte) Response {
	t.Helper()
	var e Envelope
	if json.Unmarshal(raw, &e) != nil {
		t.Fatal("bad envelope")
	}
	nonce, err := decode(e.Nonce, 24)
	if err != nil {
		t.Fatalf("invalid response: %s", raw)
	}
	cipher, _ := decode(e.Box, 0)
	var n [24]byte
	copy(n[:], nonce)
	plain, ok := box.Open(nil, cipher, &n, &s.state.Public, priv)
	if !ok {
		t.Fatal("response not authentic")
	}
	var out Response
	json.Unmarshal(plain, &out)
	return out
}
func req() Request {
	return Request{ID: relay.RandomID(), Deadline: time.Now().Unix() + 120, Kind: "api", Method: "POST", Path: "/api/runs", BotID: "soren", Body: json.RawMessage(`{"goal":"hello"}`)}
}
func TestTrustReplayAndPersistence(t *testing.T) {
	s, pub, priv, calls := fixture(t)
	request := req()
	send := func(r Request) Response { return openResponse(t, s, priv, s.Handle(seal(t, s, pub, priv, r))) }
	if r := send(request); r.Status != 403 {
		t.Fatal(r)
	}
	s.state.Devices[b64(pub[:])] = Device{Key: b64(pub[:]), Name: "phone"}
	if e := s.save(); e != nil {
		t.Fatal(e)
	}
	if r := send(request); r.Status != 200 {
		t.Fatal(r)
	}
	if r := send(request); r.Status != 200 {
		t.Fatal(r)
	}
	if calls.Load() != 1 {
		t.Fatal("duplicate execution")
	}
	restored, e := New(s.db, filepath.Dir(s.path), s.handler, "", "", true)
	if e != nil {
		t.Fatal(e)
	}
	if r := openResponse(t, restored, priv, restored.Handle(seal(t, restored, pub, priv, request))); r.Status != 200 {
		t.Fatal(r)
	}
	if calls.Load() != 1 {
		t.Fatal("replayed after restart")
	}
	altered := request
	altered.Body = json.RawMessage(`{"goal":"changed"}`)
	if r := send(altered); r.Status != 409 {
		t.Fatal("ID reuse accepted", r)
	}
	delete(s.state.Devices, b64(pub[:]))
	if r := send(request); r.Status != 403 {
		t.Fatal("revoked device read replay result", r)
	}
}
func TestPairingRequiresMacApprovalAndExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s, pub, priv, _ := fixture(t)
	s.pair = &pairing{Secret: relay.RandomID(), Expires: time.Now().Unix() + 300}
	request := req()
	request.Kind = "pair"
	request.Secret = s.pair.Secret
	request.Name = "iPhone"
	send := func(r Request) Response { return openResponse(t, s, priv, s.Handle(seal(t, s, pub, priv, r))) }
	wrong := request
	wrong.Secret = "wrong"
	if r := send(wrong); r.Status != 403 {
		t.Fatal(r)
	}
	if r := send(request); r.Status != 202 {
		t.Fatal(r)
	}
	if len(s.state.Devices) != 0 {
		t.Fatal("trusted without approval")
	}
	other, _, _ := box.GenerateKey(rand.Reader)
	if r := s.dispatch(b64(other[:]), other[:], request); r.Status != 409 {
		t.Fatal("pending replaced", r)
	}
	router := gin.New()
	s.Register(router)
	approve := func(key, code string) int {
		body, _ := json.Marshal(map[string]any{"key": key, "code": code, "approve": true})
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/mobile/decision", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w.Code
	}
	if approve(b64(pub[:]), "wrong") != 409 {
		t.Fatal("confirmation mismatch accepted")
	}
	if approve(b64(pub[:]), s.pair.Code) != 204 {
		t.Fatal("approval failed")
	}
	if r := send(request); r.Status != 200 {
		t.Fatal(r)
	}
	s.pair = &pairing{Secret: request.Secret, Expires: time.Now().Unix() - 1}
	delete(s.state.Devices, b64(pub[:]))
	if r := send(request); r.Status != 403 {
		t.Fatal("expired accepted", r)
	}
}
func TestAllowlistDeadlinesAndTampering(t *testing.T) {
	s, pub, priv, calls := fixture(t)
	s.state.Devices[b64(pub[:])] = Device{}
	for _, path := range []string{"/api/mobile", "/api/identity", "/api/connections", "/api/runs/../../mobile", "/api/runs?admin=1", "http://evil.test/api/runs"} {
		r := req()
		r.Path = path
		if res := openResponse(t, s, priv, s.Handle(seal(t, s, pub, priv, r))); res.Status != 403 {
			t.Fatal(path, res)
		}
	}
	for _, deadline := range []int64{time.Now().Unix() - 1, time.Now().Unix() + 1000} {
		r := req()
		r.Deadline = deadline
		if res := openResponse(t, s, priv, s.Handle(seal(t, s, pub, priv, r))); res.Status != 400 {
			t.Fatal(res)
		}
	}
	raw := seal(t, s, pub, priv, req())
	var env Envelope
	json.Unmarshal(raw, &env)
	cipher, _ := decode(env.Box, 0)
	cipher[0] ^= 1
	env.Box = b64(cipher)
	raw, _ = json.Marshal(env)
	if string(s.Handle(raw)) != "{}" {
		t.Fatal("tampered accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("forbidden request executed")
	}
}
func TestInterruptedWriteIsNotReexecuted(t *testing.T) {
	s, pub, priv, calls := fixture(t)
	s.state.Devices[b64(pub[:])] = Device{}
	r := req() // A durable reservation left by an interrupted process must stop a replay.
	raw, _ := json.Marshal(r)
	sum := sha256Digest(raw)
	s.db.Create(&receipt{ID: b64(pub[:]) + ":" + r.ID, Digest: sum, Deadline: r.Deadline})
	res := openResponse(t, s, priv, s.Handle(seal(t, s, pub, priv, r)))
	if res.Status != 409 || calls.Load() != 0 {
		t.Fatal(res)
	}
}
func TestRealRelayRoundTripAndOffline(t *testing.T) {
	s, pub, priv, calls := fixture(t)
	rs := relay.New(nil)
	server := httptest.NewServer(rs)
	defer server.Close()
	s.relayURL = server.URL
	s.state.Devices[b64(pub[:])] = Device{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	raw := seal(t, s, pub, priv, req())
	url := server.URL + "/v1/hosts/" + relay.HostID(s.state.HostToken) + "/exchange"
	var result []byte
	for i := 0; i < 50; i++ {
		res, e := http.Post(url, "application/json", bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == 200 {
			result = body
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(result) == 0 {
		t.Fatal("relay did not connect")
	}
	if r := openResponse(t, s, priv, result); r.Status != 200 {
		t.Fatal(r)
	}
	if calls.Load() != 1 {
		t.Fatal("missing execution")
	}
	// A different host cannot authenticate a poll using this host's secret.
	r := httptest.NewRequest("GET", "/v1/hosts/"+relay.RandomID()+"/poll", nil)
	r.Header.Set("Authorization", "Bearer "+s.state.HostToken)
	w := httptest.NewRecorder()
	rs.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("host isolation failed")
	}
	cancel()
}

func sha256Digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
