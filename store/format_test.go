package store

import "testing"

func TestParseOutputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    OutputFormat
		wantErr bool
	}{
		{"", FormatTable, false},
		{"table", FormatTable, false},
		{"TABLE", FormatTable, false},
		{"json", FormatJSON, false},
		{"Yaml", FormatYAML, false},
		{"unsupported", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOutputFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
