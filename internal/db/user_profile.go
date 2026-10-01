package db

import (
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetUserProfile 获取用户主页自定义内容；用户从未设置过时返回 nil, nil
func GetUserProfile(userID uint) (*model.UserProfile, error) {
	var profile model.UserProfile
	err := db.Where("user_id = ?", userID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get user profile")
	}
	return &profile, nil
}

// SaveUserProfile 保存用户主页自定义内容，不存在则创建，已存在则更新
func SaveUserProfile(profile *model.UserProfile) error {
	profile.ContentHTML = utils.MarkdownToHtml(profile.Content)
	return errors.WithStack(db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"content", "content_html", "updated_at"}),
	}).Create(profile).Error)
}

// GetForumPostsByUserID 获取指定用户发布的帖子，只返回对 viewerID 可见的部分
func GetForumPostsByUserID(userID uint, cursor int, limit int, viewerID uint) ([]model.ForumPost, error) {
	dbQuery := db.Preload("User").Preload("Tags").
		Where("user_id = ?", userID).
		Where(forumPostVisibleToUserQuery(viewerID), viewerID)

	if cursor != 0 {
		dbQuery = dbQuery.Where("id < ?", cursor)
	}

	var posts []model.ForumPost
	err := dbQuery.Order("id DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

// GetUserForumStats 统计用户对 viewerID 可见的帖子数、评论数与获赞总数
func GetUserForumStats(userID uint, viewerID uint) (postCount int64, commentCount int64, likesReceived int64, err error) {
	if err = db.Model(&model.ForumPost{}).
		Where("user_id = ?", userID).
		Where(forumPostVisibleToUserQuery(viewerID), viewerID).
		Count(&postCount).Error; err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to count user posts")
	}

	if err = db.Model(&model.ForumComment{}).
		Where("user_id = ?", userID).
		Where(forumCommentVisibleToUserQuery(viewerID), viewerID).
		Count(&commentCount).Error; err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to count user comments")
	}

	if err = db.Model(&model.ForumPost{}).
		Where("user_id = ?", userID).
		Where(forumPostVisibleToUserQuery(viewerID), viewerID).
		Select("COALESCE(SUM(likenum), 0)").
		Scan(&likesReceived).Error; err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to sum user post likes")
	}

	return postCount, commentCount, likesReceived, nil
}
