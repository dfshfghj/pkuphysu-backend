package handles

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/moderation"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	// 每个帖子允许的最大修改次数
	maxForumPostEdits = 3
	// 帖子列表附带的评论：每个帖子的默认/最大条数
	defaultCommentLimit = 3
	maxCommentLimit     = 10
	// 正文引用卡片：单次最多解析的帖子数、摘要的最大字符数
	maxPostQuoteIDs        = 50
	postQuoteExcerptLength = 120
)

// 新内容的初始状态：REVIEW_REQUIRED = true 时先审后发，false 时直接发布
func initialForumContentStatus() string {
	if config.Conf.Moderation.ReviewRequired {
		return model.ForumContentStatusPending
	}
	return model.ForumContentStatusApproved
}

func editedAtUnix(editedAt *time.Time) interface{} {
	if editedAt == nil {
		return nil
	}
	return editedAt.Unix()
}

var mentionLinkPattern = regexp.MustCompile(`\[@[^\]\n]*\]\(/u/(\d+)\)`)

func parseMentionUserIDs(content string) []uint {
	matches := mentionLinkPattern.FindAllStringSubmatch(content, -1)
	ids := make([]uint, 0, len(matches))
	seen := make(map[uint]bool, len(matches))

	for _, match := range matches {
		value, err := strconv.ParseUint(match[1], 10, 32)
		if err != nil || value == 0 || seen[uint(value)] {
			continue
		}
		seen[uint(value)] = true
		ids = append(ids, uint(value))
	}

	return ids
}

func notifyMentions(content string, authorID uint, postID uint, commentID uint, title string, message string, extraIDs ...uint) {
	ids := append(parseMentionUserIDs(content), extraIDs...)
	if len(ids) == 0 {
		return
	}

	seen := make(map[uint]bool, len(ids))
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || id == authorID || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}
	if len(unique) == 0 {
		return
	}

	existing, err := db.GetExistingUserIDs(unique)
	if err != nil {
		logrus.WithError(err).Warn("failed to filter mentioned users")
		return
	}

	for _, userID := range existing {
		if err := db.CreateNotification(&model.Notification{
			UserID:           userID,
			Title:            title,
			Content:          message,
			Type:             "forum_mention",
			Read:             false,
			RelatedUserID:    authorID,
			RelatedPostID:    postID,
			RelatedCommentID: commentID,
		}); err != nil {
			logrus.WithError(err).Warn("failed to create mention notification")
		}
	}
}

func notifyCommentMentions(comment *model.ForumComment) {
	var extraIDs []uint
	if comment.QuoteID != nil {
		if quoted, err := db.GetForumCommentByID(*comment.QuoteID); err == nil && quoted != nil {
			extraIDs = append(extraIDs, quoted.UserID)
		}
	}
	notifyMentions(comment.Content, comment.UserID, comment.PostID, comment.ID, "mention_comment", "在评论中提到了您", extraIDs...)
}

func publishPostEffects(post *model.ForumPost) {
	notifyMentions(post.Content, post.UserID, post.ID, 0, "mention_post", "在帖子中提到了您")
}

func publishCommentEffects(comment *model.ForumComment) {
	approvedCount, err := db.CountApprovedComments(comment.PostID)
	if err != nil {
		logrus.WithError(err).Warn("failed to count approved comments")
	} else if err := db.UpdateForumPostReplyNum(comment.PostID, int(approvedCount)); err != nil {
		logrus.WithError(err).Warn("failed to update post reply num")
	}

	post, err := db.GetForumPostByID(int(comment.PostID))
	if err != nil {
		logrus.WithError(err).Warn("failed to load post for comment notification")
	} else if post.UserID != comment.UserID {
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

	notifyCommentMentions(comment)
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
		"userid":   comment.Quote.UserID,
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

	currentUser := c.MustGet("CurrentUser").(*model.User)
	userID := currentUser.ID
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
		"id":             post.ID,
		"text":           post.ContentHTML,
		"timestamp":      post.CreatedAt.Unix(),
		"follownum":      post.Follownum,
		"likenum":        post.Likenum,
		"reply":          post.Reply,
		"tags":           tags,
		"status":         post.Status,
		"is_follow":      isFollow,
		"is_like":        isLike,
		"userid":         post.User.ID,
		"username":       post.User.Username,
		"edit_count":     post.EditCount,
		"max_edit_count": maxForumPostEdits,
		"edited_at":      editedAtUnix(post.LastEditedAt),
	}

	if views, err := db.GetForumPollViews([]uint{post.ID}, userID); err == nil {
		postData["poll"] = buildPollPayload(views[post.ID])
	} else {
		postData["poll"] = nil
	}

	if survey, err := db.GetForumSurveyByPostID(post.ID); err == nil {
		responseCount, countErr := db.GetSurveyResponseCount(survey.ID)
		if countErr != nil {
			logrus.WithError(countErr).Warn("failed to count survey responses")
		}
		postData["survey"] = buildSurveyMetaPayload(survey, responseCount, currentUser.ID, currentUser.IsAdmin())
	} else {
		postData["survey"] = nil
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
	if err := attachCommentPreviews([]map[string]interface{}{postData}, []model.ForumPost{*post}, userID, commentLimit); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
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

	commentLimit, err := strconv.Atoi(c.DefaultQuery("comment_limit", strconv.Itoa(defaultCommentLimit)))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}
	if commentLimit < 0 {
		utils.RespondError(c, 400, "InvalidParam", errors.New("comment_limit 不能为负数"))
		return
	}
	if commentLimit > maxCommentLimit {
		commentLimit = maxCommentLimit
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

	currentUser := c.MustGet("CurrentUser").(*model.User)
	userID := currentUser.ID

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

	// 每个帖子附带最新的若干条可见评论
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

	if err := attachCommentPreviews(postData, posts, userID, commentLimit); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	attachPolls(postData, posts, userID)
	attachSurveys(postData, posts, currentUser)

	utils.RespondSuccess(c, postData)
}

// forumPostSummary 构建帖子列表中的单条帖子数据（不含评论预览），供帖子列表与用户主页复用
func forumPostSummary(post model.ForumPost, isFollow int, isLike int) map[string]interface{} {
	tags := make([]string, len(post.Tags))
	for i, tag := range post.Tags {
		tags[i] = tag.Name
	}

	return map[string]interface{}{
		"id":         post.ID,
		"text":       post.ContentHTML,
		"type":       post.Type,
		"timestamp":  post.CreatedAt.Unix(),
		"follownum":  post.Follownum,
		"likenum":    post.Likenum,
		"reply":      post.Reply,
		"tags":       tags,
		"status":     post.Status,
		"is_follow":  isFollow,
		"is_like":    isLike,
		"userid":     post.User.ID,
		"username":   post.User.Username,
		"edit_count": post.EditCount,
	}
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

// latestCommentPayloads 构建帖子列表中评论预览的响应数据
func latestCommentPayloads(comments []model.ForumComment, likedIDs map[uint]bool, viewerID uint) []map[string]interface{} {
	items := make([]map[string]interface{}, len(comments))
	for i := range comments {
		comment := &comments[i]
		isLike := 0
		if likedIDs[comment.ID] {
			isLike = 1
		}
		items[i] = map[string]interface{}{
			"cid":       comment.ID,
			"pid":       comment.PostID,
			"text":      comment.ContentHTML,
			"quote":     quotePayloadForViewer(comment, viewerID),
			"timestamp": comment.CreatedAt.Unix(),
			"userid":    comment.User.ID,
			"username":  comment.User.Username,
			"status":    comment.Status,
			"likenum":   comment.Likenum,
			"is_like":   isLike,
		}
	}
	return items
}

// attachCommentPreviews 为帖子列表批量附加最新评论预览（含当前用户的评论点赞状态）
func attachCommentPreviews(postData []map[string]interface{}, posts []model.ForumPost, viewerID uint, limit int) error {
	if len(postData) == 0 {
		return nil
	}

	if limit <= 0 {
		for i := range postData {
			postData[i]["comments"] = []map[string]interface{}{}
		}
		return nil
	}

	postIDs := make([]uint, len(posts))
	for i, post := range posts {
		postIDs[i] = post.ID
	}

	commentsByPost, err := db.GetLatestCommentsByPostIDs(postIDs, viewerID, limit)
	if err != nil {
		return err
	}

	// 批量查询当前用户的评论点赞状态，避免逐条查询
	likedCommentIDs := make(map[uint]bool)
	allCommentIDs := make([]uint, 0, len(posts)*limit)
	for _, comments := range commentsByPost {
		for _, comment := range comments {
			allCommentIDs = append(allCommentIDs, comment.ID)
		}
	}
	if len(allCommentIDs) > 0 {
		if ids, err := db.GetUserLikedCommentIDs(viewerID, allCommentIDs); err == nil {
			for _, id := range ids {
				likedCommentIDs[id] = true
			}
		}
	}

	for i, post := range posts {
		postData[i]["comments"] = latestCommentPayloads(commentsByPost[post.ID], likedCommentIDs, viewerID)
	}
	return nil
}

// notifyOnce 发送去重通知：同一 (接收人, 类型, 发起人, 帖子/评论) 只通知一次，
// 避免用户反复点赞/取消/再点赞时刷出大量通知。
func notifyOnce(userID uint, notifType, content string, relatedUserID, relatedPostID, relatedCommentID uint) {
	if userID == 0 || userID == relatedUserID {
		return
	}

	exists, err := db.NotificationExists(userID, notifType, relatedUserID, relatedPostID, relatedCommentID)
	if err != nil || exists {
		return
	}

	_ = db.CreateNotification(&model.Notification{
		UserID:           userID,
		Title:            notifType,
		Content:          content,
		Type:             notifType,
		Read:             false,
		RelatedUserID:    relatedUserID,
		RelatedPostID:    relatedPostID,
		RelatedCommentID: relatedCommentID,
	})
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
		Poll *struct {
			Multiple bool     `json:"multiple"`
			Options  []string `json:"options"`
		} `json:"poll"`
		Survey *surveyInput `json:"survey"`
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

	var poll *model.ForumPoll
	if req.Poll != nil {
		multiple, options, err := normalizePollInput(req.Poll.Multiple, req.Poll.Options)
		if err != nil {
			utils.RespondError(c, 400, "InvalidPoll", err)
			return
		}
		pollOptions := make([]model.ForumPollOption, len(options))
		for i, text := range options {
			pollOptions[i] = model.ForumPollOption{Text: text, Position: i}
		}
		poll = &model.ForumPoll{Multiple: multiple, Options: pollOptions}
	}

	var survey *model.ForumSurvey
	if req.Survey != nil {
		normalized, err := normalizeSurveyInput(*req.Survey)
		if err != nil {
			utils.RespondError(c, 400, "InvalidSurvey", err)
			return
		}
		normalized.UserID = currentUser.ID
		survey = normalized
	}

	post := model.ForumPost{
		Content: req.Text,
		Status:  initialForumContentStatus(),
		UserID:  currentUser.ID,
		Tags:    tags,
		Poll:    poll,
		Survey:  survey,
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
	} else {
		publishPostEffects(&post)
	}

	utils.RespondSuccess(c, gin.H{
		"message": message,
		"id":      post.ID,
		"status":  post.Status,
	})
}

// UpdatePost 作者修改自己的帖子，最多 maxForumPostEdits 次，旧内容存入历史版本
func UpdatePost(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	var req struct {
		Text string   `json:"text"`
		Tags []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	post, err := db.GetForumPostByID(pid)
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)
	if post.UserID != currentUser.ID {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权修改该帖子"))
		return
	}
	if post.EditCount >= maxForumPostEdits {
		utils.RespondError(c, 403, "EditLimitExceeded", fmt.Errorf("帖子最多修改%d次", maxForumPostEdits))
		return
	}
	if _, blocked := moderation.MatchSensitiveWord(req.Text); blocked {
		utils.RespondError(c, 403, "SensitiveContentRejected", errors.New("帖子包含敏感词，已被直接拒绝"))
		return
	}

	status := initialForumContentStatus()
	if err := db.EditForumPost(post, req.Text, req.Tags, status); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	message := "帖子修改成功"
	if status == model.ForumContentStatusPending {
		moderation.EnqueuePost(post.ID)
		message = "帖子修改成功，等待审核"
	}

	utils.RespondSuccess(c, gin.H{
		"message":        message,
		"id":             post.ID,
		"status":         status,
		"edit_count":     post.EditCount + 1,
		"max_edit_count": maxForumPostEdits,
	})
}

// DeleteOwnPost 作者删除自己的帖子，删除后帖子对所有人不可见
func DeleteOwnPost(c *gin.Context) {
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

	currentUser := c.MustGet("CurrentUser").(*model.User)
	if post.UserID != currentUser.ID {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权删除该帖子"))
		return
	}

	if err := deletePostWithNotifications(post); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "帖子删除成功"})
}

// GetPostVersions 获取帖子的全部版本（含当前版本），对所有可见该帖子的用户开放
func GetPostVersions(c *gin.Context) {
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

	versions, err := db.GetForumPostVersions(post.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	versionData := make([]map[string]interface{}, 0, len(versions)+1)
	for _, version := range versions {
		versionData = append(versionData, map[string]interface{}{
			"version":    version.Version,
			"content":    version.Content,
			"text":       version.ContentHTML,
			"timestamp":  version.CreatedAt.Unix(),
			"is_current": false,
		})
	}

	currentTimestamp := post.CreatedAt
	if post.LastEditedAt != nil {
		currentTimestamp = *post.LastEditedAt
	}
	versionData = append(versionData, map[string]interface{}{
		"version":    post.EditCount + 1,
		"content":    post.Content,
		"text":       post.ContentHTML,
		"timestamp":  currentTimestamp.Unix(),
		"is_current": true,
	})

	utils.RespondSuccess(c, gin.H{
		"post_id":        post.ID,
		"edit_count":     post.EditCount,
		"max_edit_count": maxForumPostEdits,
		"versions":       versionData,
	})
}

func GetFollowedPosts(c *gin.Context) {
	limitStr := c.Query("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		utils.RespondError(c, 400, "InvalidLimit", err)
		return
	}

	commentLimit, err := strconv.Atoi(c.DefaultQuery("comment_limit", strconv.Itoa(defaultCommentLimit)))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}
	if commentLimit < 0 {
		utils.RespondError(c, 400, "InvalidParam", errors.New("comment_limit 不能为负数"))
		return
	}
	if commentLimit > maxCommentLimit {
		commentLimit = maxCommentLimit
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

	likedPostMap := make(map[uint]bool)
	if len(posts) > 0 {
		minID := posts[len(posts)-1].ID
		maxID := posts[0].ID
		if likes, err := db.GetLikedIDs(currentUser.ID, minID, maxID); err == nil {
			for _, postID := range likes {
				likedPostMap[postID] = true
			}
		}
	}

	postData := make([]map[string]interface{}, len(posts))
	for i, post := range posts {
		isLike := 0
		if likedPostMap[post.ID] {
			isLike = 1
		}
		postData[i] = forumPostSummary(post, 1, isLike)
	}

	if err := attachCommentPreviews(postData, posts, currentUser.ID, commentLimit); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	attachPolls(postData, posts, currentUser.ID)
	attachSurveys(postData, posts, currentUser)

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

		// 发送通知给帖子作者（同一用户只通知一次）
		notifyOnce(
			post.UserID,
			"forum_follow",
			fmt.Sprintf("用户 %s 关注了您的帖子", currentUser.Username),
			currentUser.ID,
			uint(postID),
			0,
		)

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

		// 发送通知给帖子作者（同一用户只通知一次）
		notifyOnce(
			post.UserID,
			"forum_like",
			fmt.Sprintf("用户 %s 点赞了您的帖子", currentUser.Username),
			currentUser.ID,
			uint(postID),
			0,
		)
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

		// 发送通知给评论作者（同一用户只通知一次）
		notifyOnce(
			comment.UserID,
			"comment_like",
			fmt.Sprintf("用户 %s 点赞了您的评论", currentUser.Username),
			currentUser.ID,
			comment.PostID,
			uint(commentID),
		)
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

	if req.Status == model.ForumContentStatusApproved {
		if post, err := db.GetForumPostByID(int(postID)); err == nil {
			publishPostEffects(post)
		}
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

	if req.Status == model.ForumContentStatusApproved {
		publishCommentEffects(comment)
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

// deletePostWithNotifications 删除帖子并通知其关注者与评论者
func deletePostWithNotifications(post *model.ForumPost) error {
	// 先获取通知目标，再删除帖子。删除后关联关系会被清理，无法再查到关注/评论用户。
	followers, followerErr := db.GetPostFollowers(post.ID)
	commenters, commenterErr := db.GetPostCommenters(post.ID)

	if err := db.DeleteForumPostByID(post.ID); err != nil {
		return err
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

	return nil
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

	if err := deletePostWithNotifications(post); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "帖子删除成功"})
}

// deleteCommentWithNotifications 删除评论并通知相关评论者
func deleteCommentWithNotifications(comment *model.ForumComment) error {
	if err := db.DeleteForumCommentByID(comment.ID); err != nil {
		return err
	}

	// 发送通知给回复这条评论的用户（即引用了此评论的评论者）
	replyingComments, err := db.GetCommentsQuotingComment(comment.ID)
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

	return nil
}

// DeleteOwnComment 作者删除自己的评论
func DeleteOwnComment(c *gin.Context) {
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

	currentUser := c.MustGet("CurrentUser").(*model.User)
	if comment.UserID != currentUser.ID {
		utils.RespondError(c, 403, "Forbidden", errors.New("无权删除该评论"))
		return
	}

	if err := deleteCommentWithNotifications(comment); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "评论删除成功"})
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

	if err := deleteCommentWithNotifications(comment); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
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
		"id":             post.ID,
		"content":        post.Content,
		"timestamp":      post.CreatedAt.Unix(),
		"status":         post.Status,
		"userid":         post.User.ID,
		"username":       post.User.Username,
		"edit_count":     post.EditCount,
		"max_edit_count": maxForumPostEdits,
	}

	utils.RespondSuccess(c, rawData)
}

// parsePostQuoteIDs 解析引用卡片的 ids 参数（逗号分隔）：去重、跳过非法值、限制数量
func parsePostQuoteIDs(raw string) []uint {
	ids := make([]uint, 0, 8)
	seen := make(map[uint]bool)

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil || value == 0 {
			continue
		}

		id := uint(value)
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)

		if len(ids) >= maxPostQuoteIDs {
			break
		}
	}

	return ids
}

// GetPostQuotes 批量返回被引用帖子的作者与摘要，供正文里的引用卡片渲染
func GetPostQuotes(c *gin.Context) {
	ids := parsePostQuoteIDs(c.Query("ids"))
	if len(ids) == 0 {
		utils.RespondSuccess(c, []map[string]interface{}{})
		return
	}

	viewerID := c.MustGet("CurrentUser").(*model.User).ID

	posts, err := db.GetForumPostsByIDsForViewer(ids, viewerID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	visible := make(map[uint]model.ForumPost, len(posts))
	for _, post := range posts {
		visible[post.ID] = post
	}

	result := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		post, ok := visible[id]
		if !ok || post.User == nil {
			result = append(result, map[string]interface{}{
				"id":        id,
				"available": false,
			})
			continue
		}

		result = append(result, map[string]interface{}{
			"id":        post.ID,
			"available": true,
			"userid":    post.User.ID,
			"username":  post.User.Username,
			"text":      post.ContentHTML,
			"excerpt":   utils.TruncateString(strings.Join(strings.Fields(post.ContentText), " "), postQuoteExcerptLength),
			"timestamp": post.CreatedAt.Unix(),
		})
	}

	utils.RespondSuccess(c, result)
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
