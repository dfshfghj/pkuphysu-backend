package db

import (
	"strings"

	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"
)

// GetForumPostByID 根据ID获取单个帖子
func GetForumPostByID(pid int) (*model.ForumPost, error) {
	var post model.ForumPost
	err := db.Preload("User").Preload("Tags").Where("id = ?", pid).First(&post).Error
	return &post, err
}

// GetForumPosts 获取帖子列表
func GetForumPosts(cursor int, limit int, tag string, keywords []string) ([]model.ForumPost, error) {
	dbQuery := db.Preload("User").Preload("Tags")

	if tag != "" {
		// 通过关联表查询包含指定tag的帖子
		dbQuery = dbQuery.Joins("JOIN forum_post_tags ON forum_posts.id = forum_post_tags.post_id").
			Joins("JOIN forum_tags ON forum_post_tags.tag_id = forum_tags.id").
			Where("forum_tags.name = ?", tag)
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
func GetForumComments(pid string, cursor int, limit int, sort string) ([]model.ForumComment, error) {
	dbQuery := db.Preload("User").Preload("Quote").Preload("Quote.User").Where("post_id = ?", pid)

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

// CreateForumComment 创建评论
func CreateForumComment(comment *model.ForumComment) error {
	comment.ContentHTML = utils.MarkdownToHtml(comment.Content)
	comment.ContentText = utils.MarkdownToText(comment.Content)
	return db.Create(comment).Error
}

// CreateForumPost 创建帖子
func CreateForumPost(post *model.ForumPost) error {
	post.ContentHTML = utils.MarkdownToHtml(post.Content)
	post.ContentText = utils.MarkdownToText(post.Content)

	// 处理标签
	if len(post.Tags) > 0 {
		var finalTags []model.ForumTag
		for _, tag := range post.Tags {
			if tag.Name == "" {
				continue
			}

			var existingTag model.ForumTag
			// 查找是否已存在同名标签
			if err := db.Where("name = ?", tag.Name).First(&existingTag).Error; err != nil {
				// 标签不存在，创建新标签
				newTag := model.ForumTag{Name: tag.Name}
				if err := db.Create(&newTag).Error; err != nil {
					return err
				}
				finalTags = append(finalTags, newTag)
			} else {
				finalTags = append(finalTags, existingTag)
			}
		}
		post.Tags = finalTags
	}

	return db.Create(post).Error
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
	err = db.Preload("User").Preload("Tags").Where("id IN ?", postIDs).Order("id DESC").Find(&posts).Error
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

// GetForumPostsByIDs 根据ID列表获取帖子
func GetForumPostsByIDs(postIDs []uint) ([]model.ForumPost, error) {
	var posts []model.ForumPost
	err := db.Preload("User").Preload("Tags").Where("id IN ?", postIDs).Order("id DESC").Find(&posts).Error
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

// DeleteForumPostByID 根据ID删除帖子（管理员专用）
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

	// 删除帖子的关联数据
	// 1. 删除帖子与标签的关联
	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumPostTag{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 2. 删除帖子的关注记录
	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumFollow{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 3. 删除帖子的点赞记录
	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumLike{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 4. 删除帖子的所有评论及其关联数据
	var comments []model.ForumComment
	if err := tx.Where("post_id = ?", postID).Find(&comments).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, comment := range comments {
		// 删除评论的点赞记录
		if err := tx.Where("comment_id = ?", comment.ID).Delete(&model.CommentLike{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 删除所有评论
	if err := tx.Where("post_id = ?", postID).Delete(&model.ForumComment{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 5. 最后删除帖子本身
	if err := tx.Where("id = ?", postID).Delete(&model.ForumPost{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// DeleteForumCommentByID 根据ID删除评论（管理员专用）
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

	// 1. 删除评论的点赞记录
	if err := tx.Where("comment_id = ?", commentID).Delete(&model.CommentLike{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 2. 删除评论本身
	if err := tx.Where("id = ?", commentID).Delete(&model.ForumComment{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 3. 更新帖子的回复计数
	var comment model.ForumComment
	if err := tx.Where("id = ?", commentID).First(&comment).Error; err != nil {
		// 如果评论不存在，直接返回成功
		tx.Rollback()
		return nil
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
