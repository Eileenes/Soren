package connectors

import "sort"

func actionKey(b binding) string {
	if b.Connection.ID == "" {
		return b.Action
	}
	return b.Connection.Kind + "." + b.Action
}

type ActionOption struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Approval bool   `json:"approval"`
}

func (s *Service) Actions(bot string) ([]ActionOption, error) {
	catalog, e := s.catalog(bot)
	if e != nil {
		return nil, e
	}
	out := []ActionOption{}
	seen := map[string]bool{}
	for _, b := range catalog {
		key := actionKey(b)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ActionOption{key, b.Tool.Function.Description, b.Approval})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
