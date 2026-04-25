package domainlist

import "testing"

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ascii lowercase", "EXAMPLE.COM", "example.com"},
		{"trailing dot", "example.com.", "example.com"},
		{"idn punycode", "пример.рф", "xn--e1afmkfd.xn--p1ai"},
		{"already normalized", "example.com", "example.com"},
		{"mixed case idn", "ПРИМЕР.РФ", "xn--e1afmkfd.xn--p1ai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDomain(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
