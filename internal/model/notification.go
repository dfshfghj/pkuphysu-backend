package model

import (
	"time"
)

type Notification struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	UserID           uint      `json:"user_id" gorm:"index"`             // 接收通知的用户ID
	Title            string    `json:"title"`                            // 通知标题（可由前端拼接）
	Content          string    `json:"content"`                          // 通知内容（可由前端拼接）
	Type             string    `json:"type"`                             // 通知类型（如：forum_comment, forum_delete等）
	Read             bool      `json:"read" gorm:"default:false"`        // 是否已读
	CreatedAt        time.Time `json:"created_at" gorm:"autoCreateTime"` // 创建时间
	RelatedUserID    uint      `json:"related_user_id"`                  // 关联用户ID（发起通知的用户）
	RelatedPostID    uint      `json:"related_post_id"`                  // 关联帖子ID
	RelatedCommentID uint      `json:"related_comment_id"`               // 关联评论ID
}
