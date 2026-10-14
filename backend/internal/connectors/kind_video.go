package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Keep the wire format and prices owned by VideoAPI. No video is created while
// validating credentials, querying capabilities, or polling an existing job.
func init() {
	registerKind(&kindSpec{
		Kind: "video", Title: "视频 API", Base: "https://video-api.app.baizhi.cloud/openapi/v1", Check: "/video-capabilities",
		Tools: []kindTool{
			{Action: "get_video_capabilities", Desc: "查询当前视频规格、时长范围、图生视频支持情况和积分价格。生成前必须查询，以返回值为准，不推测时长或价格。只向用户说明公开规格、状态和积分，不解释内部模型、供应商、数据源或钱包标识。", Props: map[string]string{}, Execute: videoCapabilities},
			{Action: "create_video", Desc: "提交视频生成任务（消耗 API Key 所属账号的积分，需要确认）。先查询 get_video_capabilities。prompt 必填；可提供公网 HTTPS 原图 image_url 进行图生视频。不要传本地路径、base64 或聊天预览图；没有原图 URL 时应请用户提供，不得擅自改为文生视频。文生视频不传 resolution；图生视频不传则由服务决定默认值。不传时长按实际成片计费。返回 job_id 后使用 get_video 查询，只有 succeeded 才表示视频已完成，切勿重复创建同一任务。", Approval: true,
				Props: map[string]string{"prompt": "视频内容、运动和镜头描述", "image_url": "可选公网 HTTPS 原始图片地址；不能是本地路径或 base64", "duration_seconds": "可选秒数，依据最新 capabilities 和用户要求，不要自行固定时长", "aspect_ratio": "可选画面比例，依据最新 capabilities", "resolution": "仅图生视频可填，依据最新 capabilities；文生视频不要传"}, Required: []string{"prompt"}, Execute: videoCreate},
			{Action: "get_video", Desc: "查询当前 API Key 所属用户和团队的视频任务及积分 points。queued/running 等非终态会等待并查询最多约 30 秒，仍未完成时保留 job_id 稍后再查，不要重新创建任务。succeeded 时将临时 asset_url 原样下载到本机并返回文件信息；failed 为失败，points 为 0。排队中 points 为预扣，成功时为最终扣费。本工具不重新生成视频。",
				Props: map[string]string{"job_id": "create_video 返回的任务 ID"}, Required: []string{"job_id"}, Execute: videoGet},
		},
	})
}

type videoAPIError struct {
	status int
	public string
}

func (e *videoAPIError) Error() string { return e.public }

// Never expose service error bodies, transport URLs, credentials, or internals.
func (s *Service) videoRequest(ctx context.Context, k *kindSpec, key, method, path string, body any) (map[string]json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, errors.New("视频参数无效")
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base(k)+path, reader)
	if err != nil {
		return nil, errors.New("视频服务地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("视频服务暂时不可用，请稍后查询任务状态")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := "视频服务暂时不可用，请稍后再试"
		switch res.StatusCode {
		case 401, 403:
			message = "视频 API Key 无效、已停用或权限不足，请重新连接"
		case 402:
			message = "积分不足，请到 API Key 所属账号充值"
		case 404:
			message = "视频任务不存在或不属于当前 API Key，请核对账号和任务 ID"
		case 400, 422:
			message = "视频参数或图片内容不符合要求，请核对当前规格和原图"
		case 429:
			message = "视频服务请求过于频繁，请稍后再试"
		}
		return nil, &videoAPIError{res.StatusCode, message}
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, errors.New("视频服务返回的数据无效")
	}
	var envelope struct {
		Code string                     `json:"code"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Code != "ok" || len(envelope.Data) == 0 {
		return nil, errors.New("视频服务未返回有效结果，请稍后查询任务状态")
	}
	return envelope.Data, nil
}

func videoCapabilities(ctx context.Context, s *Service, k *kindSpec, key string, _ map[string]string) (string, error) {
	data, err := s.videoRequest(ctx, k, key, "GET", k.Check, nil)
	if err != nil {
		return videoProblem("unavailable", err), nil
	}
	return jsonText(data), nil
}

func videoProblem(status string, err error) string {
	return jsonText(map[string]any{"status": status, "message": err.Error()})
}

func publicVideoURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsMulticast()) {
		return false
	}
	return true
}

func videoBody(args map[string]string) (map[string]any, error) {
	prompt := strings.TrimSpace(args["prompt"])
	if prompt == "" {
		return nil, errors.New("请填写视频描述")
	}
	body := map[string]any{"prompt": prompt}
	if image := strings.TrimSpace(args["image_url"]); image != "" {
		if !publicVideoURL(image) {
			return nil, errors.New("请提供公网 HTTPS 原图地址；本地原图上传暂不支持")
		}
		body["image_url"] = image
	}
	if args["image_base64"] != "" || args["image"] != "" {
		return nil, errors.New("请提供公网 HTTPS 原图地址，不要传图片编码或本地路径")
	}
	if raw := strings.TrimSpace(args["duration_seconds"]); raw != "" {
		seconds, err := strconv.ParseFloat(raw, 64)
		if err != nil || seconds <= 0 || !json.Valid([]byte(raw)) {
			return nil, errors.New("视频时长必须是正数，请根据当前规格填写")
		}
		body["duration_seconds"] = seconds
	}
	if ratio := strings.TrimSpace(args["aspect_ratio"]); ratio != "" {
		body["aspect_ratio"] = ratio
	}
	if resolution := strings.TrimSpace(args["resolution"]); resolution != "" {
		if body["image_url"] == nil {
			return nil, errors.New("文生视频不要指定 resolution；请先查询当前规格")
		}
		body["resolution"] = resolution
	}
	return body, nil
}

func videoCreate(ctx context.Context, s *Service, k *kindSpec, key string, args map[string]string) (string, error) {
	body, err := videoBody(args)
	if err != nil {
		return videoProblem("not_submitted", err), nil
	}
	// Recheck current capabilities before every paid submission. Fail closed if
	// the service is unavailable; do not silently submit a different video type.
	capabilities, err := s.videoRequest(ctx, k, key, "GET", k.Check, nil)
	if err != nil {
		return videoProblem("not_submitted", err), nil
	}
	if body["image_url"] != nil && string(capabilities["supports_image_to_video"]) == "false" {
		return videoProblem("not_submitted", errors.New("当前服务不支持图生视频，任务未提交")), nil
	}
	data, err := s.videoRequest(ctx, k, key, "POST", "/videos", body)
	if err != nil {
		var public *videoAPIError
		if errors.As(err, &public) && public.status >= 400 && public.status < 500 && public.status != 408 {
			return videoProblem("not_submitted", err), nil
		}
		// An ambiguous paid POST must use the existing unknown/replay protection.
		return "", err
	}
	return s.videoResult(ctx, k, data)
}

func videoGet(ctx context.Context, s *Service, k *kindSpec, key string, args map[string]string) (string, error) {
	id, err := pathID(args, "job_id")
	if err != nil {
		return videoProblem("invalid_request", errors.New("视频任务 ID 无效")), nil
	}
	return s.waitForVideo(ctx, k, key, id, 5*time.Second, 30*time.Second)
}

// Bounded polling saves model calls while preserving a pending job at timeout.
func (s *Service) waitForVideo(ctx context.Context, k *kindSpec, key, id string, interval, window time.Duration) (string, error) {
	pollCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	var latest map[string]json.RawMessage
	for {
		data, err := s.videoRequest(pollCtx, k, key, "GET", "/videos/"+id, nil)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			if pollCtx.Err() != nil && latest != nil {
				return s.videoResult(ctx, k, latest)
			}
			return jsonText(map[string]any{"status": "unavailable", "job_id": id, "message": err.Error()}), nil
		}
		if data["job_id"] == nil && data["id"] == nil {
			data["job_id"], _ = json.Marshal(id)
		}
		latest = data
		status := videoString(data, "status")
		if status != "queued" && status != "pending" && status != "running" && status != "processing" {
			return s.videoResult(ctx, k, data)
		}
		timer := time.NewTimer(interval)
		select {
		case <-pollCtx.Done():
			timer.Stop()
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return s.videoResult(ctx, k, latest)
		case <-timer.C:
		}
	}
}

func videoString(data map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(data[key], &value)
	return value
}

func (s *Service) videoResult(ctx context.Context, k *kindSpec, data map[string]json.RawMessage) (string, error) {
	out := map[string]any{}
	for _, field := range []string{"job_id", "id", "status", "points", "duration_seconds", "aspect_ratio", "resolution", "created_at", "updated_at"} {
		if v, ok := data[field]; ok {
			out[field] = v
		}
	}
	status := videoString(data, "status")
	if status == "" || (videoString(data, "job_id") == "" && videoString(data, "id") == "") {
		return "", errors.New("视频服务未返回有效的任务状态和 ID，请核实后再操作，不要重复生成")
	}
	if status == "failed" {
		out["error_code"], out["points"] = "video_failed", 0
		out["message"] = "视频生成失败，本次任务积分为 0。"
		return jsonText(out), nil
	}
	if status != "succeeded" {
		out["message"] = "视频尚未完成，请保留任务 ID 继续查询；points 为预扣积分，不要重复提交生成。"
		return jsonText(out), nil
	}
	var result map[string]json.RawMessage
	_ = json.Unmarshal(data["result"], &result)
	asset := videoString(result, "asset_url")
	if asset == "" {
		asset = videoString(data, "asset_url")
	}
	if asset == "" || !publicVideoURL(asset) {
		out["message"] = "任务已成功，但未返回有效的视频下载地址，请重新查询。"
		out["saved"] = false
		return jsonText(out), nil
	}
	file, err := s.saveVideo(ctx, k, asset)
	if err != nil {
		out["message"] = "视频已生成，但本机保存未完成，请用同一任务 ID 重新查询下载，不要重新生成。"
		out["saved"] = false
	} else {
		out["file"], out["saved"] = json.RawMessage(file), true
	}
	return jsonText(out), nil
}

func (s *Service) saveVideo(ctx context.Context, k *kindSpec, asset string) (string, error) {
	// Use the signed URL verbatim. No key, cookies, parsing of internal object
	// paths, redirects, or proxy access to local/private networks.
	req, err := http.NewRequestWithContext(ctx, "GET", asset, nil)
	if err != nil {
		return "", err
	}
	res, err := s.DownloadHTTP.Do(req)
	if err != nil {
		return "", err
	}
	media, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if media != "application/octet-stream" && !strings.HasPrefix(media, "video/") {
		res.Body.Close()
		return "", errors.New("下载内容不是视频")
	}
	ext := ".bin"
	switch media {
	case "video/mp4":
		ext = ".mp4"
	case "video/quicktime":
		ext = ".mov"
	default:
		if extensions, err := mime.ExtensionsByType(media); err == nil && len(extensions) > 0 {
			ext = extensions[0]
		}
	}
	return s.store(k, res, fmt.Sprintf("video-%s%s", uuid.NewString(), ext))
}
