package archiveutil

import "testing"

func TestNormalizeArchiveName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{name: "adds extension and replaces spaces", input: "OpenWRT Anubis test3", want: "OpenWRT-Anubis-test3.wacz", wantOK: true},
		{name: "preserves existing extension", input: "archive.wacz", want: "archive.wacz", wantOK: true},
		{name: "preserves uppercase extension", input: "archive.WACZ", want: "archive.WACZ", wantOK: true},
		{name: "drops parent directories", input: "nested/archive name", want: "archive-name.wacz", wantOK: true},
		{name: "rejects empty name", input: "   ", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeArchiveName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("NormalizeArchiveName(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}

			if got != tt.want {
				t.Fatalf("NormalizeArchiveName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
