package connectors

func designGet(path string, q ...string) func(map[string]string) (apiCall, error) {
	return func(a map[string]string) (apiCall, error) {
		return apiCall{Method: "GET", Path: path, Query: query(a, q...)}, nil
	}
}

func designByID(prefix, suffix, saveExt string) func(map[string]string) (apiCall, error) {
	return func(a map[string]string) (apiCall, error) {
		id, e := pathID(a, "id")
		c := apiCall{Method: "GET", Path: prefix + id + suffix}
		if saveExt != "" {
			c.SaveAs = "template-" + id + saveExt
		}
		return c, e
	}
}

func init() {
	id := func(d string) map[string]string { return map[string]string{"id": d} }
	registerKind(&kindSpec{
		Kind: "design", Title: "设计提示词", Base: "https://design-prompt.app.baizhi.cloud/openapi/v1", Check: "/design-directions",
		Tools: []kindTool{
			{Action: "search_templates", Desc: "搜索已发布的设计模板（分页）。",
				Props: map[string]string{"q": "自然语言搜索词", "page": "页码", "page_size": "每页数量，最大 100", "mode": "模板类型", "category": "分类", "platform": "平台", "scenario": "场景"},
				Build: func(a map[string]string) (apiCall, error) {
					if _, _, e := intArg(a, "page_size", 1, 100); e != nil {
						return apiCall{}, e
					}
					if _, _, e := intArg(a, "page", 1, 10000); e != nil {
						return apiCall{}, e
					}
					return designGet("/catalog/templates", "q", "page", "page_size", "mode", "category", "platform", "scenario")(a)
				}},
			{Action: "get_template", Desc: "获取设计模板详情，含完整中英文提示词 localizedPrompt。", Props: id("模板 ID"), Required: []string{"id"},
				Build: designByID("/catalog/templates/", "", "")},
			{Action: "download_template", Desc: "下载设计模板 ZIP 包到本机并返回文件路径。", Props: id("模板 ID"), Required: []string{"id"},
				Build: designByID("/catalog/templates/", "/download", ".zip")},
			{Action: "search_visual_styles", Desc: "按使用场景和关键词搜索视觉风格（最多 5 条）。",
				Props: map[string]string{"context": "使用场景：deck、document、image、prototype 或 video", "q": "风格名称或关键词", "category": "可选分类", "variant": "可选变体", "recommended": "可选：是否只看推荐", "count": "返回数量，最多 5"}, Required: []string{"context"},
				Build: func(a map[string]string) (apiCall, error) {
					if _, e := oneOf(a, "context", "deck", "document", "image", "prototype", "video"); e != nil {
						return apiCall{}, e
					}
					if _, _, e := intArg(a, "count", 1, 5); e != nil {
						return apiCall{}, e
					}
					return designGet("/visual-styles", "context", "q", "category", "variant", "recommended", "count")(a)
				}},
			{Action: "get_visual_style", Desc: "按 ID 获取完整视觉风格定义。", Props: id("视觉风格 ID"), Required: []string{"id"},
				Build: designByID("/visual-styles/", "", "")},
			{Action: "search_directions", Desc: "列出或按情绪、参考、方向名称搜索方向基线（色板、字体、姿态）。", Props: map[string]string{"q": "可选关键词"},
				Build: designGet("/design-directions", "q")},
			{Action: "search_design_systems", Desc: "按品牌或设计特征搜索已有设计系统。", Props: map[string]string{"q": "品牌名称或设计特征", "count": "返回数量，最多 20"},
				Build: func(a map[string]string) (apiCall, error) {
					if _, _, e := intArg(a, "count", 1, 20); e != nil {
						return apiCall{}, e
					}
					return designGet("/design-systems", "q", "count")(a)
				}},
			{Action: "get_design_system", Desc: "按 ID 获取完整 DESIGN.md 设计规范（如 linear.app）。", Props: id("设计系统 ID"), Required: []string{"id"},
				Build: designByID("/design-systems/", "", "")},
		},
	})
}
