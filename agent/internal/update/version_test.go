package update

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "0.2.0", "0.2.0", 0},
		{"patch newer", "0.2.0", "0.2.1", -1},
		{"minor newer", "0.2.0", "0.3.0", -1},
		{"major newer", "0.2.0", "1.0.0", -1},
		{"current is newer", "0.3.0", "0.2.0", 1},
		{"double digit minor", "0.9.0", "0.10.0", -1},
		{"double digit patch", "0.2.9", "0.2.10", -1},
		{"missing patch treated as zero", "0.2", "0.2.0", 0},
		{"three vs two segments", "1.0.0", "1.0", 0},
		{"single segment", "1", "2", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareVersions(tt.a, tt.b)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareVersions_Invalid(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"empty string", "", "0.2.0"},
		{"non-numeric", "abc", "0.2.0"},
		{"partial non-numeric", "0.2.x", "0.2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compareVersions(tt.a, tt.b)
			if err == nil {
				t.Errorf("expected error for compareVersions(%q, %q)", tt.a, tt.b)
			}
		})
	}
}

func TestStripVPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v0.2.0", "0.2.0"},
		{"V0.2.0", "0.2.0"},
		{"0.2.0", "0.2.0"},
		{"v1.0.0", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripVPrefix(tt.input)
			if got != tt.want {
				t.Errorf("stripVPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
