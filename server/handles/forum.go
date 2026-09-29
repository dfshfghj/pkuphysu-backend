package handles

import (
	"errors"
	"strconv"
	"strings"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/moderation"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// 新内容的初始状态：REVIEW_REQUIRED = true 时先审后发，false 时直接发布
func initialForumContentStatus() string {
	if config.Conf.Moderation.ReviewRequired {
		return model.ForumContentStatusPending
	}
	return model.ForumContentStatusApproved
}

// 评论不经过审核直接可见时，补齐审核流程里的副作用（帖子回复数、通知楼主）
func publishCommentEffects(comment *model.ForumComment) {
	approvedCount, err := db.CountApprovedComments(comment.PostID)
	if err != nil {
		logrus.WithError(err).Warn("failed to count approved comments")
		return
	}
	if err := db.UpdateForumPostReplyNum(comment.PostID, int(approvedCount)); err != nil {
		logrus.WithError(err).Warn("failed to update post reply num")
	}

	post, err := db.GetForumPostByID(int(comment.PostID))
	if err != nil {
		logrus.WithError(err).Warn("failed to load post for comment notification")
		return
	}
	if post.UserID == comment.UserID {
		return
	}
	if err := db.CreateNotification(&model.Notification{
		UserID:        post.UserID,
		Title:         "new_comment",
		Content:       "您的帖子收到了新评论",
		Type:          "forum_comment",
		Read:          false,
		RelatedUserID: comment.UserID,
		RelatedPostID: post.ID,
	}); err != nil {
		logrus.WithError(err).Warn("failed to create comment notification")
	}
}

func canViewForumContent(status string, ownerID uint, viewerID uint) bool {
	return status == model.ForumContentStatusApproved || ownerID == viewerID
}

func quotePayloadForViewer(comment *model.ForumComment, viewerID uint) gin.H {
	if comment == nil || comment.Quote == nil || comment.Quote.User == nil {
		return nil
	}
	if !canViewForumContent(comment.Quote.Status, comment.Quote.UserID, viewerID) {
		return nil
	}

	return gin.H{
		"cid":      comment.Quote.ID,
		"username": comment.Quote.User.Username,
		"text":     comment.Quote.ContentHTML,
		"status":   comment.Quote.Status,
	}
}

func isAdminReviewStatus(status string) bool {
	return status == model.ForumContentStatusApproved || status == model.ForumContentStatusRejected
}

func GetPost(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	post, err := db.GetForumPostByID(pid)
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID
	if !canViewForumContent(post.Status, post.UserID, userID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}

	isFollow := 0
	followed, err := db.GetUserFollowStatus(userID, post.ID)
	if err == nil && followed {
		isFollow = 1
	}

	isLike := 0
	liked, err := db.GetUserLikeStatus(userID, post.ID)
	if err == nil && liked {
		isLike = 1
	}

	tags := make([]string, len(post.Tags))
	for i, tag := range post.Tags {
		tags[i] = tag.Name
	}

	postData := map[string]interface{}{
		"id":        post.ID,
		"text":      post.ContentHTML,
		"timestamp": post.CreatedAt.Unix(),
		"follownum": post.Follownum,
		"likenum":   post.Likenum,
		"reply":     post.Reply,
		"tags":      tags,
		"status":    post.Status,
		"is_follow": isFollow,
		"is_like":   isLike,
		"userid":    post.User.ID,
		"username":  post.User.Username,
	}

	utils.RespondSuccess(c, postData)
}

func GetPosts(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	cursor, err := strconv.Atoi(c.DefaultQuery("begin", "0"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	var tags []string
	if c.Request.URL.Query().Has("tag") {
		tags = c.QueryArray("tag")
		for i := len(tags) - 1; i >= 0; i-- {
			tags[i] = strings.TrimSpace(tags[i])
			if tags[i] == "" {
				tags = append(tags[:i], tags[i+1:]...)
			}
		}
	}

	var keywords []string
	if c.Request.URL.Query().Has("keyword") {
		keywords = c.QueryArray("keyword")
		for i := len(keywords) - 1; i >= 0; i-- {
			keywords[i] = strings.TrimSpace(keywords[i])
			if keywords[i] == "" {
				keywords = append(keywords[:i], keywords[i+1:]...)
			}
		}
	}

	var posts []model.ForumPost

	userID := c.MustGet("CurrentUser").(*model.User).ID

	posts, err = db.GetForumPosts(cursor, limit, tags, keywords, userID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if len(posts) == 0 {
		utils.RespondSuccess(c, []map[string]interface{}{})
		return
	}
	minID := posts[len(posts)-1].ID
	maxID := posts[0].ID

	followedPostMap := make(map[uint]bool)
	follows, err := db.GetFollowedIDs(userID, minID, maxID)
	if err == nil {
		for _, postID := range follows {
			followedPostMap[postID] = true
		}
	}

	likedPostMap := make(map[uint]bool)
	likes, err := db.GetLikedIDs(userID, minID, maxID)
	if err == nil {
		for _, postID := range likes {
			likedPostMap[postID] = true
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

		tags := make([]string, len(post.Tags))
		for j, tag := range post.Tags {
			tags[j] = tag.Name
		}

		postData[i] = map[string]interface{}{
			"id":        post.ID,
			"text":      post.ContentHTML,
			"type":      post.Type,
			"timestamp": post.CreatedAt.Unix(),
			"follownum": post.Follownum,
			"likenum":   post.Likenum,
			"reply":     post.Reply,
			"tags":      tags,
			"status":    post.Status,
			"is_follow": isFollow,
			"is_like":   isLike,
			"userid":    post.User.ID,
			"username":  post.User.Username,
		}
	}

	utils.RespondSuccess(c, postData)
}

func GetComments(c *gin.Context) {
	id := c.Param("id")

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidLimit", err)
		return
	}

	sort := c.DefaultQuery("sort", "asc")

	cursor, err := strconv.Atoi(c.DefaultQuery("begin", "0"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID

	postID, err := strconv.Atoi(id)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	post, err := db.GetForumPostByID(postID)
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if !canViewForumContent(post.Status, post.UserID, userID) {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	comments, err := db.GetForumComments(id, cursor, limit, sort, userID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	commentData := make([]map[string]interface{}, len(comments))
	for i, comment := range comments {
		quoteInfo := quotePayloadForViewer(&comment, userID)

		// 检查当前用户是否点赞了该评论
		isLike := 0
		liked, err := db.GetUserCommentLikeStatus(userID, comment.ID)
		if err == nil && liked {
			isLike = 1
		}

		commentData[i] = map[string]interface{}{
			"cid":       comment.ID,
			"pid":       comment.PostID,
			"text":      comment.ContentHTML,
			"quote":     quoteInfo,
			"timestamp": comment.CreatedAt.Unix(),
			"userid":    comment.User.ID,
			"username":  comment.User.Username,
			"status":    comment.Status,
			"likenum":   comment.Likenum,
			"is_like":   isLike,
		}
	}

	utils.RespondSuccess(c, commentData)
}

func SubmitComment(c *gin.Context) {
	var req struct {
		Pid   uint   `json:"pid"`
		Text  string `json:"text"`
		Quote *uint  `json:"quote"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	post, err := db.GetForumPostByID(int(req.Pid))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if !canViewForumContent(post.Status, post.UserID, currentUser.ID) {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if _, blocked := moderation.MatchSensitiveWord(req.Text); blocked {
		utils.RespondError(c, 403, "SensitiveContentRejected", errors.New("评论包含敏感词，已被直接拒绝"))
		return
	}

	comment := model.ForumComment{
		Content: req.Text,
		Status:  initialForumContentStatus(),
		Likenum: 0,
		PostID:  req.Pid,
		UserID:  currentUser.ID,
	}

	if req.Quote != nil {
		comment.QuoteID = req.Quote
	}

	err = db.CreateForumComment(&comment)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	message := "评论成功"
	if comment.Status == model.ForumContentStatusPending {
		moderation.EnqueueComment(comment.ID)
		message = "评论提交成功，等待审核"
	} else {
		publishCommentEffects(&comment)
	}

	utils.RespondSuccess(c, gin.H{
		"message": message,
		"cid":     comment.ID,
		"status":  comment.Status,
	})
}

func SubmitPost(c *gin.Context) {
	var req struct {
		Text string   `json:"text"`
		Tags []string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)
	if _, blocked := moderation.MatchSensitiveWord(req.Text); blocked {
		utils.RespondError(c, 403, "SensitiveContentRejected", errors.New("帖子包含敏感词，已被直接拒绝"))
		return
	}

	// 构建tag列表
	var tags []model.ForumTag
	for _, tagName := range req.Tags {
		if tagName != "" { // 忽略空tag
			tags = append(tags, model.ForumTag{Name: tagName})
		}
	}

	post := model.ForumPost{
		Content: req.Text,
		Status:  initialForumContentStatus(),
		UserID:  currentUser.ID,
		Tags:    tags,
	}

	err := db.CreateForumPost(&post)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	message := "帖子发布成功"
	if post.Status == model.ForumContentStatusPending {
		moderation.EnqueuePost(post.ID)
		message = "帖子发布成功，等待审核"
	}

	utils.RespondSuccess(c, gin.H{
		"message": message,
		"id":      post.ID,
		"status":  post.Status,
	})
}

func GetFollowedPosts(c *gin.Context) {
	limitStr := c.Query("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		utils.RespondError(c, 400, "InvalidLimit", err)
		return
	}

	beginStr := c.Query("begin")
	var cursorValue int
	if beginStr != "" {
		beginVal, err := strconv.Atoi(beginStr)
		if err != nil {
			utils.RespondError(c, 400, "InvalidBegin", err)
			return
		}
		cursorValue = beginVal
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	posts, err := db.GetFollowedPosts(currentUser.ID, cursorValue, limit)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	postData := make([]map[string]interface{}, len(posts))
	for i, post := range posts {
		// 提取tag名称列表
		tags := make([]string, len(post.Tags))
		for j, tag := range post.Tags {
			tags[j] = tag.Name
		}

		postData[i] = map[string]interface{}{
			"id":        post.ID,
			"text":      post.ContentHTML,
			"type":      post.Type,
			"timestamp": post.CreatedAt.Unix(),
			"follownum": post.Follownum,
			"likenum":   post.Likenum,
			"reply":     post.Reply,
			"tags":      tags,
			"status":    post.Status,
			"is_follow": 1,
			"userid":    post.User.ID,
			"username":  post.User.Username,
		}
	}

	utils.RespondSuccess(c, postData)
}

func FollowPost(c *gin.Context) {
	id := c.Param("id")

	postID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	// 检查是否已经关注
	isFollowed, err := db.GetUserFollowStatus(currentUser.ID, uint(postID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if !isFollowed {
		// 未关注，新建关注记录
		err = db.FollowPost(currentUser.ID, uint(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 获取当前帖子的 Follownum 并加1
		post, err := db.GetForumPostByID(int(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		post.Follownum += 1
		err = db.UpdateForumPostFollownum(uint(postID), post.Follownum)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 发送通知给帖子作者
		// if post.UserID != currentUser.ID {
		//	notification := &model.Notification{
		//		UserID:  post.UserID,
		//		Title:   "您的帖子被关注了",
		//		Content: fmt.Sprintf("用户 %s 关注了您的帖子", currentUser.Username),
		//		Type:    "forum_follow",
		//		Read:    false,
		//	}
		//	db.CreateNotification(notification)
		//}

		utils.RespondSuccess(c, gin.H{"message": "关注成功"})
	} else {
		// 已关注，删除关注记录
		err = db.UnfollowPost(currentUser.ID, uint(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 获取当前帖子的 Follownum 并减1（确保不小于0）
		post, err := db.GetForumPostByID(int(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		if post.Follownum > 0 {
			post.Follownum -= 1
		}
		err = db.UpdateForumPostFollownum(uint(postID), post.Follownum)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		utils.RespondSuccess(c, gin.H{"message": "取消关注成功"})
	}
}

func LikePost(c *gin.Context) {
	id := c.Param("id")

	postID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	// 检查是否已经点赞
	isLiked, err := db.GetUserLikeStatus(currentUser.ID, uint(postID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	var newLikenum int
	if !isLiked {
		// 未点赞，添加点赞记录
		err = db.LikePost(currentUser.ID, uint(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 更新 Likenum: +1
		post, err := db.GetForumPostByID(int(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		newLikenum = post.Likenum + 1

		// 发送通知给帖子作者
		// if post.UserID != currentUser.ID {
		//	notification := &model.Notification{
		//		UserID:  post.UserID,
		//		Title:   "您的帖子被点赞了",
		//		Content: fmt.Sprintf("用户 %s 点赞了您的帖子", currentUser.Username),
		//		Type:    "forum_like",
		//		Read:    false,
		//	}
		//	db.CreateNotification(notification)
		// }
	} else {
		// 已点赞，取消点赞
		err = db.UnlikePost(currentUser.ID, uint(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 更新 Likenum: -1（不能小于0）
		post, err := db.GetForumPostByID(int(postID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		if post.Likenum > 0 {
			newLikenum = post.Likenum - 1
		} else {
			newLikenum = 0
		}
	}

	// 更新数据库中的 Likenum 字段
	err = db.UpdateForumPostLikenum(uint(postID), newLikenum)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	message := "点赞成功"
	if isLiked {
		message = "取消点赞成功"
	}

	utils.RespondSuccess(c, gin.H{
		"message":  message,
		"likenum":  newLikenum,
		"is_liked": !isLiked,
	})
}

func LikeComment(c *gin.Context) {
	id := c.Param("id")

	commentID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	// 检查是否已经点赞
	isLiked, err := db.GetUserCommentLikeStatus(currentUser.ID, uint(commentID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	var newLikenum int
	if !isLiked {
		// 未点赞，添加点赞记录
		err = db.LikeComment(currentUser.ID, uint(commentID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 获取当前评论并更新 Likenum: +1
		comment, err := db.GetForumCommentByID(uint(commentID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		newLikenum = comment.Likenum + 1

		// if comment.UserID != currentUser.ID {
		// 	notification := &model.Notification{
		// 		UserID:  comment.UserID,
		// 		Title:   "您的评论被点赞了",
		// 		Content: fmt.Sprintf("用户 %s 点赞了您的评论", currentUser.Username),
		// 		Type:    "comment_like",
		//		Read:    false,
		//	}
		//	db.CreateNotification(notification)
		// }
	} else {
		// 已点赞，取消点赞
		err = db.UnlikeComment(currentUser.ID, uint(commentID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		// 获取当前评论并更新 Likenum: -1（不能小于0）
		comment, err := db.GetForumCommentByID(uint(commentID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		if comment.Likenum > 0 {
			newLikenum = comment.Likenum - 1
		} else {
			newLikenum = 0
		}
	}

	// 更新数据库中的 Likenum 字段
	err = db.UpdateCommentLikenum(uint(commentID), newLikenum)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	message := "点赞成功"
	if isLiked {
		message = "取消点赞成功"
	}

	utils.RespondSuccess(c, gin.H{
		"message":  message,
		"likenum":  newLikenum,
		"is_liked": !isLiked,
	})
}

// GetTags 获取所有可用的标签列表
func GetTags(c *gin.Context) {
	tags, err := db.GetTags()
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	type TagInfo struct {
		ID        uint   `json:"id"`
		TagName   string `json:"tag_name"`
		IsDefault bool   `json:"is_default"`
	}

	tagInfos := make([]TagInfo, len(tags))
	for i, tag := range tags {
		tagInfos[i] = TagInfo{
			ID:        tag.ID,
			TagName:   tag.Name,
			IsDefault: tag.IsDefault,
		}
	}

	utils.RespondSuccess(c, tagInfos)
}

func ReviewPostByID(c *gin.Context) {
	id := c.Param("id")
	postID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}
	if !isAdminReviewStatus(req.Status) {
		utils.RespondError(c, 400, "InvalidStatus", nil)
		return
	}

	if err := db.SetForumPostStatus(uint(postID), req.Status); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	if err := db.ResolveForumReports(model.ForumReportTargetPost, uint(postID), c.MustGet("CurrentUser").(*model.User).ID); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if req.Status == model.ForumContentStatusRejected {
		if post, err := db.GetForumPostByID(int(postID)); err == nil {
			db.CreateNotification(&model.Notification{
				UserID:        post.UserID,
				Title:         "post_rejected",
				Content:       "您的帖子未通过审核",
				Type:          "forum_post_moderation",
				Read:          false,
				RelatedPostID: post.ID,
			})
		}
	}

	utils.RespondSuccess(c, gin.H{"message": "帖子审核状态已更新", "status": req.Status})
}

func ReviewCommentByID(c *gin.Context) {
	id := c.Param("id")
	commentID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}
	if !isAdminReviewStatus(req.Status) {
		utils.RespondError(c, 400, "InvalidStatus", nil)
		return
	}

	if err := db.SetForumCommentStatus(uint(commentID), req.Status); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	if err := db.ResolveForumReports(model.ForumReportTargetComment, uint(commentID), c.MustGet("CurrentUser").(*model.User).ID); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	comment, err := db.GetForumCommentByID(uint(commentID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	approvedCount, err := db.CountApprovedComments(comment.PostID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	if err := db.UpdateForumPostReplyNum(comment.PostID, int(approvedCount)); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if req.Status == model.ForumContentStatusApproved {
		post, err := db.GetForumPostByID(int(comment.PostID))
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
		if post.UserID != comment.UserID {
			db.CreateNotification(&model.Notification{
				UserID:        post.UserID,
				Title:         "new_comment",
				Content:       "您的帖子收到了新评论",
				Type:          "forum_comment",
				Read:          false,
				RelatedUserID: comment.UserID,
				RelatedPostID: post.ID,
			})
		}
	}

	if req.Status == model.ForumContentStatusRejected {
		db.CreateNotification(&model.Notification{
			UserID:           comment.UserID,
			Title:            "comment_rejected",
			Content:          "您的评论未通过审核",
			Type:             "forum_comment_moderation",
			Read:             false,
			RelatedPostID:    comment.PostID,
			RelatedCommentID: comment.ID,
		})
	}

	utils.RespondSuccess(c, gin.H{"message": "评论审核状态已更新", "status": req.Status})
}

// DeletePostByID 管理员按ID删除帖子
func DeletePostByID(c *gin.Context) {
	id := c.Param("id")

	postID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	// 获取帖子信息，以便发送通知
	post, err := db.GetForumPostByID(int(postID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if post == nil {
		utils.RespondError(c, 404, "PostNotFound", nil)
		return
	}

	// 先获取通知目标，再删除帖子。删除后关联关系会被清理，无法再查到关注/评论用户。
	followers, followerErr := db.GetPostFollowers(uint(postID))
	commenters, commenterErr := db.GetPostCommenters(uint(postID))

	err = db.DeleteForumPostByID(uint(postID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	// 发送通知给关注者
	notifiedUsers := make(map[uint]bool)
	if followerErr == nil {
		for _, followerID := range followers {
			if followerID != post.UserID { // 不通知自己
				notification := &model.Notification{
					UserID:        followerID,
					Title:         "post_deleted",
					Content:       "您关注的帖子已被删除",
					Type:          "forum_delete",
					Read:          false,
					RelatedUserID: post.UserID,
					RelatedPostID: post.ID,
				}
				db.CreateNotification(notification)
				notifiedUsers[followerID] = true
			}
		}
	}

	// 发送通知给评论者
	if commenterErr == nil {
		for _, commenterID := range commenters {
			if commenterID != post.UserID && !notifiedUsers[commenterID] { // 不通知自己，也不重复通知
				notification := &model.Notification{
					UserID:        commenterID,
					Title:         "post_deleted",
					Content:       "您评论过的帖子已被删除",
					Type:          "forum_delete",
					Read:          false,
					RelatedUserID: post.UserID,
					RelatedPostID: post.ID,
				}
				db.CreateNotification(notification)
				notifiedUsers[commenterID] = true
			}
		}
	}

	utils.RespondSuccess(c, gin.H{"message": "帖子删除成功"})
}

// DeleteCommentByID 管理员按ID删除评论
func DeleteCommentByID(c *gin.Context) {
	id := c.Param("id")

	commentID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	// 获取评论信息，以便发送通知
	comment, err := db.GetForumCommentByID(uint(commentID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	if comment == nil {
		utils.RespondError(c, 404, "CommentNotFound", nil)
		return
	}

	err = db.DeleteForumCommentByID(uint(commentID))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	// 发送通知给回复这条评论的用户（即引用了此评论的评论者）
	replyingComments, err := db.GetCommentsQuotingComment(uint(commentID))
	if err == nil {
		// 使用map去重，避免重复通知
		notifiedUsers := make(map[uint]bool)

		for _, replyingComment := range replyingComments {
			if replyingComment.UserID != comment.UserID && !notifiedUsers[replyingComment.UserID] {
				notification := &model.Notification{
					UserID:        replyingComment.UserID,
					Title:         "comment_deleted",
					Content:       "您回复的评论已被删除",
					Type:          "comment_delete",
					Read:          false,
					RelatedUserID: comment.UserID,
					RelatedPostID: comment.PostID,
				}
				db.CreateNotification(notification)
				notifiedUsers[replyingComment.UserID] = true
			}
		}
	}

	// 如果此评论引用了其他评论，也要通知原评论作者
	if comment.QuoteID != nil {
		quotedComment, err := db.GetForumCommentByID(*comment.QuoteID)
		if err == nil && quotedComment != nil && quotedComment.UserID != comment.UserID {
			notification := &model.Notification{
				UserID:        quotedComment.UserID,
				Title:         "comment_deleted",
				Content:       "您评论的评论已被删除",
				Type:          "comment_delete",
				Read:          false,
				RelatedUserID: comment.UserID,
				RelatedPostID: comment.PostID,
			}
			db.CreateNotification(notification)
		}
	}

	utils.RespondSuccess(c, gin.H{"message": "评论删除成功"})
}

func GetRawPost(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	post, err := db.GetForumPostByID(pid)
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID
	if !canViewForumContent(post.Status, post.UserID, userID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}

	rawData := map[string]interface{}{
		"id":        post.ID,
		"content":   post.Content,
		"timestamp": post.CreatedAt.Unix(),
		"status":    post.Status,
		"userid":    post.User.ID,
		"username":  post.User.Username,
	}

	utils.RespondSuccess(c, rawData)
}

func GetRawComment(c *gin.Context) {
	cid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	comment, err := db.GetForumCommentByID(uint(cid))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID
	if !canViewForumContent(comment.Status, comment.UserID, userID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}

	rawData := map[string]interface{}{
		"cid":       comment.ID,
		"content":   comment.Content,
		"timestamp": comment.CreatedAt.Unix(),
		"status":    comment.Status,
		"userid":    comment.User.ID,
		"username":  comment.User.Username,
	}

	utils.RespondSuccess(c, rawData)
}
