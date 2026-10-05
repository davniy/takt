package pi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPiPathCanonicalizationHelper(t *testing.T) {
	path := os.Getenv("TAKT_TEST_CANONICAL_PATH")
	if path == "" {
		return
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(resolved)
	os.Exit(0)
}

func TestPiWorkspaceGuardAllowsAncestorMetadataForCanonicalization(t *testing.T) {
	runPathMetadataGuard(t, true)
}

func TestPiWorkspaceGuardAncestorMetadataKeepsSiblingContentDenied(t *testing.T) {
	runPathMetadataGuard(t, false)
}

func runPathMetadataGuard(t *testing.T, canonicalize bool) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("native macOS sandbox required")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node required")
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, ".runs", "candidate")
	if err := os.MkdirAll(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	guard, cleanup, err := installWorkspaceGuard()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	command := fmt.Sprintf("cat %q", outside)
	condition := "result.details.exitCode === 0"
	if canonicalize {
		helper, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command = fmt.Sprintf("%q -test.run=^TestPiPathCanonicalizationHelper$", helper)
		condition = "result.details.exitCode !== 0"
	}
	script := filepath.Join(workspace, "check.mjs")
	source := fmt.Sprintf(`import guard from %q
let bash
guard({on() {}, registerTool(tool) { bash = tool }})
const result = await bash.execute("metadata", {command: %q}, undefined, undefined, {cwd: process.env.TAKT_WORKSPACE})
if (%s) { console.error(JSON.stringify(result)); process.exit(1) }
`, filepath.ToSlash(guard), command, condition)
	if err := os.WriteFile(script, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, script)
	cmd.Env = append(os.Environ(), "TAKT_WORKSPACE="+workspace, "TAKT_ARTIFACTS_DIR=", "TAKT_TEST_CANONICAL_PATH="+workspace)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("guard boundary failed: %v\n%s", err, output)
	}
}
