package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/manifest"
)

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func TestApplyPreservesFileModes(t *testing.T) {
	s := setupStore(t)
	addSkillToStore(t, s, "tools", "# Tools")
	script := filepath.Join(s.SkillsDir(), "tools", "scripts", "run")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	addResourceToStore(t, s, "bin", map[string]string{"scripts/setup.sh": "#!/bin/sh\n"})
	if err := os.Chmod(filepath.Join(s.ResourcesDir(), "bin", "scripts", "setup.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	repo := t.TempDir()
	m := &manifest.Manifest{
		Layout:    "pi",
		Skills:    []string{"tools"},
		Resources: []string{"bin"},
	}
	stores, defaultStore := singleStoreMap(s)

	// A previous deploy left the script without the executable bit; a forced
	// re-apply must restore the store's mode.
	stale := filepath.Join(repo, ".agents", "skills", "tools", "scripts", "run")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(stores, defaultStore, m, repo, Options{Force: true}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	cases := map[string]os.FileMode{
		filepath.Join(repo, ".agents", "skills", "tools", "scripts", "run"): 0o755,
		filepath.Join(repo, ".agents", "skills", "tools", "SKILL.md"):       0o644,
		filepath.Join(repo, "scripts", "setup.sh"):                          0o755,
	}
	for path, want := range cases {
		if got := fileMode(t, path); got != want {
			t.Errorf("%s: mode = %o, want %o", path, got, want)
		}
	}
}
