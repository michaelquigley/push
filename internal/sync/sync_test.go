package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestParseManifestLine(t *testing.T) {
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	tests := []struct {
		name     string
		line     string
		wantPath string
		wantHash string
	}{
		{"new format", hash + "  bin/tool", "bin/tool", hash},
		{"legacy format", "bin/tool", "bin/tool", ""},
		{"short hex prefix", "abcdef  bin/tool", "bin/tool", ""},
		{"non-hex prefix", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  bin/tool", "", ""},
		{"exactly 64 chars no separator", hash + "x", hash + "x", ""},
		{"single space separator", hash + " bin/tool", hash + " bin/tool", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseManifestLine(tt.line)
			if tt.wantHash != "" {
				// new format — check both fields
				if got.Hash != tt.wantHash {
					t.Errorf("hash = %q, want %q", got.Hash, tt.wantHash)
				}
				if got.Path != filepath.FromSlash(tt.wantPath) {
					t.Errorf("path = %q, want %q", got.Path, filepath.FromSlash(tt.wantPath))
				}
			} else {
				// legacy — hash should be empty, path is the whole line
				if got.Hash != "" {
					t.Errorf("hash = %q, want empty", got.Hash)
				}
				if got.Path != filepath.FromSlash(tt.line) {
					t.Errorf("path = %q, want %q", got.Path, filepath.FromSlash(tt.line))
				}
			}
		})
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("known content", func(t *testing.T) {
		content := []byte("hello world\n")
		path := filepath.Join(dir, "hello.txt")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}
		got, err := hashFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := sha256Hex(content)
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := hashFile(filepath.Join(dir, "nonexistent"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestCopyFileReturnsHash(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	content := []byte("binary content here")
	srcPath := filepath.Join(srcDir, "tool")
	if err := os.WriteFile(srcPath, content, 0755); err != nil {
		t.Fatal(err)
	}

	dstPath := filepath.Join(dstDir, "tool")
	hash, err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatal(err)
	}

	// check hash correctness
	want := sha256Hex(content)
	if hash != want {
		t.Errorf("hash = %s, want %s", hash, want)
	}

	// check file content
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch")
	}

	// check permissions (skip on Windows)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(dstPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("perm = %o, want 0755", info.Mode().Perm())
		}
	}
}

func TestCopyTree(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// create source tree
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"alpha":    []byte("alpha content"),
		"sub/beta": []byte("beta content"),
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, filepath.FromSlash(rel)), content, 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := copyTree(srcDir, dstDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != len(files) {
		t.Fatalf("got %d entries, want %d", len(entries), len(files))
	}

	entryMap := make(map[string]string)
	for _, e := range entries {
		entryMap[e.Path] = e.Hash
	}

	for rel, content := range files {
		hash, ok := entryMap[rel]
		if !ok {
			t.Errorf("missing entry for %s", rel)
			continue
		}
		want := sha256Hex(content)
		if hash != want {
			t.Errorf("%s: hash = %s, want %s", rel, hash, want)
		}
		// verify destination file exists
		dst := filepath.Join(dstDir, filepath.FromSlash(rel))
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if string(got) != string(content) {
			t.Errorf("%s: content mismatch", rel)
		}
	}
}

func TestWriteReadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	name := "test-build"

	entries := []ManifestEntry{
		{Path: "bin/tool", Hash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{Path: "lib/helper.so", Hash: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"},
	}

	if err := writeManifest(dir, name, entries); err != nil {
		t.Fatal(err)
	}

	got := readManifest(dir, name)
	if len(got) != len(entries) {
		t.Fatalf("got %d entries, want %d", len(got), len(entries))
	}
	for i, e := range got {
		if e.Path != filepath.FromSlash(entries[i].Path) {
			t.Errorf("[%d] path = %q, want %q", i, e.Path, entries[i].Path)
		}
		if e.Hash != entries[i].Hash {
			t.Errorf("[%d] hash = %q, want %q", i, e.Hash, entries[i].Hash)
		}
	}
}

func TestWriteReadManifestLegacy(t *testing.T) {
	dir := t.TempDir()
	name := "legacy-build"

	// write legacy format (path-only lines)
	content := "bin/tool\nlib/helper.so\n"
	if err := os.WriteFile(filepath.Join(dir, name+".files"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := readManifest(dir, name)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	for _, e := range got {
		if e.Hash != "" {
			t.Errorf("legacy entry should have empty hash, got %q", e.Hash)
		}
	}
	if got[0].Path != filepath.FromSlash("bin/tool") {
		t.Errorf("path = %q, want %q", got[0].Path, filepath.FromSlash("bin/tool"))
	}
	if got[1].Path != filepath.FromSlash("lib/helper.so") {
		t.Errorf("path = %q, want %q", got[1].Path, filepath.FromSlash("lib/helper.so"))
	}
}

func TestFilesVerified(t *testing.T) {
	t.Run("all match", func(t *testing.T) {
		stateDir := t.TempDir()
		installDir := t.TempDir()
		name := "build"

		content := []byte("hello")
		hash := sha256Hex(content)

		if err := os.WriteFile(filepath.Join(installDir, "tool"), content, 0755); err != nil {
			t.Fatal(err)
		}
		entries := []ManifestEntry{{Path: "tool", Hash: hash}}
		if err := writeManifest(stateDir, name, entries); err != nil {
			t.Fatal(err)
		}

		if !filesVerified(stateDir, name, installDir) {
			t.Error("expected verified = true")
		}
	})

	t.Run("modified file", func(t *testing.T) {
		stateDir := t.TempDir()
		installDir := t.TempDir()
		name := "build"

		original := []byte("original")
		hash := sha256Hex(original)

		// write a different file
		if err := os.WriteFile(filepath.Join(installDir, "tool"), []byte("modified"), 0755); err != nil {
			t.Fatal(err)
		}
		entries := []ManifestEntry{{Path: "tool", Hash: hash}}
		if err := writeManifest(stateDir, name, entries); err != nil {
			t.Fatal(err)
		}

		if filesVerified(stateDir, name, installDir) {
			t.Error("expected verified = false for modified file")
		}
	})

	t.Run("deleted file", func(t *testing.T) {
		stateDir := t.TempDir()
		installDir := t.TempDir()
		name := "build"

		hash := sha256Hex([]byte("content"))
		entries := []ManifestEntry{{Path: "tool", Hash: hash}}
		if err := writeManifest(stateDir, name, entries); err != nil {
			t.Fatal(err)
		}

		if filesVerified(stateDir, name, installDir) {
			t.Error("expected verified = false for deleted file")
		}
	})

	t.Run("legacy manifest", func(t *testing.T) {
		stateDir := t.TempDir()
		installDir := t.TempDir()
		name := "build"

		if err := os.WriteFile(filepath.Join(installDir, "tool"), []byte("x"), 0755); err != nil {
			t.Fatal(err)
		}
		// write legacy manifest (no hashes)
		if err := os.WriteFile(filepath.Join(stateDir, name+".files"), []byte("tool\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if filesVerified(stateDir, name, installDir) {
			t.Error("expected verified = false for legacy manifest")
		}
	})

	t.Run("empty manifest", func(t *testing.T) {
		stateDir := t.TempDir()
		installDir := t.TempDir()
		name := "build"

		// no manifest file at all
		if filesVerified(stateDir, name, installDir) {
			t.Error("expected verified = false for empty manifest")
		}
	})
}

func TestCleanOldFiles(t *testing.T) {
	installDir := t.TempDir()

	// create files
	if err := os.WriteFile(filepath.Join(installDir, "keep"), []byte("k"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "remove"), []byte("r"), 0644); err != nil {
		t.Fatal(err)
	}

	oldEntries := []ManifestEntry{
		{Path: "keep", Hash: "aaa"},
		{Path: "remove", Hash: "bbb"},
	}
	newEntries := []ManifestEntry{
		{Path: "keep", Hash: "aaa"},
	}

	cleanOldFiles(installDir, oldEntries, newEntries)

	if _, err := os.Stat(filepath.Join(installDir, "keep")); err != nil {
		t.Errorf("'keep' should still exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, "remove")); !os.IsNotExist(err) {
		t.Errorf("'remove' should have been deleted")
	}
}
