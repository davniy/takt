package du

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsAllocatedSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{path}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(stdout.String(), "0\t") {
		t.Fatalf("allocated file reported as empty: %q", stdout.String())
	}
}
