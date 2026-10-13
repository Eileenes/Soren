package connectors

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

var (
	imageRatios      = []string{"auto", "1:1", "4:3", "3:4", "16:9", "9:16"}
	imageResolutions = []string{"1K", "2K", "4K"}
)

func imageBody(a map[string]string) (map[string]any, error) {
	prompt, e := textArg(a, "prompt", 4000)
	if e != nil {
		return nil, e
	}
	body := map[string]any{"prompt": prompt}
	if n, ok, e := intArg(a, "image_count", 1, 4); e != nil {
		return nil, e
	} else if ok {
		body["image_count"] = n
	}
	if n, ok, e := intArg(a, "seed", 0, 1<<31-1); e != nil {
		return nil, e
	} else if ok {
		body["seed"] = n
	}
	for name, allowed := range map[string][]string{"ratio": imageRatios, "resolution": imageResolutions} {
		v, e := oneOf(a, name, allowed...)
		if e != nil {
			return nil, e
		}
		if v != "" {
			body[name] = v
		}
	}
	return body, nil
}

// saveTaskImages downloads finished task images (public URLs, no API key sent).
func saveTaskImages(ctx context.Context, s *Service, k *kindSpec, body string) (string, error) {
	var res struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status    string   `json:"status"`
			ImageURLs []string `json:"image_urls"`
		} `json:"data"`
	}
	if e := decode(body, &res); e != nil {
		return body, nil
	}
	out := map[string]any{"code": res.Code, "message": res.Message, "status": res.Data.Status}
	if res.Data.Status != "completed" {
		return jsonText(out), nil
	}
	base, _ := url.Parse(s.base(k))
	files := []any{}
	for i, raw := range res.Data.ImageURLs {
		if i >= 8 {
			break
		}
		u, e := url.Parse(raw)
		if e != nil || u.User != nil || !(u.Scheme == "https" || (base != nil && u.Host == base.Host)) {
			return "", errors.New("图片地址无效")
		}
		ext := strings.ToLower(path.Ext(u.Path))
		if ext == "" || len(ext) > 5 {
			ext = ".png"
		}
		resp, e := s.fetchPublic(ctx, u.String())
		if e != nil {
			return "", e
		}
		saved, e := s.store(k, resp, fmt.Sprintf("%s-%d%s", cleanName(path.Base(u.Path), "image"), i+1, ext))
		if e != nil {
			return "", e
		}
		var f map[string]any
		_ = decode(saved, &f)
		f["url"] = raw
		files = append(files, f)
	}
	out["files"] = files
	return jsonText(out), nil
}

func init() {
	common := map[string]string{"prompt": "提示词，描述想要的图像或变化", "image_count": "生成数量 1–4，默认 1", "ratio": "比例：auto、1:1、4:3、3:4、16:9、9:16", "resolution": "分辨率：1K、2K、4K，默认 2K", "seed": "可选随机种子（整数）"}
	edit := map[string]string{"image_urls": "参考图 HTTPS URL，多张用换行或逗号分隔（最多 8 张）"}
	for k, v := range common {
		edit[k] = v
	}
	registerKind(&kindSpec{
		Kind: "image", Title: "AI 生图", Base: "https://image-generate.app.baizhi.cloud/openapi/v1", Check: "/pricing",
		Tools: []kindTool{
			{Action: "text_to_image", Desc: "根据文字提示词创建异步生图任务（按成功图片消耗积分，需要确认）。返回 task_id，用 get_task 查询并保存图片。", Approval: true,
				Props: common, Required: []string{"prompt"},
				Build: func(a map[string]string) (apiCall, error) {
					b, e := imageBody(a)
					return apiCall{Method: "POST", Path: "/text_to_image", Body: b}, e
				}},
			{Action: "edit_image", Desc: "基于参考图创建图生图任务（风格变换、重绘等；消耗积分，需要确认）。", Approval: true,
				Props: edit, Required: []string{"prompt", "image_urls"},
				Build: func(a map[string]string) (apiCall, error) {
					b, e := imageBody(a)
					if e != nil {
						return apiCall{}, e
					}
					urls := []string{}
					for _, raw := range strings.FieldsFunc(a["image_urls"], func(r rune) bool { return r == '\n' || r == ',' || r == ' ' }) {
						u, e := url.Parse(raw)
						if e != nil || u.Scheme != "https" || u.Host == "" {
							return apiCall{}, errors.New("参考图必须是 HTTPS URL")
						}
						urls = append(urls, raw)
					}
					if len(urls) == 0 || len(urls) > 8 {
						return apiCall{}, errors.New("请提供 1–8 张参考图")
					}
					b["image_urls"] = urls
					return apiCall{Method: "POST", Path: "/edit_image", Body: b}, nil
				}},
			{Action: "get_task", Desc: "查询生图任务状态（processing、running、completed、failed）；完成后把图片保存到本机并返回文件路径。",
				Props: map[string]string{"task_id": "任务 ID"}, Required: []string{"task_id"}, After: saveTaskImages,
				Build: func(a map[string]string) (apiCall, error) {
					id, e := pathID(a, "task_id")
					return apiCall{Method: "GET", Path: "/tasks/" + id}, e
				}},
			{Action: "retry_task", Desc: "重试失败或部分成功的生图任务（可能消耗积分，需要确认）。", Approval: true,
				Props: map[string]string{"task_id": "任务 ID"}, Required: []string{"task_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := pathID(a, "task_id")
					return apiCall{Method: "POST", Path: "/tasks/" + id + "/retry"}, e
				}},
			{Action: "pricing", Desc: "查询文生图、图生图的积分价格（只读）。",
				Props: map[string]string{},
				Build: func(map[string]string) (apiCall, error) { return apiCall{Method: "GET", Path: "/pricing"}, nil }},
		},
	})
}
