package utils

import (
	"net/http/httptest"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func TestRespondErrorAllowsNilError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	RespondError(c, 404, "NotFound", nil)

	if recorder.Code != 404 {
		t.Fatalf("RespondError() status = %d, want 404", recorder.Code)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "non positive limit returns empty", input: "abc", maxLen: 0, want: ""},
		{name: "shorter than limit is returned as is", input: "abc", maxLen: 5, want: "abc"},
		{name: "equal to limit is returned as is", input: "abcde", maxLen: 5, want: "abcde"},
		{name: "ascii over limit gets ellipsis", input: "abcdefgh", maxLen: 5, want: "ab..."},
		{name: "limit within ellipsis length", input: "abcdefgh", maxLen: 3, want: "abc"},
		{
			name:   "chinese counted by rune not byte",
			input:  "北大物理学院物理学院物理学院物理学院物理学院物理学院物理学院物理学院",
			maxLen: 40,
			want:   "北大物理学院物理学院物理学院物理学院物理学院物理学院物理学院物理学院",
		},
		{
			name:   "chinese over limit is cut on rune boundary",
			input:  "北大物理学院物理学院物理学院",
			maxLen: 8,
			want:   "北大物理学...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Fatalf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("TruncateString() produced invalid utf-8: %q", got)
			}
		})
	}
}
