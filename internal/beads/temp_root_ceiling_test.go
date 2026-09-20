package beads

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setupTempRootFixture points TMPDIR at a symlink to a fresh root that holds a
// stray .beads (the GH#6603 contamination) and returns the physical root plus
// the two nested store dirs, neither of which is inside a git repo.
func setupTempRootFixture(t *testing.T) (root, storeA, storeB string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR override is not honored by os.TempDir() on Windows")
	}
	t.Setenv("BEADS_DIR", "")
	root = t.TempDir()
	link := filepath.Join(t.TempDir(), "tmproot-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("TMPDIR", link)

	if err := os.MkdirAll(filepath.Join(root, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".beads", "metadata.json"), []byte(`{"backend":"dolt"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	storeA = filepath.Join(root, "store-a")
	storeB = filepath.Join(root, "store-b")
	for _, d := range []string{storeA, storeB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root, storeA, storeB
}

// TestFindBeadsDir_StopsAtOSTempRoot guards GH#6603: a .beads at os.TempDir()
// itself must not be adopted by targets nested beneath it.
func TestFindBeadsDir_StopsAtOSTempRoot(t *testing.T) {
	_, storeA, storeB := setupTempRootFixture(t)
	for _, dir := range []string{storeA, storeB} {
		t.Chdir(dir)
		if got := FindBeadsDir(); got != "" {
			t.Errorf("FindBeadsDir() from %s = %q, want no adoption of the OS temp root's .beads", dir, got)
		}
		if got := FindBeadsDirFrom(dir); got != "" {
			t.Errorf("FindBeadsDirFrom(%s) = %q, want no adoption of the OS temp root's .beads", dir, got)
		}
	}
}

// TestFindBeadsDir_OSTempRootAsStartStillResolves keeps an explicit start at
// the temp root working: only the ancestor ascent is capped.
func TestFindBeadsDir_OSTempRootAsStartStillResolves(t *testing.T) {
	root, _, _ := setupTempRootFixture(t)
	want, _ := filepath.EvalSymlinks(filepath.Join(root, ".beads"))
	t.Chdir(root)
	if got, _ := filepath.EvalSymlinks(FindBeadsDir()); got != want {
		t.Errorf("FindBeadsDir() at temp root = %q, want %q", got, want)
	}
	if got, _ := filepath.EvalSymlinks(FindBeadsDirFrom(root)); got != want {
		t.Errorf("FindBeadsDirFrom(temp root) = %q, want %q", got, want)
	}
}
