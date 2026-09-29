package handles

import (
	"testing"

	"pkuphysu-backend/internal/model"
)

func TestCanViewForumContent(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		ownerID  uint
		viewerID uint
		want     bool
	}{
		{
			name:     "approved content is public",
			status:   model.ForumContentStatusApproved,
			ownerID:  1,
			viewerID: 2,
			want:     true,
		},
		{
			name:     "pending content visible to owner",
			status:   model.ForumContentStatusPending,
			ownerID:  1,
			viewerID: 1,
			want:     true,
		},
		{
			name:     "manual review content hidden from others",
			status:   model.ForumContentStatusManualReview,
			ownerID:  1,
			viewerID: 2,
			want:     false,
		},
		{
			name:     "rejected content hidden from others",
			status:   model.ForumContentStatusRejected,
			ownerID:  1,
			viewerID: 2,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canViewForumContent(tt.status, tt.ownerID, tt.viewerID); got != tt.want {
				t.Fatalf("canViewForumContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotePayloadForViewer(t *testing.T) {
	comment := &model.ForumComment{
		Quote: &model.ForumComment{
			ID:          10,
			ContentHTML: "<p>quoted</p>",
			Status:      model.ForumContentStatusManualReview,
			UserID:      1,
			User: &model.User{
				ID:       1,
				Username: "owner",
			},
		},
	}

	if payload := quotePayloadForViewer(comment, 2); payload != nil {
		t.Fatalf("expected hidden quote payload for non-owner, got %#v", payload)
	}

	payload := quotePayloadForViewer(comment, 1)
	if payload == nil {
		t.Fatal("expected quote payload for owner")
	}
	if payload["status"] != model.ForumContentStatusManualReview {
		t.Fatalf("unexpected quote status: %#v", payload["status"])
	}
}
