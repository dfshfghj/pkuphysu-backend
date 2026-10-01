package model

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// MaxUserProfileContentLength 用户主页自定义区块允许的最大字符数（按 rune 计）
const MaxUserProfileContentLength = 5000

// UserProfile 用户主页的自定义内容
type UserProfile struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	UserID      uint   `json:"user_id" gorm:"uniqueIndex;constraint:OnDelete:CASCADE;"`
	Content     string `json:"content"`      // 用户编辑的原始 Markdown
	ContentHTML string `json:"content_html"` // 渲染并清洗后的 HTML
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateUserProfileContent 校验主页自定义内容，超长时返回可直接展示给用户的错误
func ValidateUserProfileContent(content string) error {
	if utf8.RuneCountInString(content) > MaxUserProfileContentLength {
		return fmt.Errorf("主页内容最多 %d 个字符", MaxUserProfileContentLength)
	}
	return nil
}
