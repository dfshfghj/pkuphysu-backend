package handles

import (
	"errors"
	"strconv"
	"strings"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	defaultForumReportThreshold = 3
	forumReportReasonMaxLen     = 128
	forumReportDetailMaxLen     = 1000
	adminReportReasonLimit      = 5
)

type forumReportPayload struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

func forumReportThreshold() int {
	if config.Conf == nil || config.Conf.Moderation.ReportThreshold <= 0 {
		return defaultForumReportThreshold
	}
	return config.Conf.Moderation.ReportThreshold
}

func normalizeForumReportInput(req forumReportPayload) (forumReportPayload, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	req.Detail = strings.TrimSpace(req.Detail)

	switch {
	case req.Reason == "":
		return req, errors.New("举报理由不能为空")
	case len(req.Reason) > forumReportReasonMaxLen:
		return req, errors.New("举报理由过长")
	case len(req.Detail) > forumReportDetailMaxLen:
		return req, errors.New("举报补充说明过长")
	default:
		return req, nil
	}
}

func maybeEscalateReportedContent(targetType string, targetID uint, currentStatus string, ownerID uint, postID uint, commentID uint) (string, bool, error) {
	if currentStatus != model.ForumContentStatusApproved {
		return currentStatus, false, nil
	}

	nextStatus := model.ForumContentStatusManualReview
	var err error
	switch targetType {
	case model.ForumReportTargetPost:
		err = db.SetForumPostStatus(targetID, nextStatus)
	case model.ForumReportTargetComment:
		err = db.SetForumCommentStatus(targetID, nextStatus)
	default:
		return currentStatus, false, errors.New("invalid report target")
	}
	if err != nil {
		return currentStatus, false, err
	}

	content := "您的内容因被多次举报，已转入管理员复核"
	notificationType := "forum_post_report"
	title := "post_reported"
	if targetType == model.ForumReportTargetComment {
		notificationType = "forum_comment_report"
		title = "comment_reported"
	}

	_ = db.CreateNotification(&model.Notification{
		UserID:           ownerID,
		Title:            title,
		Content:          content,
		Type:             notificationType,
		Read:             false,
		RelatedPostID:    postID,
		RelatedCommentID: commentID,
	})

	return nextStatus, true, nil
}

func reportPost(c *gin.Context, post *model.ForumPost, req forumReportPayload) {
	currentUser := c.MustGet("CurrentUser").(*model.User)
	if post.UserID == currentUser.ID {
		utils.RespondError(c, 400, "InvalidOperation", errors.New("不能举报自己的帖子"))
		return
	}

	alreadyReported, err := db.HasPendingForumReportByUser(currentUser.ID, model.ForumReportTargetPost, post.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	if alreadyReported {
		utils.RespondError(c, 409, "DuplicateReport", errors.New("您已举报过该帖子"))
		return
	}

	postID := post.ID
	report := model.ForumReport{
		TargetType: model.ForumReportTargetPost,
		TargetID:   post.ID,
		ReporterID: currentUser.ID,
		Reason:     req.Reason,
		Detail:     req.Detail,
		PostID:     &postID,
	}
	if err := db.CreateForumReport(&report); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	reportCount, err := db.CountPendingForumReports(model.ForumReportTargetPost, post.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	finalStatus := post.Status
	thresholdReached := int(reportCount) >= forumReportThreshold()
	escalated := false
	if thresholdReached {
		finalStatus, escalated, err = maybeEscalateReportedContent(model.ForumReportTargetPost, post.ID, post.Status, post.UserID, post.ID, 0)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
	}

	utils.RespondSuccess(c, gin.H{
		"message":           "举报已提交",
		"report_id":         report.ID,
		"report_count":      reportCount,
		"threshold":         forumReportThreshold(),
		"threshold_reached": thresholdReached,
		"status":            finalStatus,
		"escalated":         escalated,
	})
}

func ReportPost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	var req forumReportPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}
	req, err = normalizeForumReportInput(req)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)
	post, err := db.GetForumPostByID(int(id))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if !canViewForumContent(post.Status, post.UserID, currentUser.ID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}

	reportPost(c, post, req)
}

func ReportComment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.RespondError(c, 400, "InvalidID", err)
		return
	}

	var req forumReportPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}
	req, err = normalizeForumReportInput(req)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)
	comment, err := db.GetForumCommentByID(uint(id))
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if !canViewForumContent(comment.Status, comment.UserID, currentUser.ID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}
	if comment.UserID == currentUser.ID {
		utils.RespondError(c, 400, "InvalidOperation", errors.New("不能举报自己的评论"))
		return
	}

	alreadyReported, err := db.HasPendingForumReportByUser(currentUser.ID, model.ForumReportTargetComment, comment.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	if alreadyReported {
		utils.RespondError(c, 409, "DuplicateReport", errors.New("您已举报过该评论"))
		return
	}

	postID := comment.PostID
	commentID := comment.ID
	report := model.ForumReport{
		TargetType: model.ForumReportTargetComment,
		TargetID:   comment.ID,
		ReporterID: currentUser.ID,
		Reason:     req.Reason,
		Detail:     req.Detail,
		PostID:     &postID,
		CommentID:  &commentID,
	}
	if err := db.CreateForumReport(&report); err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	reportCount, err := db.CountPendingForumReports(model.ForumReportTargetComment, comment.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	finalStatus := comment.Status
	thresholdReached := int(reportCount) >= forumReportThreshold()
	escalated := false
	if thresholdReached {
		finalStatus, escalated, err = maybeEscalateReportedContent(model.ForumReportTargetComment, comment.ID, comment.Status, comment.UserID, comment.PostID, comment.ID)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}
	}

	utils.RespondSuccess(c, gin.H{
		"message":           "举报已提交",
		"report_id":         report.ID,
		"report_count":      reportCount,
		"threshold":         forumReportThreshold(),
		"threshold_reached": thresholdReached,
		"status":            finalStatus,
		"escalated":         escalated,
	})
}

func ListForumReports(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		utils.RespondError(c, 400, "InvalidLimit", err)
		return
	}

	groups, err := db.ListPendingForumReportGroups(limit)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	items := make([]gin.H, 0, len(groups))
	for _, group := range groups {
		reports, err := db.GetForumReportsByTarget(group.TargetType, group.TargetID, adminReportReasonLimit)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		reasons := make([]gin.H, 0, len(reports))
		for _, report := range reports {
			reporterName := ""
			if report.Reporter != nil {
				reporterName = report.Reporter.Username
			}
			reasons = append(reasons, gin.H{
				"report_id":   report.ID,
				"reason":      report.Reason,
				"detail":      report.Detail,
				"reporter_id": report.ReporterID,
				"reporter":    reporterName,
				"timestamp":   report.CreatedAt.Unix(),
			})
		}

		item := gin.H{
			"target_type":        group.TargetType,
			"target_id":          group.TargetID,
			"report_count":       group.ReportCount,
			"latest_reported_at": group.LatestReportedAt.Unix(),
			"reasons":            reasons,
		}

		switch group.TargetType {
		case model.ForumReportTargetPost:
			post, err := db.GetForumPostByID(int(group.TargetID))
			if err != nil {
				continue
			}
			item["post_id"] = post.ID
			item["status"] = post.Status
			item["author_id"] = post.UserID
			item["author"] = post.User.Username
			item["preview"] = utils.TruncateString(post.ContentText, 160)
		case model.ForumReportTargetComment:
			comment, err := db.GetForumCommentByID(group.TargetID)
			if err != nil {
				continue
			}
			item["post_id"] = comment.PostID
			item["comment_id"] = comment.ID
			item["status"] = comment.Status
			item["author_id"] = comment.UserID
			item["author"] = comment.User.Username
			item["preview"] = utils.TruncateString(comment.ContentText, 160)
		default:
			continue
		}

		items = append(items, item)
	}

	utils.RespondSuccess(c, gin.H{
		"threshold": forumReportThreshold(),
		"items":     items,
	})
}
