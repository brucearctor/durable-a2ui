package a2ui_test

import (
	"testing"

	"github.com/ab/durable-a2ui/backend/pkg/a2ui"
)

func TestA2UIBuildersAndValidation(t *testing.T) {
	cs := a2ui.CreateSurface("main", true)
	if cs["version"] != a2ui.Version {
		t.Fatalf("expected version %s, got %v", a2ui.Version, cs["version"])
	}

	p := a2ui.Path("/title")
	if p["@path"] != "/title" {
		t.Fatalf("expected @path binding '/title', got %v", p)
	}

	raw := `[{"version":"1.0","createSurface":{"surfaceId":"test","catalogId":"https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"}},{"version":"1.0","updateComponents":{"surfaceId":"test","components":[{"id":"root","component":"Text","text":"Hello A2UI v1.0"}]}}]`
	msgs, err := a2ui.ParseAndValidate(raw)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	_, err = a2ui.ParseAndValidate(`[{"version":"1.0","updateComponents":{"surfaceId":"test","components":[{"id":"not_root","component":"Text"}]}}]`)
	if err == nil {
		t.Fatal("expected error when root component is missing")
	}
}
