package connectors

import (
	"github.com/google/uuid"
)

func pageArgs(a map[string]string, extra ...string) (apiCall, error) {
	if _, _, e := intArg(a, "page", 1, 10000); e != nil {
		return apiCall{}, e
	}
	if _, _, e := intArg(a, "page_size", 1, 100); e != nil {
		return apiCall{}, e
	}
	return apiCall{Method: "GET", Query: query(a, append([]string{"page", "page_size"}, extra...)...)}, nil
}

func browserID(a map[string]string, name, prefix, suffix, method string) (apiCall, error) {
	id, e := pathID(a, name)
	return apiCall{Method: method, Path: prefix + id + suffix}, e
}

func init() {
	sid := map[string]string{"session_id": "运行（Session）ID"}
	pid := map[string]string{"profile_id": "档案（Profile）ID"}
	pages := map[string]string{"page": "页码，默认 1", "page_size": "每页数量 1–100，默认 20", "status": "可选状态筛选", "query": "可选关键词"}
	registerKind(&kindSpec{
		Kind: "browser", Title: "云浏览器", Base: "https://capybrowser.app.baizhi.cloud/openapi/v1", Check: "/me",
		Tools: []kindTool{
			{Action: "create_session", Desc: "启动一个云浏览器运行（按时长计费，需要确认）。返回 session_id，轮询 get_session 直到 ready。用完务必 stop_session。", Approval: true,
				Props: map[string]string{"max_seconds": "最长运行秒数 1–7000，默认 900", "profile_id": "可选：复用登录状态的档案 ID"},
				Build: func(a map[string]string) (apiCall, error) {
					body := map[string]any{}
					if n, ok, e := intArg(a, "max_seconds", 1, 7000); e != nil {
						return apiCall{}, e
					} else if ok {
						body["max_seconds"] = n
					}
					if a["profile_id"] != "" {
						id, e := pathID(a, "profile_id")
						if e != nil {
							return apiCall{}, e
						}
						body["profile_id"] = id
					}
					// Each approved invocation executes once, so one key per call is stable.
					return apiCall{Method: "POST", Path: "/browser/sessions", Body: body, Headers: map[string]string{"Idempotency-Key": uuid.NewString()}}, nil
				}},
			{Action: "get_session", Desc: "查询运行状态（starting、ready、stopping、released、failed 等）、到期时间和计费信息。",
				Props: sid, Required: []string{"session_id"},
				Build: func(a map[string]string) (apiCall, error) {
					return browserID(a, "session_id", "/browser/sessions/", "", "GET")
				}},
			{Action: "list_sessions", Desc: "分页列出我的云浏览器运行。", Props: pages,
				Build: func(a map[string]string) (apiCall, error) {
					c, e := pageArgs(a, "status", "query")
					c.Path = "/browser/sessions"
					return c, e
				}},
			{Action: "stop_session", Desc: "结束并释放一个运行（停止计费，结束后无法再下载文件；需要确认）。", Approval: true,
				Props: sid, Required: []string{"session_id"},
				Build: func(a map[string]string) (apiCall, error) {
					return browserID(a, "session_id", "/browser/sessions/", "", "DELETE")
				}},
			{Action: "begin_handoff", Desc: "需要用户本人登录或授权时，为运行生成「人工处理」链接（human_action.url），交给用户打开。",
				Props: sid, Required: []string{"session_id"},
				Build: func(a map[string]string) (apiCall, error) {
					c, e := browserID(a, "session_id", "/browser/sessions/", "/handoff", "POST")
					c.Body = map[string]any{}
					return c, e
				}},
			{Action: "list_downloads", Desc: "列出运行中浏览器下载的文件（只有 state 为 ready 的可下载）。",
				Props: sid, Required: []string{"session_id"},
				Build: func(a map[string]string) (apiCall, error) {
					return browserID(a, "session_id", "/browser/sessions/", "/downloads", "GET")
				}},
			{Action: "download_file", Desc: "把运行中已就绪的下载文件保存到本机，校验大小和 SHA-256 后返回路径。",
				Props: map[string]string{"session_id": "运行 ID", "download_id": "list_downloads 返回的条目 id"}, Required: []string{"session_id", "download_id"},
				Build: func(a map[string]string) (apiCall, error) {
					s, e := pathID(a, "session_id")
					if e != nil {
						return apiCall{}, e
					}
					d, e := pathID(a, "download_id")
					return apiCall{Method: "GET", Path: "/browser/sessions/" + s + "/downloads/" + d + "/content", SaveAs: "*" + d}, e
				}},
			{Action: "list_profiles", Desc: "列出保存登录状态的档案（只读，不计费）。", Props: pages,
				Build: func(a map[string]string) (apiCall, error) {
					c, e := pageArgs(a, "status", "query")
					c.Path = "/browser/profiles"
					return c, e
				}},
			{Action: "get_profile", Desc: "查询档案详情（只读）。", Props: pid, Required: []string{"profile_id"},
				Build: func(a map[string]string) (apiCall, error) {
					return browserID(a, "profile_id", "/browser/profiles/", "", "GET")
				}},
			{Action: "create_profile", Desc: "新建一个档案用于跨运行保存登录状态（只写元数据，不启动浏览器）。",
				Props: map[string]string{"name": "档案名称"}, Required: []string{"name"},
				Build: func(a map[string]string) (apiCall, error) {
					n, e := textArg(a, "name", 100)
					return apiCall{Method: "POST", Path: "/browser/profiles", Body: map[string]any{"name": n}}, e
				}},
			{Action: "delete_profile", Desc: "删除档案及其保存的登录状态（不可恢复，需要确认）。", Approval: true,
				Props: pid, Required: []string{"profile_id"},
				Build: func(a map[string]string) (apiCall, error) {
					return browserID(a, "profile_id", "/browser/profiles/", "", "DELETE")
				}},
			{Action: "pricing", Desc: "查询云浏览器价格与计时规则（只读，不计费）。", Props: map[string]string{},
				Build: func(map[string]string) (apiCall, error) { return apiCall{Method: "GET", Path: "/pricing"}, nil }},
			{Action: "usage", Desc: "分页查询已结束运行的时长与费用记录。", Props: map[string]string{"page": "页码", "page_size": "每页数量 1–100"},
				Build: func(a map[string]string) (apiCall, error) {
					c, e := pageArgs(a)
					c.Path = "/browser/usage"
					return c, e
				}},
		},
	})
}
