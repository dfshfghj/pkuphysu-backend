package moderation

import (
	"testing"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/model"
)

func TestEvaluateRiskLevel(t *testing.T) {
	tests := []struct {
		name      string
		riskLevel string
		want      string
	}{
		{
			name:      "pass becomes approved",
			riskLevel: "PASS",
			want:      model.ForumContentStatusApproved,
		},
		{
			name:      "non pass becomes manual review",
			riskLevel: "REJECT",
			want:      model.ForumContentStatusManualReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &moderationResponse{}
			resp.ResultList = append(resp.ResultList, struct {
				RiskLevel string `json:"risk_level"`
			}{RiskLevel: tt.riskLevel})

			got, err := evaluateRiskLevel(resp)
			if err != nil {
				t.Fatalf("evaluateRiskLevel() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("evaluateRiskLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvaluateRiskLevelRejectsEmptyResult(t *testing.T) {
	if _, err := evaluateRiskLevel(&moderationResponse{}); err == nil {
		t.Fatal("expected error for empty moderation result")
	}
}

func TestMatchSensitiveWord(t *testing.T) {
	oldConf := config.Conf
	oldMatcher := sensitiveMatcher
	t.Cleanup(func() {
		config.Conf = oldConf
		sensitiveMatcher = oldMatcher
	})

	config.Conf = &config.Config{
		Moderation: config.ModerationConfig{
			SensitiveWords: []string{"违禁词", "Spam"},
		},
	}
	Init()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "rejects configured chinese word",
			input: "这里带有违禁词内容",
			want:  true,
		},
		{
			name:  "rejects configured word case insensitively",
			input: "This contains spam content",
			want:  true,
		},
		{
			name:  "ignores safe content",
			input: "正常讨论内容",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := MatchSensitiveWord(tt.input)
			if got != tt.want {
				t.Fatalf("MatchSensitiveWord() = %v, want %v", got, tt.want)
			}
		})
	}
}
