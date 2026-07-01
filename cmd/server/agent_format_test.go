package main

import "testing"

func TestFormatCNYText(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{name: "yuan", value: 1234, want: "1234.00元"},
		{name: "wan", value: 12345678, want: "1234.57万元"},
		{name: "yi", value: 1100208768, want: "11.00亿元"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCNYText(tt.value); got != tt.want {
				t.Fatalf("formatCNYText(%f) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestFormatShareText(t *testing.T) {
	if got := formatShareText(229000000); got != "2.29亿股" {
		t.Fatalf("formatShareText returned %q", got)
	}
}
