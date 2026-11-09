package a2ui

import (
	"encoding/json"
	"fmt"
)

const (
	Version        = "v0.9"
	BasicCatalogID = "https://a2ui.org/specification/v0_9/catalogs/basic/catalog.json"
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

func UpdateComponents(surfaceID string, components []map[string]any) map[string]any {
	return map[string]any{
		"version": Version,
		"updateComponents": map[string]any{
			"surfaceId":  surfaceID,
			"components": components,
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
	return map[string]any{"path": jsonPointer}
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
				return nil, fmt.Errorf("Message %d: must be a JSON object", i)
			}
			list = append(list, m)
		}
	default:
		return nil, fmt.Errorf("A2UI payload must be a JSON object or array of objects")
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("A2UI message list cannot be empty")
	}

	for i, msg := range list {
		if v, ok := msg["version"].(string); !ok || v != Version {
			return nil, fmt.Errorf("Message %d: missing or invalid 'version' (must be %q)", i, Version)
		}
		hasCreate := msg["createSurface"] != nil
		hasUpdateComp := msg["updateComponents"] != nil
		hasUpdateData := msg["updateDataModel"] != nil
		hasDelete := msg["deleteSurface"] != nil
		if !hasCreate && !hasUpdateComp && !hasUpdateData && !hasDelete {
			return nil, fmt.Errorf("Message %d: must contain createSurface, updateComponents, updateDataModel, or deleteSurface", i)
		}

		if uc, ok := msg["updateComponents"].(map[string]any); ok {
			compsRaw, ok := uc["components"].([]any)
			if !ok || len(compsRaw) == 0 {
				return nil, fmt.Errorf("Message %d: updateComponents requires non-empty 'components' array", i)
			}
			hasRoot := false
			for _, c := range compsRaw {
				if cm, ok := c.(map[string]any); ok && cm["id"] == "root" {
					hasRoot = true
				}
			}
			if !hasRoot {
				return nil, fmt.Errorf("Message %d: updateComponents must include a component with id='root'", i)
			}
		}
	}

	return list, nil
}
