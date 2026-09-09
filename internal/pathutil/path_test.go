package pathutil

import "testing"

func TestIsSafeRelative(t *testing.T) {
	tests := map[string]bool{
		"source":           true,
		"group/app":        true,
		`group\app`:        true,
		".":                true,
		"../outside":       false,
		`..\outside`:       false,
		"/absolute":        false,
		`\absolute`:        false,
		`C:\absolute`:      false,
		`C:drive-relative`: false,
		`\\server\share`:   false,
		"":                 false,
	}

	for value, expected := range tests {
		t.Run(value, func(t *testing.T) {
			if actual := IsSafeRelative(value); actual != expected {
				t.Fatalf("IsSafeRelative(%q) = %v, want %v", value, actual, expected)
			}
		})
	}
}
