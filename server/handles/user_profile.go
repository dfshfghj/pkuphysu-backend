package handles

import (
	"errors"
	"strconv"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/moderation"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	// 用户主页默认/最大返回的帖子条数
	defaultProfilePostLimit = 10
	maxProfilePostLimit     = 25
)

// GetUserProfile 获取任意用户的主页：公开信息、统计数据、自定义内容与最近的帖子
func GetUserProfile(c *gin.Context) {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	target, err := db.GetUserById(uint(targetID))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	viewer := c.MustGet("CurrentUser").(*model.User)
	viewerID := viewer.ID

	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultProfilePostLimit)))
	if err != nil || limit <= 0 {
		utils.RespondError(c, 400, "InvalidParam", errors.New("limit 必须为正整数"))
		return
	}
	if limit > maxProfilePostLimit {
		limit = maxProfilePostLimit
	}

	cursor, err := strconv.Atoi(c.DefaultQuery("begin", "0"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	commentLimit, err := strconv.Atoi(c.DefaultQuery("comment_limit", strconv.Itoa(defaultCommentLimit)))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}
	if commentLimit < 0 {
		commentLimit = 0
	}
	if commentLimit > maxCommentLimit {
		commentLimit = maxCommentLimit
	}

	posts, err := db.GetForumPostsByUserID(target.ID, cursor, limit, viewerID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	postCount, commentCount, likesReceived, err := db.GetUserForumStats(target.ID, viewerID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	profile, err := db.GetUserProfile(target.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	profileData := gin.H{"content": "", "updated_at": nil}
	if profile != nil {
		profileData["content"] = profile.ContentHTML
		profileData["updated_at"] = profile.UpdatedAt.Unix()
	}

	followedPostMap := make(map[uint]bool)
	likedPostMap := make(map[uint]bool)
	if len(posts) > 0 {
		minID := posts[len(posts)-1].ID
		maxID := posts[0].ID
		if follows, err := db.GetFollowedIDs(viewerID, minID, maxID); err == nil {
			for _, postID := range follows {
				followedPostMap[postID] = true
			}
		}
		if likes, err := db.GetLikedIDs(viewerID, minID, maxID); err == nil {
			for _, postID := range likes {
				likedPostMap[postID] = true
			}
		}
	}

	postData := make([]map[string]interface{}, len(posts))
	for i, post := range posts {
		isFollow := 0
		if followedPostMap[post.ID] {
			isFollow = 1
		}
		isLike := 0
		if likedPostMap[post.ID] {
			isLike = 1
		}
		postData[i] = forumPostSummary(post, isFollow, isLike)
	}

	if err := attachCommentPreviews(postData, posts, viewerID, commentLimit); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	attachPolls(postData, posts, viewerID)
	attachSurveys(postData, posts, viewer)

	utils.RespondSuccess(c, gin.H{
		"user": gin.H{
			"id":       target.ID,
			"username": target.Username,
			"bio":      target.Bio,
			"role":     target.Role,
			"verified": target.Verified,
		},
		"is_self": target.ID == viewerID,
		"profile": profileData,
		"stats": gin.H{
			"post_count":     postCount,
			"comment_count":  commentCount,
			"likes_received": likesReceived,
		},
		"posts": postData,
	})
}

// GetUserStats 获取用户的公开统计（帖子数、评论数、获赞数）
func GetUserStats(c *gin.Context) {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	if _, err := db.GetUserById(uint(targetID)); err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	viewerID := c.MustGet("CurrentUser").(*model.User).ID

	postCount, commentCount, likesReceived, err := db.GetUserForumStats(uint(targetID), viewerID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"post_count":     postCount,
		"comment_count":  commentCount,
		"likes_received": likesReceived,
	})
}

// GetMyProfileRaw 获取自己主页自定义内容的原始 Markdown，供编辑使用
func GetMyProfileRaw(c *gin.Context) {
	currentUser := c.MustGet("CurrentUser").(*model.User)

	profile, err := db.GetUserProfile(currentUser.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	content := ""
	if profile != nil {
		content = profile.Content
	}

	utils.RespondSuccess(c, gin.H{
		"content":    content,
		"max_length": model.MaxUserProfileContentLength,
	})
}

// UpdateMyProfile 更新自己主页的自定义 Markdown 区块
func UpdateMyProfile(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	if err := model.ValidateUserProfileContent(req.Content); err != nil {
		utils.RespondError(c, 400, "ContentTooLong", err)
		return
	}
	if _, blocked := moderation.MatchSensitiveWord(req.Content); blocked {
		utils.RespondError(c, 403, "SensitiveContentRejected", errors.New("主页内容包含敏感词，已被直接拒绝"))
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	profile := &model.UserProfile{
		UserID:  currentUser.ID,
		Content: req.Content,
	}
	if err := db.SaveUserProfile(profile); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message": "主页内容已更新",
		"content": profile.ContentHTML,
	})
}
