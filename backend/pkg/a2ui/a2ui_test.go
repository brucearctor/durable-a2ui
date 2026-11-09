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

	raw := `[{"version":"v0.9","createSurface":{"surfaceId":"test","catalogId":"https://a2ui.org/specification/v0_9/catalogs/basic/catalog.json"}},{"version":"v0.9","updateComponents":{"surfaceId":"test","components":[{"id":"root","component":"Text","text":"Hello A2UI"}]}}]`
	msgs, err := a2ui.ParseAndValidate(raw)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	_, err = a2ui.ParseAndValidate(`[{"version":"v0.9","updateComponents":{"surfaceId":"test","components":[{"id":"not_root","component":"Text"}]}}]`)
	if err == nil {
		t.Fatal("expected error when root component is missing")
	}
}
