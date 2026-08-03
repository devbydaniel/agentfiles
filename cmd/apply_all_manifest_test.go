package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devbydaniel/agentfiles/internal/registry"
)

// writeManifest must produce byte-identical output regardless of the host's
// global default_store. Before this was fixed, "store" was emitted only when
// it differed from defaultStore, so a repo deployed from a laptop
// (default_store = "work") and from a container (default_store = "private")
// got two different manifests and each host dirtied the other's tree.
func TestWriteManifestIsIndependentOfDefaultStore(t *testing.T) {
	repo := registry.Repo{
		Path:      t.TempDir(),
		Store:     "private",
		Bundle:    "assistant",
		Layout:    "claude",
		SkillsAdd: []string{"file-document", "work:manage-linear"},
	}

	read := func(defaultStore string) string {
		t.Helper()
		if err := writeManifest(repo, defaultStore); err != nil {
			t.Fatalf("writeManifest(%q): %v", defaultStore, err)
		}
		b, err := os.ReadFile(filepath.Join(repo.Path, ".agentfiles"))
		if err != nil {
			t.Fatalf("reading manifest: %v", err)
		}
		return string(b)
	}

	// "private" matches repo.Store, "work" does not — both must agree.
	matching := read("private")
	differing := read("work")

	if matching != differing {
		t.Errorf("manifest differs by host default_store:\ndefault=private:\n%s\ndefault=work:\n%s", matching, differing)
	}

	want := "store = \"private\"\nbundle = \"assistant\"\nlayout = \"claude\"\nskills_add = [\"file-document\", \"work:manage-linear\"]\n"
	if matching != want {
		t.Errorf("unexpected manifest:\ngot:\n%s\nwant:\n%s", matching, want)
	}
}

// A registry entry with no explicit store falls back to the host default, and
// that fallback must still be written out rather than left implicit.
func TestWriteManifestFallsBackToDefaultStore(t *testing.T) {
	repo := registry.Repo{
		Path:   t.TempDir(),
		Bundle: "assistant",
		Layout: "claude",
	}

	if err := writeManifest(repo, "private"); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(repo.Path, ".agentfiles"))
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	want := "store = \"private\"\nbundle = \"assistant\"\nlayout = \"claude\"\n"
	if string(b) != want {
		t.Errorf("got:\n%s\nwant:\n%s", b, want)
	}
}
