package cmd

import "testing"

func TestParseBoolFlag(t *testing.T) {
	cases := []struct {
		in      string
		want    *bool
		wantErr bool
	}{
		{"", nil, false},
		{"true", boolPtr(true), false},
		{"false", boolPtr(false), false},
		{"yes", nil, true},
	}
	for _, tc := range cases {
		got, err := parseBoolFlag(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseBoolFlag(%q): expected error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseBoolFlag(%q): unexpected error %v", tc.in, err)
			continue
		}
		if (got == nil) != (tc.want == nil) {
			t.Errorf("parseBoolFlag(%q): nil mismatch got %v want %v", tc.in, got, tc.want)
			continue
		}
		if got != nil && *got != *tc.want {
			t.Errorf("parseBoolFlag(%q): got %v want %v", tc.in, *got, *tc.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
