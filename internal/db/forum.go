package db

import (
	"errors"
	"strings"
	"time"

	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"gorm.io/gorm"
)

func forumPostVisibleToUserQuery(userID uint) string {
	return "(status = '" + model.ForumContentStatusApproved + "' OR user_id = ?)"
}

func forumCommentVisibleToUserQuery(userID uint) string {
	return "(status = '" + model.ForumContentStatusApproved + "' OR user_id = ?)"
}

// GetForumPostByID 根据ID获取单个帖子
func GetForumPostByID(pid int) (*model.ForumPost, error) {
	var post model.ForumPost
	err := db.Preload("User").Preload("Tags").Where("id = ?", pid).First(&post).Error
	return &post, err
}

// GetForumPosts 获取帖子列表
func GetForumPosts(cursor int, limit int, tags []string, keywords []string, userID uint) ([]model.ForumPost, error) {
	dbQuery := db.Preload("User").Preload("Tags").Where(forumPostVisibleToUserQuery(userID), userID)

	if len(tags) > 0 {
		// 处理多个 tag，使用 OR 条件
		var tagConditions []string
		var tagArgs []interface{}

		for _, tag := range tags {
			if tag != "" {
				tagConditions = append(tagConditions, "pkuphysu_forum_tags.name = ?")
				tagArgs = append(tagArgs, tag)
			}
		}

		if len(tagConditions) > 0 {
			dbQuery = dbQuery.Joins("JOIN pkuphysu_forum_post_tags ON pkuphysu_forum_posts.id = pkuphysu_forum_post_tags.forum_post_id").
				Joins("JOIN pkuphysu_forum_tags ON pkuphysu_forum_post_tags.forum_tag_id = pkuphysu_forum_tags.id").
				Where("("+strings.Join(tagConditions, " OR ")+")", tagArgs...)
		}
	}

	// 处理多个 keyword，使用 OR 条件
	if len(keywords) > 0 {
		var keywordConditions []string
		var keywordArgs []interface{}

		for _, keyword := range keywords {
			if keyword != "" {
				keywordConditions = append(keywordConditions, "content_text ILIKE ?")
				keywordArgs = append(keywordArgs, "%"+keyword+"%")
			}
		}

		if len(keywordConditions) > 0 {
			condition := "(" + strings.Join(keywordConditions, " OR ") + ")"
			dbQuery = dbQuery.Where(condition, keywordArgs...)
		}
	}

	if cursor != 0 {
		dbQuery = dbQuery.Where("id < ?", cursor)
	}

	var posts []model.ForumPost
	err := dbQuery.Order("id DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

// GetForumComments 获取评论列表
func GetForumComments(pid string, cursor int, limit int, sort string, userID uint) ([]model.ForumComment, error) {
	dbQuery := db.Preload("User").Preload("Quote").Preload("Quote.User").
		Where("post_id = ?", pid).
		Where(forumCommentVisibleToUserQuery(userID), userID)

	if sort == "desc" {
		dbQuery = dbQuery.Order("id DESC")
	} else {
		dbQuery = dbQuery.Order("id ASC")
	}

	if cursor != 0 {
		if sort == "desc" {
			dbQuery = dbQuery.Where("id < ?", cursor)
		} else {
			dbQuery = dbQuery.Where("id > ?", cursor)
		}
	}

	var comments []model.ForumComment
	err := dbQuery.Limit(limit).Find(&comments).Error
	return comments, err
}

// GetLatestCommentsByPostIDs 返回各帖子最新 limit 条可见评论（按时间正序），供帖子列表预览
func GetLatestCommentsByPostIDs(postIDs []uint, viewerID uint, limit int) (map[uint][]model.ForumComment, error) {
	result := make(map[uint][]model.ForumComment, len(postIDs))

	var visiblePostIDs []uint
	if err := db.Model(&model.ForumPost{}).
		Where("id IN ?", postIDs).
		Where(forumPostVisibleToUserQuery(viewerID), viewerID).
		Pluck("id", &visiblePostIDs).Error; err != nil {
		return nil, err
	}

	for _, postID := range visiblePostIDs {
		comments := make([]model.ForumComment, 0, limit)
		err := db.Preload("User").Preload("Quote.User").
			Where("post_id = ?", postID).
			Where(forumCommentVisibleToUserQuery(viewerID), viewerID).
			Order("id DESC").Limit(limit).
			Find(&comments).Error
		if err != nil {
			return nil, err
		}

		// 取最新的 limit 条后翻转为时间正序，便于前端直接渲染
		for i, j := 0, len(comments)-1; i < j; i, j = i+1, j-1 {
			comments[i], comments[j] = comments[j], comments[i]
		}
		result[postID] = comments
	}
	return result, nil
}

// CreateForumComment 创建评论
func CreateForumComment(comment *model.ForumComment) error {
	comment.ContentHTML = utils.MarkdownToHtml(comment.Content)
	comment.ContentText = utils.MarkdownToText(comment.Content)
	if comment.Status == "" {
		comment.Status = model.ForumContentStatusPending
	}
	return db.Create(comment).Error
}

// CreateForumPost 创建帖子
func CreateForumPost(post *model.ForumPost) error {
	post.ContentHTML = utils.MarkdownToHtml(post.Content)
	post.ContentText = utils.MarkdownToText(post.Content)
	if post.Status == "" {
		post.Status = model.ForumContentStatusPending
	}

	// 处理标签
	if len(post.Tags) > 0 {
		names := make([]string, len(post.Tags))
		for i, tag := range post.Tags {
			names[i] = tag.Name
		}
		tags, err := resolveForumTags(db, names)
		if err != nil {
			return err
		}
		post.Tags = tags
	}

	return db.Create(post).Error
}

// EditForumPost 将帖子当前内容存为历史版本后，更新正文、标签与状态
func EditForumPost(post *model.ForumPost, content string, tagNames []string, status string) error {
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	// 当前内容成为第 EditCount+1 个历史版本，版本时间取该内容生效的时间
	version := model.ForumPostVersion{
		PostID:      post.ID,
		Version:     post.EditCount + 1,
		Content:     post.Content,
		ContentHTML: post.ContentHTML,
		ContentText: post.ContentText,
		CreatedAt:   post.CreatedAt,
	}
	if post.LastEditedAt != nil {
		version.CreatedAt = *post.LastEditedAt
	}
	if err := tx.Create(&version).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&model.ForumPost{}).Where("id = ?", post.ID).Updates(map[string]interface{}{
		"content":        content,
		"content_html":   utils.MarkdownToHtml(content),
		"content_text":   utils.MarkdownToText(content),
		"status":         status,
		"edit_count":     post.EditCount + 1,
		"last_edited_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where(&model.ForumPostTag{PostID: post.ID}).Delete(&model.ForumPostTag{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	tags, err := resolveForumTags(tx, tagNames)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, tag := range tags {
		link := model.ForumPostTag{PostID: post.ID, TagID: tag.ID}
		if err := tx.Create(&link).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetForumPostVersions 获取帖子的历史版本，按版本号升序
func GetForumPostVersions(postID uint) ([]model.ForumPostVersion, error) {
	var versions []model.ForumPostVersion
	err := db.Where("post_id = ?", postID).Order("version ASC").Find(&versions).Error
	return versions, err
}

// resolveForumTags 按名称查找标签，不存在的自动创建
func resolveForumTags(tx *gorm.DB, names []string) ([]model.ForumTag, error) {
	var tags []model.ForumTag
	seen := make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		var tag model.ForumTag
		err := tx.Where("name = ?", name).First(&tag).Error
		if err == nil {
			tags = append(tags, tag)
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		tag = model.ForumTag{Name: name}
		if err := tx.Create(&tag).Error; err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// GetForumCommentByID 根据ID获取单个评论
func GetForumCommentByID(commentID uint) (*model.ForumComment, error) {
	var comment model.ForumComment
	err := db.Preload("User").Where("id = ?", commentID).First(&comment).Error
	return &comment, err
}

func GetFollowedIDs(userID uint, minID uint, maxID uint) ([]uint, error) {
	dbQuery := db.Model(&model.ForumFollow{}).Select("post_id").Where("user_id = ?", userID)

	if minID > 0 {
		dbQuery = dbQuery.Where("post_id >= ?", minID)
	}
	if maxID > 0 {
		dbQuery = dbQuery.Where("post_id <= ?", maxID)
	}

	var postIDs []uint
	err := dbQuery.Pluck("post_id", &postIDs).Error
	return postIDs, err
}

func GetFollowedPosts(userID uint, cursor int, limit int) ([]model.ForumPost, error) {
	dbQuery := db.Model(&model.ForumFollow{}).Select("post_id").Where("user_id = ?", userID)

	if cursor != 0 {
		dbQuery = dbQuery.Where("post_id < ?", cursor)
	}

	var postIDs []uint
	err := dbQuery.Order("post_id DESC").Limit(limit).Pluck("post_id", &postIDs).Error
	if err != nil {
		return nil, err
	}

	if len(postIDs) == 0 {
		return []model.ForumPost{}, nil
	}

	// 根据帖子ID获取完整的帖子信息
	var posts []model.ForumPost
	err = db.Preload("User").Preload("Tags").
		Where("id IN ?", postIDs).
		Where(forumPostVisibleToUserQuery(userID), userID).
		Order("id DESC").
		Find(&posts).Error
	return posts, err
}

// GetUserFollowStatus 检查用户是否关注特定帖子
func GetUserFollowStatus(userID, postID uint) (bool, error) {
	var count int64
	err := db.Model(&model.ForumFollow{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FollowPost 关注帖子
func FollowPost(userID, postID uint) error {
	follow := model.ForumFollow{
		UserID: userID,
		PostID: postID,
	}
	return db.Create(&follow).Error
}

// UnfollowPost 取消关注帖子
func UnfollowPost(userID, postID uint) error {
	return db.Where("user_id = ? AND post_id = ?", userID, postID).Delete(&model.ForumFollow{}).Error
}

// UpdateForumPostFollownum 更新帖子的关注数量（原Likenum字段）
func UpdateForumPostFollownum(postID uint, follownum int) error {
	return db.Model(&model.ForumPost{}).Where("id = ?", postID).Update("follownum", follownum).Error
}

// UpdateForumPostLikenum 更新帖子的点赞数量（新增函数）
func UpdateForumPostLikenum(postID uint, likenum int) error {
	return db.Model(&model.ForumPost{}).Where("id = ?", postID).Update("likenum", likenum).Error
}

// UpdateForumPostReplyNum 更新帖子的回复数量
func UpdateForumPostReplyNum(postID uint, replynum int) error {
	return db.Model(&model.ForumPost{}).Where("id = ?", postID).Update("reply", replynum).Error
}

func SetForumPostStatus(postID uint, status string) error {
	return db.Model(&model.ForumPost{}).Where("id = ?", postID).Update("status", status).Error
}

func TransitionForumPostStatus(postID uint, fromStatus string, toStatus string) (bool, error) {
	result := db.Model(&model.ForumPost{}).
		Where("id = ? AND status = ?", postID, fromStatus).
		Update("status", toStatus)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func TransitionForumCommentStatus(commentID uint, fromStatus string, toStatus string) (bool, error) {
	result := db.Model(&model.ForumComment{}).
		Where("id = ? AND status = ?", commentID, fromStatus).
		Update("status", toStatus)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func CountApprovedComments(postID uint) (int64, error) {
	var count int64
	err := db.Model(&model.ForumComment{}).
		Where("post_id = ? AND status = ?", postID, model.ForumContentStatusApproved).
		Count(&count).Error
	return count, err
}

// GetForumPostsByIDs 根据ID列表获取帖子
func GetForumPostsByIDs(postIDs []uint) ([]model.ForumPost, error) {
	var posts []model.ForumPost
	err := db.Preload("User").Preload("Tags").
		Where("id IN ?", postIDs).
		Where("status = ?", model.ForumContentStatusApproved).
		Order("id DESC").
		Find(&posts).Error
	return posts, err
}

// GetForumPostsByIDsForViewer 按 ID 批量获取帖子，只返回对 viewerID 可见的那些
func GetForumPostsByIDsForViewer(postIDs []uint, viewerID uint) ([]model.ForumPost, error) {
	var posts []model.ForumPost
	err := db.Preload("User").
		Where("id IN ?", postIDs).
		Where(forumPostVisibleToUserQuery(viewerID), viewerID).
		Find(&posts).Error
	return posts, err
}

// GetUserLikeStatus 检查用户是否点赞特定帖子
func GetUserLikeStatus(userID, postID uint) (bool, error) {
	var count int64
	err := db.Model(&model.ForumLike{}).Where("user_id = ? AND post_id = ?", userID, postID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// LikePost 点赞帖子
func LikePost(userID, postID uint) error {
	like := model.ForumLike{
		UserID: userID,
		PostID: postID,
	}
	return db.Create(&like).Error
}

// UnlikePost 取消点赞帖子
func UnlikePost(userID, postID uint) error {
	return db.Where("user_id = ? AND post_id = ?", userID, postID).Delete(&model.ForumLike{}).Error
}

// GetLikedIDs 获取用户点赞的帖子ID列表（在指定范围内）
func GetLikedIDs(userID uint, minID uint, maxID uint) ([]uint, error) {
	dbQuery := db.Model(&model.ForumLike{}).Select("post_id").Where("user_id = ?", userID)

	if minID > 0 {
		dbQuery = dbQuery.Where("post_id >= ?", minID)
	}
	if maxID > 0 {
		dbQuery = dbQuery.Where("post_id <= ?", maxID)
	}

	var postIDs []uint
	err := dbQuery.Pluck("post_id", &postIDs).Error
	return postIDs, err
}

// GetUserCommentLikeStatus 检查用户是否点赞特定评论
func GetUserCommentLikeStatus(userID, commentID uint) (bool, error) {
	var count int64
	err := db.Model(&model.CommentLike{}).Where("user_id = ? AND comment_id = ?", userID, commentID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserLikedCommentIDs 批量获取用户点赞过的评论ID
func GetUserLikedCommentIDs(userID uint, commentIDs []uint) ([]uint, error) {
	var likedIDs []uint
	err := db.Model(&model.CommentLike{}).
		Where("user_id = ? AND comment_id IN ?", userID, commentIDs).
		Pluck("comment_id", &likedIDs).Error
	return likedIDs, err
}

// LikeComment 点赞评论
func LikeComment(userID, commentID uint) error {
	like := model.CommentLike{
		UserID:    userID,
		CommentID: commentID,
	}
	return db.Create(&like).Error
}

// UnlikeComment 取消点赞评论
func UnlikeComment(userID, commentID uint) error {
	return db.Where("user_id = ? AND comment_id = ?", userID, commentID).Delete(&model.CommentLike{}).Error
}

// UpdateCommentLikenum 更新评论的点赞数量
func UpdateCommentLikenum(commentID uint, likenum int) error {
	return db.Model(&model.ForumComment{}).Where("id = ?", commentID).Update("likenum", likenum).Error
}

func SetForumCommentStatus(commentID uint, status string) error {
	return db.Model(&model.ForumComment{}).Where("id = ?", commentID).Update("status", status).Error
}

func ListPendingForumPosts(limit int) ([]model.ForumPost, error) {
	var posts []model.ForumPost
	err := db.Preload("User").Preload("Tags").
		Where("status = ?", model.ForumContentStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&posts).Error
	return posts, err
}

func ListPendingForumComments(limit int) ([]model.ForumComment, error) {
	var comments []model.ForumComment
	err := db.Preload("User").Preload("Quote").Preload("Quote.User").
		Where("status = ?", model.ForumContentStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

// GetTags 获取所有系统默认标签列表
func GetTags() ([]model.ForumTag, error) {
	var tags []model.ForumTag
	err := db.Where("is_default = ?", true).Find(&tags).Error
	return tags, err
}

// GetPostsByTagNames 根据多个tag名称获取帖子
func GetPostsByTagNames(tagNames []string, cursor int, limit int) ([]model.ForumPost, error) {
	if len(tagNames) == 0 {
		return []model.ForumPost{}, nil
	}

	dbQuery := db.Preload("User").Preload("Tags").
		Joins("JOIN forum_post_tags ON forum_posts.id = forum_post_tags.post_id").
		Joins("JOIN forum_tags ON forum_post_tags.tag_id = forum_tags.id").
		Where("forum_tags.name IN ?", tagNames)

	if cursor != 0 {
		dbQuery = dbQuery.Where("forum_posts.id < ?", cursor)
	}

	var posts []model.ForumPost
	err := dbQuery.Group("forum_posts.id").Order("forum_posts.id DESC").Limit(limit).Find(&posts).Error
	return posts, err
}

func GetPostFollowers(postID uint) ([]uint, error) {
	var followerIDs []uint
	err := db.Raw(`
        SELECT DISTINCT user_id 
        FROM forum_follows 
        WHERE post_id = ? AND deleted_at IS NULL
    `, postID).Scan(&followerIDs).Error
	if err != nil {
		return nil, err
	}
	return followerIDs, nil
}

func GetPostCommenters(postID uint) ([]uint, error) {
	var commenterIDs []uint
	err := db.Raw(`
        SELECT DISTINCT user_id 
        FROM forum_comments 
        WHERE post_id = ? AND deleted_at IS NULL
    `, postID).Scan(&commenterIDs).Error
	if err != nil {
		return nil, err
	}
	return commenterIDs, nil
}

func GetCommentsQuotingComment(commentID uint) ([]model.ForumComment, error) {
	var comments []model.ForumComment
	err := db.Where("quote_id = ? AND deleted_at IS NULL", commentID).Find(&comments).Error
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func DeleteForumPostByID(postID uint) error {
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	if err := tx.Where(&model.ForumPostTag{PostID: postID}).Delete(&model.ForumPostTag{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumFollow{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumLike{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	var poll model.ForumPoll
	if err := tx.Where("post_id = ?", postID).First(&poll).Error; err == nil {
		if err := tx.Where("poll_id = ?", poll.ID).Delete(&model.ForumPollVote{}).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Where("poll_id = ?", poll.ID).Delete(&model.ForumPollOption{}).Error; err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Where("id = ?", poll.ID).Delete(&model.ForumPoll{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		return err
	}

	var comments []model.ForumComment
	if err := tx.Where("post_id = ?", postID).Find(&comments).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, comment := range comments {
		if err := tx.Where("comment_id = ?", comment.ID).Delete(&model.CommentLike{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumComment{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("id = ?", postID).Delete(&model.ForumPost{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func DeleteForumCommentByID(commentID uint) error {
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return err
	}

	var comment model.ForumComment
	if err := tx.Where("id = ?", commentID).First(&comment).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("comment_id = ?", commentID).Delete(&model.CommentLike{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Where("id = ?", commentID).Delete(&model.ForumComment{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	postID := comment.PostID
	var post model.ForumPost
	if err := tx.Where("id = ?", postID).First(&post).Error; err != nil {
		tx.Rollback()
		return err
	}

	if post.Reply > 0 {
		post.Reply--
		if err := tx.Model(&post).Update("reply", post.Reply).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}
