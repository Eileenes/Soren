package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Baizhi Cloud apps (music, image, video, browser, design prompt) share one shape:
// a fixed HTTPS OpenAPI prefix, `Authorization: Bearer <API Key>`, and a set
// of REST endpoints. Each kind is described declaratively below.

type apiCall struct {
	Method  string
	Path    string
	Query   url.Values
	Body    any
	Headers map[string]string
	// SaveAs downloads the binary response into DataDir instead of returning it.
	SaveAs string
}

type kindTool struct {
	Action   string
	Desc     string
	Props    map[string]string
	Required []string
	Approval bool
	Build    func(args map[string]string) (apiCall, error)
	Execute  func(context.Context, *Service, *kindSpec, string, map[string]string) (string, error)
	// After optionally post-processes a JSON response (for example to save result files).
	After func(ctx context.Context, s *Service, spec *kindSpec, body string) (string, error)
}

type kindSpec struct {
	Kind  string
	Title string
	Base  string // default OpenAPI prefix; overridable through Service.APIBase for tests
	Check string // side-effect-free GET used to validate a new API key
	Tools []kindTool
}

var kinds = map[string]*kindSpec{}

func registerKind(k *kindSpec) { kinds[k.Kind] = k }

// KindSpec returns the declarative spec for a Baizhi connector kind.
func KindSpec(kind string) (*kindSpec, bool) { k, ok := kinds[kind]; return k, ok }

func (k *kindSpec) Name() string { return k.Title }

func (s *Service) base(k *kindSpec) string {
	if v := s.APIBase[k.Kind]; v != "" {
		return strings.TrimRight(v, "/")
	}
	return k.Base
}

// CheckKey calls the kind's free validation endpoint with the candidate key.
func (s *Service) CheckKey(ctx context.Context, kind, key string) error {
	k, ok := kinds[kind]
	if !ok {
		return errors.New("连接类型无效")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("请填写%s API Key", k.Title)
	}
	if kind == "video" {
		_, err := s.videoRequest(ctx, k, key, "GET", k.Check, nil)
		return err
	}
	req, e := http.NewRequestWithContext(ctx, "GET", s.base(k)+k.Check, nil)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+key)
	res, e := s.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("无法连接%s", k.Title)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return fmt.Errorf("%s API Key 无效或已停用", k.Title)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%s 校验失败（状态 %d）", k.Title, res.StatusCode)
	}
	return nil
}

func (s *Service) addKindTools(c Connection, add func(string, string, map[string]any, bool, ...string)) bool {
	k, ok := kinds[c.Kind]
	if !ok {
		return false
	}
	for _, t := range k.Tools {
		props := map[string]any{}
		for name, desc := range t.Props {
			props[name] = str(desc)
		}
		add(t.Action, t.Desc, props, t.Approval, t.Required...)
	}
	return true
}

func (s *Service) executeKind(ctx context.Context, k *kindSpec, action, key string, args map[string]string) (string, error) {
	for _, t := range k.Tools {
		if t.Action != action {
			continue
		}
		if t.Execute != nil {
			return t.Execute(ctx, s, k, key, args)
		}
		call, e := t.Build(args)
		if e != nil {
			return "", e
		}
		endpoint := s.base(k) + call.Path
		if len(call.Query) > 0 {
			endpoint += "?" + call.Query.Encode()
		}
		var out string
		if call.SaveAs != "" {
			out, e = s.saveDownload(ctx, k, endpoint, key, call.SaveAs)
		} else {
			out, e = s.request(ctx, call.Method, endpoint, key, call.Body, call.Headers)
		}
		if e != nil || t.After == nil {
			return out, e
		}
		return t.After(ctx, s, k, out)
	}
	return "", errors.New("不支持的工具")
}

const maxDownload = 200 << 20

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._\-\p{Han}]+`)

func (s *Service) outputDir(k *kindSpec) (string, error) {
	dir := filepath.Join(s.DataDir, "connectors", k.Kind)
	return dir, os.MkdirAll(dir, 0o700)
}

func cleanName(name, fallback string) string {
	name = unsafeName.ReplaceAllString(filepath.Base(name), "_")
	name = strings.Trim(name, "._")
	if name == "" {
		return fallback
	}
	if len(name) > 120 {
		name = name[len(name)-120:]
	}
	return name
}

// saveDownload fetches an authenticated binary endpoint. A single redirect to an
// HTTPS location (e.g. signed object storage) is followed without the API key.
func (s *Service) saveDownload(ctx context.Context, k *kindSpec, endpoint, key, name string) (string, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("Authorization", "Bearer "+key)
	// Never let the client follow redirects with the API key attached.
	client := *s.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, e := client.Do(req)
	if e != nil {
		return "", e
	}
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, le := res.Location()
		res.Body.Close()
		if le != nil || loc.Scheme != "https" {
			return "", errors.New("下载重定向地址无效")
		}
		if res, e = s.fetchPublic(ctx, loc.String()); e != nil {
			return "", e
		}
	}
	return s.store(k, res, name)
}

func (s *Service) fetchPublic(ctx context.Context, raw string) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if e != nil {
		return nil, e
	}
	return s.HTTP.Do(req)
}

func (s *Service) store(k *kindSpec, res *http.Response, name string) (string, error) {
	defer res.Body.Close()
	if res.StatusCode == 409 {
		return "", errors.New("文件尚未生成完成，请稍后再试")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("服务返回状态 %d", res.StatusCode)
	}
	if _, params, e := mime.ParseMediaType(res.Header.Get("Content-Disposition")); e == nil && params["filename"] != "" && strings.HasPrefix(name, "*") {
		name = params["filename"]
	}
	name = cleanName(strings.TrimPrefix(name, "*"), uuid.NewString())
	dir, e := s.outputDir(k)
	if e != nil {
		return "", e
	}
	path := filepath.Join(dir, name)
	f, e := os.CreateTemp(dir, ".download-*")
	if e != nil {
		return "", e
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxDownload+1))
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return "", e
	}
	if n > maxDownload {
		return "", errors.New("文件过大")
	}
	if n == 0 {
		return "", errors.New("下载内容为空")
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if want := res.Header.Get("X-Content-SHA256"); want != "" && !strings.EqualFold(want, sum) {
		return "", errors.New("文件校验失败")
	}
	if e = os.Rename(f.Name(), path); e != nil {
		return "", e
	}
	out := map[string]any{"path": path, "bytes": n, "sha256": sum, "contentType": res.Header.Get("Content-Type")}
	if d := res.Header.Get("X-Audio-Duration"); d != "" {
		if v, pe := strconv.ParseFloat(d, 64); pe == nil {
			out["durationSeconds"] = v
		}
	}
	return jsonText(out), nil
}

// ---- argument helpers ----

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:\-]{0,127}$`)

func pathID(args map[string]string, name string) (string, error) {
	v := strings.TrimSpace(args[name])
	if !safeID.MatchString(v) || strings.Contains(v, "..") {
		return "", fmt.Errorf("%s 无效", name)
	}
	return url.PathEscape(v), nil
}

func uuidArg(args map[string]string, name string) (string, error) {
	id, e := uuid.Parse(strings.TrimSpace(args[name]))
	if e != nil {
		return "", fmt.Errorf("%s 必须是 UUID", name)
	}
	return id.String(), nil
}

func intArg(args map[string]string, name string, lo, hi int) (int, bool, error) {
	v := strings.TrimSpace(args[name])
	if v == "" {
		return 0, false, nil
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < lo || n > hi {
		return 0, false, fmt.Errorf("%s 必须是 %d–%d 的整数", name, lo, hi)
	}
	return n, true, nil
}

func oneOf(args map[string]string, name string, allowed ...string) (string, error) {
	v := strings.TrimSpace(args[name])
	if v == "" {
		return "", nil
	}
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", fmt.Errorf("%s 只能是 %s", name, strings.Join(allowed, "、"))
}

func textArg(args map[string]string, name string, max int) (string, error) {
	v := strings.TrimSpace(args[name])
	if v == "" || len([]rune(v)) > max {
		return "", fmt.Errorf("%s 不能为空且不超过 %d 字", name, max)
	}
	return v, nil
}

// query copies non-empty optional string args into url.Values.
func query(args map[string]string, names ...string) url.Values {
	q := url.Values{}
	for _, n := range names {
		if v := strings.TrimSpace(args[n]); v != "" {
			q.Set(n, v)
		}
	}
	return q
}

func decode(body string, v any) error { return json.Unmarshal([]byte(body), v) }
