package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritePreviews(t *testing.T) {
	dir := os.Getenv("MAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("MAIL_PREVIEW_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range Samples() {
		if err := os.WriteFile(filepath.Join(dir, s.Name+".html"), []byte(s.HTML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
