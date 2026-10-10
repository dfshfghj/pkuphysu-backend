package handles

import "testing"

func TestParseMentionUserIDs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []uint
	}{
		{name: "empty", content: "", want: []uint{}},
		{name: "single", content: "[@甲](/u/7)", want: []uint{7}},
		{name: "multiple", content: "[@甲](/u/7) 和 [@乙](/u/9)", want: []uint{7, 9}},
		{name: "keeps order", content: "[@甲](/u/9) [@乙](/u/7)", want: []uint{9, 7}},
		{name: "drops duplicates", content: "[@甲](/u/7) [@甲](/u/7)", want: []uint{7}},
		{name: "drops zero and invalid", content: "[@甲](/u/0) [@乙](/u/abc) [@丙](/u/7)", want: []uint{7}},
		{name: "across lines", content: "[@甲](/u/7)\n[@乙](/u/9)", want: []uint{7, 9}},
		{name: "quote link is not a mention", content: "[#45](/45)", want: []uint{}},
		{name: "plain internal link is not a mention", content: "[看这个](/45)", want: []uint{}},
		{name: "non /u href is not a mention", content: "[@甲](/user/7)", want: []uint{}},
		{name: "username with spaces", content: "[@甲 乙](/u/7)", want: []uint{7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMentionUserIDs(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("parseMentionUserIDs(%q) = %v, want %v", tt.content, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("parseMentionUserIDs(%q) = %v, want %v", tt.content, got, tt.want)
				}
			}
		})
	}
}
