package handles

import (
	"testing"

	"pkuphysu-backend/internal/config"
)

func TestNormalizeForumReportInput(t *testing.T) {
	tests := []struct {
		name    string
		input   forumReportPayload
		wantErr bool
	}{
		{
			name:    "trim and accept valid payload",
			input:   forumReportPayload{Reason: "  垃圾内容  ", Detail: "  反复刷屏  "},
			wantErr: false,
		},
		{
			name:    "reject empty reason",
			input:   forumReportPayload{Reason: "   ", Detail: "detail"},
			wantErr: true,
		},
		{
			name:    "reject too long reason",
			input:   forumReportPayload{Reason: string(make([]byte, forumReportReasonMaxLen+1))},
			wantErr: true,
		},
		{
			name:    "reject too long detail",
			input:   forumReportPayload{Reason: "spam", Detail: string(make([]byte, forumReportDetailMaxLen+1))},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeForumReportInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Reason == tt.input.Reason || got.Detail == tt.input.Detail {
				t.Fatal("expected trimmed fields")
			}
		})
	}
}

func TestForumReportThreshold(t *testing.T) {
	original := config.Conf
	defer func() { config.Conf = original }()

	config.Conf = nil
	if got := forumReportThreshold(); got != defaultForumReportThreshold {
		t.Fatalf("expected default threshold %d, got %d", defaultForumReportThreshold, got)
	}

	config.Conf = &config.Config{
		Moderation: config.ModerationConfig{
			ReportThreshold: 7,
		},
	}
	if got := forumReportThreshold(); got != 7 {
		t.Fatalf("expected configured threshold 7, got %d", got)
	}
}
