package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// RepoRoot returns the repository root (directory containing go.mod and infra/migrations).
func RepoRoot(t testing.TB) string {
	t.Helper()
	root, err := FindRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// FindRepoRoot locates the repository root from the current working directory.
func FindRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "infra", "migrations")); err == nil {
				return dir, nil
			}
		}
		if dir == filepath.Dir(dir) {
			return "", fmt.Errorf("repository root not found (expected go.mod and infra/migrations)")
		}
	}
}
