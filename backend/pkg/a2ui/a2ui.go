package a2ui

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Version        = "1.0"
	BasicCatalogID = "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"
)

func CreateSurface(surfaceID string, sendDataModel bool) map[string]any {
	cs := map[string]any{
		"surfaceId": surfaceID,
		"catalogId": BasicCatalogID,
	}
	if sendDataModel {
		cs["sendDataModel"] = true
	}
	return map[string]any{
		"version":       Version,
		"createSurface": cs,
	}
}

func normalizeComponent(c map[string]any) map[string]any {
	out := make(map[string]any, len(c))
	for k, v := range c {
		if k == "textStyle" {
			continue
		}
		if m, ok := v.(map[string]any); ok {
			if p, hasPath := m["path"].(string); hasPath && m["@path"] == nil && m["componentId"] == nil {
				out[k] = map[string]any{"@path": p}
				continue
			}
		}
		out[k] = v
	}

	if out["component"] == "Text" {
		if variant, ok := out["variant"].(string); ok {
			headingPrefixes := map[string]string{
				"h1": "# ",
				"h2": "## ",
				"h3": "### ",
				"h4": "#### ",
				"h5": "##### ",
			}
			if prefix, isHeading := headingPrefixes[variant]; isHeading {
				if txt, ok := out["text"].(string); ok && !strings.HasPrefix(txt, "#") {
					out["text"] = prefix + txt
				}
				out["variant"] = "body"
			} else if variant != "body" && variant != "caption" {
				out["variant"] = "body"
			}
		}
	}
	return out
}

func UpdateComponents(surfaceID string, components []map[string]any) map[string]any {
	normalized := make([]map[string]any, 0, len(components))
	for _, c := range components {
		normalized = append(normalized, normalizeComponent(c))
	}
	return map[string]any{
		"version": Version,
		"updateComponents": map[string]any{
			"surfaceId":  surfaceID,
			"components": normalized,
		},
	}
}

func UpdateDataModel(surfaceID string, path string, value any) map[string]any {
	udm := map[string]any{
		"surfaceId": surfaceID,
		"value":     value,
	}
	if path != "" {
		udm["path"] = path
	}
	return map[string]any{
		"version":         Version,
		"updateDataModel": udm,
	}
}

func DeleteSurface(surfaceID string) map[string]any {
	return map[string]any{
		"version": Version,
		"deleteSurface": map[string]any{
			"surfaceId": surfaceID,
		},
	}
}

func Path(jsonPointer string) map[string]any {
	return map[string]any{"@path": jsonPointer}
}

func EventAction(name string, context map[string]any) map[string]any {
	ev := map[string]any{"name": name}
	if len(context) > 0 {
		ev["context"] = context
	}
	return map[string]any{"event": ev}
}

func ParseAndValidate(raw string) ([]map[string]any, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var list []map[string]any
	switch v := parsed.(type) {
	case map[string]any:
		list = []map[string]any{v}
	case []any:
		for i, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("message %d: must be a JSON object", i)
			}
			list = append(list, m)
		}
	default:
		return nil, fmt.Errorf("a2ui payload must be a JSON object or array of objects")
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("a2ui message list cannot be empty")
	}

	for i, msg := range list {
		v, ok := msg["version"].(string)
		if !ok || (v != Version && v != "v1.0" && v != "v0.9") {
			return nil, fmt.Errorf("message %d: missing or invalid 'version' (must be %q)", i, Version)
		}
		msg["version"] = Version

		hasCreate := msg["createSurface"] != nil
		hasUpdateComp := msg["updateComponents"] != nil
		hasUpdateData := msg["updateDataModel"] != nil
		hasDelete := msg["deleteSurface"] != nil
		if !hasCreate && !hasUpdateComp && !hasUpdateData && !hasDelete {
			return nil, fmt.Errorf("message %d: must contain createSurface, updateComponents, updateDataModel, or deleteSurface", i)
		}

		if cs, ok := msg["createSurface"].(map[string]any); ok {
			if catID, ok := cs["catalogId"].(string); !ok || catID == "" || strings.Contains(catID, "v0_9") {
				cs["catalogId"] = BasicCatalogID
			}
		}

		if uc, ok := msg["updateComponents"].(map[string]any); ok {
			compsRaw, ok := uc["components"].([]any)
			if !ok || len(compsRaw) == 0 {
				return nil, fmt.Errorf("message %d: updateComponents requires non-empty 'components' array", i)
			}
			hasRoot := false
			normComps := make([]any, 0, len(compsRaw))
			for _, c := range compsRaw {
				if cm, ok := c.(map[string]any); ok {
					if cm["id"] == "root" {
						hasRoot = true
					}
					normComps = append(normComps, normalizeComponent(cm))
				} else {
					normComps = append(normComps, c)
				}
			}
			if !hasRoot {
				return nil, fmt.Errorf("message %d: updateComponents must include a component with id='root'", i)
			}
			uc["components"] = normComps
		}
	}

	return list, nil
}
