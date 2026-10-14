package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const videoCaps = `{"code":"ok","message":"ok","data":{"supports_image_to_video":true,"duration_seconds":{"min":2,"max":60},"resolution":"1080P","points_per_second":23,"image_to_video":{"default_resolution":"720P","points_per_second":27}}}`

func TestVideoConnectionAndPaidSubmissionReplay(t *testing.T) {
	checks, creates := 0, 0
	s, _ := kindFixture(t, "video", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /video-capabilities":
			checks++
			fmt.Fprint(w, videoCaps)
		case "POST /videos":
			creates++
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil || len(input) != 1 || input["prompt"] != "雪地上的猫" {
				t.Errorf("unexpected submission: %v", input)
			}
			if checks == 0 {
				t.Error("submitted before checking capabilities")
			}
			fmt.Fprint(w, `{"code":"ok","message":"ok","data":{"job_id":"video-1","status":"queued","points":115}}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	assertApprovalRequired(t, s, map[string]bool{"get_video_capabilities": false, "create_video": true, "get_video": false})
	if err := s.CheckKey(context.Background(), "video", "key-1"); err != nil {
		t.Fatal(err)
	}
	if creates != 0 {
		t.Fatal("credential validation created a paid task")
	}
	caps, err := call(t, s, "get_video_capabilities", `{}`)
	if err != nil || !strings.Contains(caps, `"points_per_second":23`) || strings.Contains(caps, `"code"`) {
		t.Fatal(caps, err)
	}

	p := &scripted{name: "c_k1_create_video", args: `{"prompt":"雪地上的猫"}`}
	if _, err := loop(s, p); !errors.Is(err, ErrApproval) {
		t.Fatal(err)
	}
	if creates != 0 {
		t.Fatal("video was created before approval")
	}
	var inv Invocation
	if err := s.DB.First(&inv, "run_id = ? AND status = ?", "r", "pending").Error; err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE runs SET status='waiting_approval' WHERE id='r'")
	if err := s.Decide(inv.ID, "a", true); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE runs SET status='running' WHERE id='r'")
	if _, err := loop(s, p); err != nil {
		t.Fatal(err)
	}
	if _, err := loop(s, p); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatalf("paid task submitted %d times", creates)
	}
	if err := s.DB.First(&inv, "id = ?", inv.ID).Error; err != nil || !strings.Contains(inv.Result, `"status":"queued"`) || strings.Contains(inv.Result, `"saved":true`) {
		t.Fatal(inv.Result, err)
	}
}

func TestVideoCreateInputsAndUnavailableCapabilities(t *testing.T) {
	var submitted []map[string]any
	var unavailable atomic.Bool
	s, _ := kindFixture(t, "video", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			if unavailable.Load() {
				w.WriteHeader(503)
				return
			}
			fmt.Fprint(w, videoCaps)
			return
		}
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		submitted = append(submitted, input)
		fmt.Fprint(w, `{"code":"ok","data":{"id":"job-2","status":"running","points":80}}`)
	})
	k := kinds["video"]
	args := map[string]string{"prompt": "缓慢推进", "image_url": "https://images.example.com/original.png", "duration_seconds": "12.5", "aspect_ratio": "16:9", "resolution": "1080P"}
	out, err := s.executeKind(context.Background(), k, "create_video", "key-1", args)
	if err != nil || len(submitted) != 1 || submitted[0]["duration_seconds"] != 12.5 || submitted[0]["resolution"] != "1080P" {
		t.Fatal(out, submitted, err)
	}
	delete(args, "resolution")
	delete(args, "duration_seconds")
	_, _ = s.executeKind(context.Background(), k, "create_video", "key-1", args)
	if _, ok := submitted[1]["resolution"]; ok {
		t.Fatal("client forced a default resolution")
	}
	if _, ok := submitted[1]["duration_seconds"]; ok {
		t.Fatal("client forced a default duration")
	}
	for _, bad := range []map[string]string{
		{"prompt": ""}, {"prompt": "x", "resolution": "1080P"}, {"prompt": "x", "image_base64": "fake"},
		{"prompt": "x", "image_url": "/tmp/original.png"}, {"prompt": "x", "image_url": "https://127.0.0.1/image.png"},
		{"prompt": "x", "image_url": "https://user:pass@example.com/image.png"}, {"prompt": "x", "duration_seconds": "NaN"},
		{"prompt": "x", "duration_seconds": "-1"},
	} {
		out, err := s.executeKind(context.Background(), k, "create_video", "key-1", bad)
		if err != nil || !strings.Contains(out, `"status":"not_submitted"`) {
			t.Fatal(bad, out, err)
		}
	}
	unavailable.Store(true)
	out, err = s.executeKind(context.Background(), k, "create_video", "key-1", args)
	if err != nil || !strings.Contains(out, "not_submitted") || len(submitted) != 2 {
		t.Fatal(out, err, len(submitted))
	}
}

func TestVideoWaitDownloadsOriginalURLWithoutCredentials(t *testing.T) {
	const original = "original-video-bytes"
	var downloads atomic.Int32
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential leaked to video download")
		}
		if r.URL.RequestURI() != "/signed/video?signature=A%2FB%2BC&expires=123" {
			t.Error("signed URL was rewritten", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "video/webm")
		fmt.Fprint(w, original)
	}))
	defer storage.Close()
	var queries atomic.Int32
	s, _ := kindFixture(t, "video", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/videos/job-done" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if queries.Add(1) == 1 {
			fmt.Fprint(w, `{"code":"ok","data":{"job_id":"job-done","status":"running","points":140}}`)
			return
		}
		fmt.Fprint(w, `{"code":"ok","data":{"job_id":"job-done","status":"succeeded","points":120,"internal_model":"hidden-model","result":{"asset_url":"https://video-files.example.com/signed/video?signature=A%2FB%2BC&expires=123"}}}`)
	})
	// Route only the public test origin to TLS storage without modifying the URL.
	transport := storage.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, storage.Listener.Addr().String())
	}
	s.DownloadHTTP = &http.Client{Transport: transport}
	out, err := s.waitForVideo(context.Background(), kinds["video"], "key-1", "job-done", time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status string
		Points int
		Saved  bool
		File   json.RawMessage
	}
	if json.Unmarshal([]byte(out), &result) != nil || result.Status != "succeeded" || result.Points != 120 || !result.Saved {
		t.Fatal(out)
	}
	mustFile(t, string(result.File), original)
	var file struct{ Path string }
	if json.Unmarshal(result.File, &file) != nil || filepath.Ext(file.Path) != ".webm" {
		t.Fatal("saved video with the wrong extension", string(result.File))
	}
	if downloads.Load() != 1 || queries.Load() != 2 || strings.Contains(out, "signature") || strings.Contains(out, "hidden-model") {
		t.Fatal(out, downloads.Load(), queries.Load())
	}
}

type videoDownloadTransport func(*http.Request) (*http.Response, error)

func (f videoDownloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVideoDownloadFailuresPreserveCompletedJob(t *testing.T) {
	for _, tc := range []struct {
		name, asset string
		status      int
		wantCalls   int
	}{
		{"missing asset", "", 200, 0},
		{"private asset", "https://127.0.0.1/private", 200, 0},
		{"expired asset", "https://files.example.com/video", 403, 1},
		{"redirect", "https://files.example.com/video", 302, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := kindFixture(t, "video", func(http.ResponseWriter, *http.Request) {
				t.Error("saving a completed video must not call the API")
			})
			calls := 0
			s.DownloadHTTP.Transport = videoDownloadTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.String() != tc.asset || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected download request: %s", r.URL)
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {"video/mp4"}, "Location": {"https://127.0.0.1/private"}}, Body: io.NopCloser(strings.NewReader("not a video")), Request: r}, nil
			})
			var data map[string]json.RawMessage
			raw := jsonText(map[string]any{"job_id": "completed", "status": "succeeded", "points": 80, "result": map[string]string{"asset_url": tc.asset}})
			_ = json.Unmarshal([]byte(raw), &data)
			out, err := s.videoResult(context.Background(), kinds["video"], data)
			if err != nil || !strings.Contains(out, `"status":"succeeded"`) || !strings.Contains(out, `"saved":false`) || !strings.Contains(out, `"points":80`) || strings.Contains(out, `"file"`) || calls != tc.wantCalls {
				t.Fatal(out, err, calls)
			}
		})
	}
}

func TestVideoPendingFailureAndCancellation(t *testing.T) {
	s, _ := kindFixture(t, "video", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "failed") {
			fmt.Fprint(w, `{"code":"ok","data":{"job_id":"failed","status":"failed","points":98,"error_code":"private-code","error":"secret configuration"}}`)
			return
		}
		fmt.Fprint(w, `{"code":"ok","data":{"job_id":"queued","status":"queued","points":98}}`)
	})
	out, err := s.waitForVideo(context.Background(), kinds["video"], "key-1", "queued", time.Millisecond, 15*time.Millisecond)
	if err != nil || !strings.Contains(out, `"status":"queued"`) || !strings.Contains(out, `"points":98`) || strings.Contains(out, `"saved":true`) {
		t.Fatal(out, err)
	}
	out, err = s.executeKind(context.Background(), kinds["video"], "get_video", "key-1", map[string]string{"job_id": "failed"})
	if err != nil || !strings.Contains(out, `"points":0`) || !strings.Contains(out, `"error_code":"video_failed"`) || strings.Contains(out, "private") || strings.Contains(out, "secret") {
		t.Fatal(out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.waitForVideo(ctx, kinds["video"], "key-1", "queued", time.Millisecond, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestVideoPublicErrorsAndAmbiguousCreateAreNotRetried(t *testing.T) {
	var code atomic.Int32
	var creates atomic.Int32
	s, _ := kindFixture(t, "video", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			fmt.Fprint(w, videoCaps)
			return
		}
		creates.Add(1)
		w.WriteHeader(int(code.Load()))
		fmt.Fprint(w, `{"message":"internal-provider-key-and-wallet-details"}`)
	})
	code.Store(402)
	out, err := s.executeKind(context.Background(), kinds["video"], "create_video", "key-1", map[string]string{"prompt": "a cat"})
	if err != nil || !strings.Contains(out, "充值") || strings.Contains(out, "internal") {
		t.Fatal(out, err)
	}
	code.Store(502)
	p := &scripted{name: "c_k1_create_video", args: `{"prompt":"a cat"}`}
	if _, err := loop(s, p); !errors.Is(err, ErrApproval) {
		t.Fatal(err)
	}
	inv := pending(t, s)
	s.DB.Exec("UPDATE runs SET status='waiting_approval' WHERE id='r'")
	if err := s.Decide(inv.ID, "a", true); err != nil {
		t.Fatal(err)
	}
	s.DB.Exec("UPDATE runs SET status='running' WHERE id='r'")
	if _, err := loop(s, p); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	if _, err := loop(s, p); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	if creates.Load() != 2 {
		t.Fatal("ambiguous paid submission repeated", creates.Load())
	}
}
