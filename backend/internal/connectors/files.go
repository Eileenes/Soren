package connectors

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FileArtifact is an immutable snapshot of a connector download. The server path
// is never accepted from the client or exposed by the file-list API.
type FileArtifact struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	RunID     string    `gorm:"index" json:"runId"`
	BotID     string    `gorm:"index" json:"botId"`
	Name      string    `json:"name"`
	Path      string    `json:"-"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Only built-in connectors produce trusted local file metadata; MCP results are
// untrusted remote content and must never authorize reading a server file.
func (s *Service) captureFiles(bot, run, output string) (string, error) {
	var value any
	if json.Unmarshal([]byte(output), &value) != nil {
		return output, nil
	}
	root, err := filepath.EvalSymlinks(filepath.Join(s.DataDir, "connectors"))
	if os.IsNotExist(err) {
		return output, nil
	}
	if err != nil {
		return "", err
	}
	count := 0
	var walk func(any) error
	walk = func(v any) error {
		switch item := v.(type) {
		case []any:
			for _, child := range item {
				if err := walk(child); err != nil {
					return err
				}
			}
		case map[string]any:
			if path, ok := item["path"].(string); ok && item["sha256"] != nil && item["bytes"] != nil {
				real, e := filepath.EvalSymlinks(path)
				if e != nil || !inside(root, real) {
					return errors.New("成果文件不在允许的目录中")
				}
				count++
				if count > 16 {
					return errors.New("单次下载文件数量过多")
				}
				src, e := os.Open(real)
				if e != nil {
					return e
				}
				defer src.Close()
				info, e := src.Stat()
				if e != nil || !info.Mode().IsRegular() || info.Size() > maxDownload {
					return errors.New("成果文件无效或过大")
				}
				dir := filepath.Join(s.DataDir, "attachments", run)
				if e = os.MkdirAll(dir, 0700); e != nil {
					return e
				}
				id := uuid.NewString()
				target := filepath.Join(dir, id)
				dst, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					return e
				}
				n, e := io.Copy(dst, io.LimitReader(src, maxDownload+1))
				closeErr := dst.Close()
				if e != nil || closeErr != nil || n != info.Size() {
					_ = os.Remove(target)
					return errors.New("保存成果文件失败")
				}
				file := FileArtifact{ID: id, RunID: run, BotID: bot, Name: filepath.Base(real), Path: target, Size: n}
				if e = s.DB.Create(&file).Error; e != nil {
					_ = os.Remove(target)
					return e
				}
				delete(item, "path")
				item["fileId"] = id
				item["fileName"] = file.Name
				item["download"] = "请使用本次调用的成果下载按钮"
			}
			for _, child := range item {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = walk(value); err != nil {
		return "", err
	}
	raw, err := json.Marshal(value)
	return string(raw), err
}

func (s *Service) OpenFile(run, id string) (*FileArtifact, *os.File, error) {
	var file FileArtifact
	if err := s.DB.First(&file, "id = ? AND run_id = ?", id, run).Error; err != nil {
		return nil, nil, err
	}
	root, err := filepath.EvalSymlinks(filepath.Join(s.DataDir, "attachments", run))
	if err != nil {
		return nil, nil, err
	}
	real, err := filepath.EvalSymlinks(file.Path)
	if err != nil || !inside(root, real) {
		return nil, nil, errors.New("成果文件不可用")
	}
	f, err := os.Open(real)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size || file.Size > maxDownload {
		f.Close()
		return nil, nil, errors.New("成果文件已变更")
	}
	return &file, f, nil
}
