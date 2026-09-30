package handles

import (
	"testing"

	"github.com/gin-gonic/gin"
	"pkuphysu-backend/internal/model"
)

func TestLatestCommentPayloadsQuote(t *testing.T) {
	quoteID := uint(9)

	newQuote := func(status string, ownerID uint) *model.ForumComment {
		return &model.ForumComment{
			ID:          quoteID,
			ContentHTML: "<p>被引用的评论</p>",
			Status:      status,
			UserID:      ownerID,
			User:        &model.User{ID: ownerID, Username: "Alice"},
		}
	}

	newComment := func(quote *model.ForumComment) model.ForumComment {
		return model.ForumComment{
			ID:          11,
			PostID:      1,
			ContentHTML: "<p>你说得对</p>",
			Status:      model.ForumContentStatusApproved,
			UserID:      8,
			User:        &model.User{ID: 8, Username: "Bob"},
			Quote:       quote,
		}
	}

	t.Run("可见的引用带上 cid 与用户名", func(t *testing.T) {
		items := latestCommentPayloads(
			[]model.ForumComment{newComment(newQuote(model.ForumContentStatusApproved, 7))},
			map[uint]bool{11: true},
			8,
		)

		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		quote, ok := items[0]["quote"].(gin.H)
		if !ok || quote == nil {
			t.Fatal("expected quote payload")
		}
		if quote["cid"] != quoteID || quote["username"] != "Alice" {
			t.Fatalf("unexpected quote payload: %v", quote)
		}
		if items[0]["is_like"] != 1 {
			t.Fatalf("expected is_like=1, got %v", items[0]["is_like"])
		}
	})

	t.Run("他人不可见的引用返回 nil（序列化为 null）", func(t *testing.T) {
		items := latestCommentPayloads(
			[]model.ForumComment{newComment(newQuote(model.ForumContentStatusPending, 7))},
			map[uint]bool{},
			8,
		)

		quote, ok := items[0]["quote"].(gin.H)
		if !ok {
			t.Fatal("expected quote key to exist")
		}
		if quote != nil {
			t.Fatalf("expected nil quote for invisible reference, got %v", quote)
		}
		if items[0]["is_like"] != 0 {
			t.Fatalf("expected is_like=0, got %v", items[0]["is_like"])
		}
	})

	t.Run("没有引用的评论 quote 为 nil", func(t *testing.T) {
		items := latestCommentPayloads([]model.ForumComment{newComment(nil)}, map[uint]bool{}, 8)

		quote, ok := items[0]["quote"].(gin.H)
		if !ok || quote != nil {
			t.Fatalf("expected nil quote, got %v", items[0]["quote"])
		}
	})
}
