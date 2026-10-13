package connectors

import "net/url"

var musicGenres = []string{"pop", "chinese-style", "folk", "children", "rock", "electronic", "hip-hop", "jazz", "classical"}

func musicMessage(a map[string]string) (map[string]any, error) {
	msg, e := textArg(a, "message", 8000)
	if e != nil {
		return nil, e
	}
	genre, e := oneOf(a, "genre", musicGenres...)
	if e != nil {
		return nil, e
	}
	body := map[string]any{"message": msg}
	if genre != "" {
		body["guide"] = map[string]any{"genre": genre}
	}
	return body, nil
}

func init() {
	genre := "可选曲风：pop、chinese-style、folk、children、rock、electronic、hip-hop、jazz、classical"
	registerKind(&kindSpec{
		Kind: "music", Title: "AI 音乐", Base: "https://ai-sound-lab.app.baizhi.cloud/openapi/music/api", Check: "/generations?limit=1",
		Tools: []kindTool{
			{Action: "start_creation", Desc: "发起一次音乐创作，整理创作方案（brief）。返回 creation_id、message_id、status；status 为 processing 时用 get_creation 轮询。",
				Props: map[string]string{"message": "创作描述，1–8000 字", "genre": genre}, Required: []string{"message"},
				Build: func(a map[string]string) (apiCall, error) {
					b, e := musicMessage(a)
					return apiCall{Method: "POST", Path: "/creations", Body: b}, e
				}},
			{Action: "continue_creation", Desc: "在已有创作中补充修改要求，只需写本次要改的内容。",
				Props: map[string]string{"creation_id": "创作 ID", "message": "补充要求，1–8000 字", "genre": genre}, Required: []string{"creation_id", "message"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "creation_id")
					if e != nil {
						return apiCall{}, e
					}
					b, e := musicMessage(a)
					return apiCall{Method: "POST", Path: "/creations/" + id + "/messages", Body: b}, e
				}},
			{Action: "get_creation", Desc: "查询创作的最新状态和方案（processing、succeeded、failed）。",
				Props: map[string]string{"creation_id": "创作 ID"}, Required: []string{"creation_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "creation_id")
					return apiCall{Method: "GET", Path: "/creations/" + id}, e
				}},
			{Action: "retry_plan", Desc: "重试状态为 failed 的创作方案。", Approval: true,
				Props: map[string]string{"creation_id": "创作 ID", "message_id": "失败方案的消息 ID"}, Required: []string{"creation_id", "message_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "creation_id")
					if e != nil {
						return apiCall{}, e
					}
					m, e := uuidArg(a, "message_id")
					return apiCall{Method: "POST", Path: "/creations/" + id + "/messages/" + m + "/retry"}, e
				}},
			{Action: "generate_music", Desc: "按已成功的方案生成音乐（消耗积分，需要用户确认）。返回音乐任务 id，用 get_generation 轮询。", Approval: true,
				Props: map[string]string{"creation_id": "创作 ID", "message_id": "方案成功的消息 ID"}, Required: []string{"creation_id", "message_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "creation_id")
					if e != nil {
						return apiCall{}, e
					}
					m, e := uuidArg(a, "message_id")
					return apiCall{Method: "POST", Path: "/creations/" + id + "/generations", Body: map[string]any{"message_id": m}}, e
				}},
			{Action: "get_generation", Desc: "查询音乐任务状态（queued、running、succeeded、failed、expired）、标题、时长和歌词。",
				Props: map[string]string{"generation_id": "音乐任务 ID"}, Required: []string{"generation_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "generation_id")
					return apiCall{Method: "GET", Path: "/generations/" + id}, e
				}},
			{Action: "list_generations", Desc: "分页列出我的音乐任务。",
				Props: map[string]string{"limit": "1–100，默认 20", "cursor": "上一页的 next_cursor"},
				Build: func(a map[string]string) (apiCall, error) {
					if _, _, e := intArg(a, "limit", 1, 100); e != nil {
						return apiCall{}, e
					}
					return apiCall{Method: "GET", Path: "/generations", Query: query(a, "limit", "cursor")}, nil
				}},
			{Action: "get_audio", Desc: "下载已成功的音乐为 mp3，保存到本机并返回文件路径与时长（秒）。",
				Props: map[string]string{"generation_id": "音乐任务 ID"}, Required: []string{"generation_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "generation_id")
					return apiCall{Method: "GET", Path: "/generations/" + id + "/audio", Query: url.Values{"format": {"mp3"}}, SaveAs: id + ".mp3"}, e
				}},
			{Action: "retry_generation", Desc: "重试 failed 或 expired 的音乐任务（可能消耗积分，需要确认）。", Approval: true,
				Props: map[string]string{"generation_id": "音乐任务 ID"}, Required: []string{"generation_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "generation_id")
					return apiCall{Method: "POST", Path: "/generations/" + id + "/retry"}, e
				}},
			{Action: "delete_generation", Desc: "删除一个音乐任务（不可恢复，需要确认）。", Approval: true,
				Props: map[string]string{"generation_id": "音乐任务 ID"}, Required: []string{"generation_id"},
				Build: func(a map[string]string) (apiCall, error) {
					id, e := uuidArg(a, "generation_id")
					return apiCall{Method: "DELETE", Path: "/generations/" + id}, e
				}},
		},
	})
}
