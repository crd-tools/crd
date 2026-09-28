package registry

import (
	"sort"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Walk обходит схему и все вложенные схемы в детерминированном порядке
// Если fn возвращает false, вложенные схемы текущего узла не обходятся
func Walk(s *apiextv1.JSONSchemaProps, fn func(*apiextv1.JSONSchemaProps) bool) {
	if s == nil || !fn(s) {
		return
	}
	walkMap(s.Properties, fn)
	walkMap(s.PatternProperties, fn)
	walkMap(s.Definitions, fn)
	if s.Items != nil {
		Walk(s.Items.Schema, fn)
		walkSlice(s.Items.JSONSchemas, fn)
	}
	walkSlice(s.AllOf, fn)
	walkSlice(s.OneOf, fn)
	walkSlice(s.AnyOf, fn)
	Walk(s.Not, fn)
	if s.AdditionalProperties != nil {
		Walk(s.AdditionalProperties.Schema, fn)
	}
	if s.AdditionalItems != nil {
		Walk(s.AdditionalItems.Schema, fn)
	}
	if len(s.Dependencies) > 0 {
		keys := make([]string, 0, len(s.Dependencies))
		for k := range s.Dependencies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			dep := s.Dependencies[k]
			if dep.Schema != nil {
				Walk(dep.Schema, fn)
			}
		}
	}
}

func walkMap(m map[string]apiextv1.JSONSchemaProps, fn func(*apiextv1.JSONSchemaProps) bool) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		Walk(&v, fn)
		m[k] = v
	}
}

func walkSlice(items []apiextv1.JSONSchemaProps, fn func(*apiextv1.JSONSchemaProps) bool) {
	for i := range items {
		Walk(&items[i], fn)
	}
}

// Refs возвращает отсортированный список уникальных ссылок в схеме
func Refs(s *apiextv1.JSONSchemaProps) []string {
	set := map[string]struct{}{}
	Walk(s, func(node *apiextv1.JSONSchemaProps) bool {
		if node.Ref != nil && *node.Ref != "" {
			set[*node.Ref] = struct{}{}
		}
		return true
	})
	out := make([]string, 0, len(set))
	for ref := range set {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}
