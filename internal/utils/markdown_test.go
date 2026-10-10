package utils

import (
	"strings"
	"testing"
)

func TestMarkdownToHtmlKeepsTaskListCheckbox(t *testing.T) {
	html := MarkdownToHtml("- [ ] todo\n- [x] done\n")

	if !strings.Contains(html, "<input") {
		t.Fatalf("task list checkbox was stripped by sanitizer: %s", html)
	}
	if !strings.Contains(html, `type="checkbox"`) {
		t.Fatalf("checkbox type attribute missing: %s", html)
	}
	if !strings.Contains(html, "disabled") {
		t.Fatalf("checkbox disabled attribute missing: %s", html)
	}
	if !strings.Contains(html, "checked") {
		t.Fatalf("checkbox checked attribute missing: %s", html)
	}
}

func TestMarkdownToHtmlParsesInlineMathStartingWithDigit(t *testing.T) {
	html := MarkdownToHtml("$1$")

	if !strings.Contains(html, "language-math") {
		t.Fatalf("$1$ was not parsed as inline math: %s", html)
	}
	if !strings.Contains(html, "1") {
		t.Fatalf("inline math content missing: %s", html)
	}
}
