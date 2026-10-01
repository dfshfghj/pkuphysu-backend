package handles

import (
	"fmt"
	"strings"
	"testing"
)

func TestParsePostQuoteIDs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []uint
	}{
		{name: "empty", raw: "", want: []uint{}},
		{name: "single id", raw: "45", want: []uint{45}},
		{name: "comma separated", raw: "45,31,23", want: []uint{45, 31, 23}},
		{name: "keeps request order", raw: "3,1,2", want: []uint{3, 1, 2}},
		{name: "trims spaces", raw: " 45 , 31 ", want: []uint{45, 31}},
		{name: "drops duplicates", raw: "45,45,31", want: []uint{45, 31}},
		{name: "drops invalid and zero", raw: "45,abc,0,-3,31", want: []uint{45, 31}},
		{name: "drops empty segments", raw: ",45,,31,", want: []uint{45, 31}},
		{name: "ignores overflow", raw: "99999999999999999999,7", want: []uint{7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePostQuoteIDs(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("parsePostQuoteIDs(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("parsePostQuoteIDs(%q) = %v, want %v", tt.raw, got, tt.want)
				}
			}
		})
	}
}

func TestParsePostQuoteIDsCapsCount(t *testing.T) {
	parts := make([]string, 0, maxPostQuoteIDs*2)
	for i := 1; i <= maxPostQuoteIDs*2; i++ {
		parts = append(parts, fmt.Sprintf("%d", i))
	}

	got := parsePostQuoteIDs(strings.Join(parts, ","))
	if len(got) != maxPostQuoteIDs {
		t.Fatalf("parsePostQuoteIDs() returned %d ids, want at most %d", len(got), maxPostQuoteIDs)
	}
	if got[0] != 1 || got[len(got)-1] != uint(maxPostQuoteIDs) {
		t.Fatalf("parsePostQuoteIDs() kept unexpected ids: first=%d last=%d", got[0], got[len(got)-1])
	}
}
