package db

import (
	"pkuphysu-backend/internal/model"
)

// GetForumPostByID 根据ID获取单个帖子
func GetForumPostByID(pid int) (*model.ForumPost, error) {
	var post model.ForumPost
	err := db.Preload("User").Where("id = ?", pid).First(&post).Error
	return &post, err
}

// GetForumPosts 获取帖子列表
func GetForumPosts(cursor int, limit int, tag string, keyword string) ([]model.ForumPost, error) {
	dbQuery := db.Preload("User")
	
	if tag != "" {
		dbQuery = dbQuery.Where("tag = ?", tag)
	}
	if keyword != "" {
		dbQuery = dbQuery.Where("content LIKE ?", "%"+keyword+"%")
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
	return db.Create(comment).Error
}

// CreateForumPost 创建帖子
func CreateForumPost(post *model.ForumPost) error {
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
	err = db.Preload("User").Where("id IN ?", postIDs).Order("id DESC").Find(&posts).Error
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
	err := db.Preload("User").Where("id IN ?", postIDs).Order("id DESC").Find(&posts).Error
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
