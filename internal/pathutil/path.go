package pathutil

import (
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
