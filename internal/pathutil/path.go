package pathutil

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// IsSafeRelative reports whether value is a relative path that cannot escape
// its root. It recognizes both slash styles so validation is consistent on
// Linux and Windows.
func IsSafeRelative(value string) bool {
	if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return false
	}

	normalized := strings.ReplaceAll(value, `\`, "/")
	if path.IsAbs(normalized) || (len(normalized) >= 2 && normalized[1] == ':') {
		return false
	}
	clean := path.Clean(normalized)
	return clean != ".." && !strings.HasPrefix(clean, "../")
}

func ValidName(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || name != strings.TrimRight(name, ". ") ||
		name == "." || name == ".." || filepath.Base(filepath.Clean(name)) != name ||
		strings.ContainsAny(name, `<>:"/\|?*`) {
		return false
	}
	for _, r := range name {
		if r < 32 {
			return false
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return false
	}
	return !(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9')
}

// Native accepts portable Windows or slash-separated relative paths on either host.
func Native(value string) string { return filepath.FromSlash(strings.ReplaceAll(value, `\`, "/")) }

func ValidRelative(value string) bool {
	if !IsSafeRelative(value) {
		return false
	}
	if value == "." {
		return true
	}
	for _, part := range strings.Split(strings.ReplaceAll(value, `\`, "/"), "/") {
		if !ValidName(part) {
			return false
		}
	}
	return true
}

// Within rejects symlink escapes as well as lexical traversal, including new destinations.
func Within(root, target string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	existing := target
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return err
		}
		existing = parent
	}
	real, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes root %q", target, root)
	}
	return nil
}
