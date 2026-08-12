package codec

import (
	"testing"
	"unicode"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"len7", 7},
		{"len10", 10},
		{"len1", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := Generate(tt.length)
			if err != nil {
				t.Fatalf("Generate(%d) error: %v", tt.length, err)
			}
			if len(code) != tt.length {
				t.Errorf("got len %d, want %d", len(code), tt.length)
			}
			for _, c := range code {
				if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
					t.Errorf("non-base62 char %q in %q", c, code)
				}
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abc123", true},
		{"ABC", true},
		{"abc-def", false},
		{"", false},
		{"abc def", false},
		{"xYz9K2m", true},
	}
	for _, tt := range tests {
		got := IsValid(tt.input)
		if got != tt.want {
			t.Errorf("IsValid(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGenerateUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		code, err := Generate(7)
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := seen[code]; dup {
			t.Errorf("collision detected: %q", code)
		}
		seen[code] = struct{}{}
	}
}
