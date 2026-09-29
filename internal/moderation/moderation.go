package moderation

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"

	"github.com/sirupsen/logrus"
)

type taskKind string

const (
	postTask    taskKind = "post"
	commentTask taskKind = "comment"
)

type task struct {
	Kind taskKind
	ID   uint
}

type client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type manager struct {
	client        *client
	queue         chan task
	retryInterval time.Duration
	batchSize     int
}

type moderationRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type moderationResponse struct {
	ResultList []struct {
		RiskLevel string `json:"risk_level"`
	} `json:"result_list"`
}

var defaultManager *manager
var sensitiveMatcher matcher

func Init() {
	cfg := config.Conf.Moderation
	sensitiveMatcher = newMatcher(cfg.SensitiveWords)

	if !cfg.Enabled {
		return
	}
	if cfg.BaseURL == "" || cfg.Token == "" {
		logrus.Warn("moderation enabled but base URL or token is missing; worker not started")
		return
	}

	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 128
	}
	retryInterval := cfg.RetryInterval
	if retryInterval <= 0 {
		retryInterval = 30 * time.Second
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	defaultManager = &manager{
		client: &client{
			baseURL: cfg.BaseURL,
			token:   cfg.Token,
			httpClient: &http.Client{
				Timeout: timeout,
			},
		},
		queue:         make(chan task, queueSize),
		retryInterval: retryInterval,
		batchSize:     batchSize,
	}

	go defaultManager.runQueue()
	go defaultManager.runRetryLoop()
}

func EnqueuePost(id uint) {
	enqueue(task{Kind: postTask, ID: id})
}

func EnqueueComment(id uint) {
	enqueue(task{Kind: commentTask, ID: id})
}

func MatchSensitiveWord(input string) (string, bool) {
	return sensitiveMatcher.match(input)
}

func enqueue(t task) {
	if defaultManager == nil {
		return
	}

	select {
	case defaultManager.queue <- t:
	default:
		logrus.WithFields(logrus.Fields{"kind": t.Kind, "id": t.ID}).Warn("moderation queue full; waiting for retry scan")
	}
}

func (m *manager) runQueue() {
	for t := range m.queue {
		if err := m.processTask(t); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"kind": t.Kind, "id": t.ID}).Warn("moderation task failed")
		}
	}
}

func (m *manager) runRetryLoop() {
	ticker := time.NewTicker(m.retryInterval)
	defer ticker.Stop()

	for range ticker.C {
		posts, err := db.ListPendingForumPosts(m.batchSize)
		if err != nil {
			logrus.WithError(err).Warn("failed to list pending forum posts")
			continue
		}
		for _, post := range posts {
			enqueue(task{Kind: postTask, ID: post.ID})
		}

		comments, err := db.ListPendingForumComments(m.batchSize)
		if err != nil {
			logrus.WithError(err).Warn("failed to list pending forum comments")
			continue
		}
		for _, comment := range comments {
			enqueue(task{Kind: commentTask, ID: comment.ID})
		}
	}
}

func (m *manager) processTask(t task) error {
	switch t.Kind {
	case postTask:
		return m.processPost(t.ID)
	case commentTask:
		return m.processComment(t.ID)
	default:
		return errors.New("unknown moderation task kind")
	}
}

func (m *manager) processPost(postID uint) error {
	post, err := db.GetForumPostByID(int(postID))
	if err != nil {
		return err
	}
	if post.Status != model.ForumContentStatusPending {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"kind":    postTask,
		"id":      post.ID,
		"user_id": post.UserID,
		"content": post.ContentText,
	}).Info("processing moderation task")

	nextStatus, err := m.reviewStatus(post.ContentText)
	if err != nil {
		return err
	}

	changed, err := db.TransitionForumPostStatus(postID, model.ForumContentStatusPending, nextStatus)
	if err != nil || !changed {
		return err
	}

	if nextStatus != model.ForumContentStatusApproved {
		notifyModerationFailure(post.UserID, post.ID, 0, "您的帖子未通过自动审核，已转入管理员复核")
	}

	return nil
}

func (m *manager) processComment(commentID uint) error {
	comment, err := db.GetForumCommentByID(commentID)
	if err != nil {
		return err
	}
	if comment.Status != model.ForumContentStatusPending {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"kind":    commentTask,
		"id":      comment.ID,
		"user_id": comment.UserID,
		"post_id": comment.PostID,
		"content": comment.ContentText,
	}).Info("processing moderation task")

	nextStatus, err := m.reviewStatus(comment.ContentText)
	if err != nil {
		return err
	}

	changed, err := db.TransitionForumCommentStatus(commentID, model.ForumContentStatusPending, nextStatus)
	if err != nil || !changed {
		return err
	}

	if nextStatus != model.ForumContentStatusApproved {
		notifyModerationFailure(comment.UserID, comment.PostID, comment.ID, "您的评论未通过自动审核，已转入管理员复核")
		return nil
	}

	approvedCount, err := db.CountApprovedComments(comment.PostID)
	if err != nil {
		return err
	}
	if err := db.UpdateForumPostReplyNum(comment.PostID, int(approvedCount)); err != nil {
		return err
	}

	post, err := db.GetForumPostByID(int(comment.PostID))
	if err != nil {
		return err
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

	return nil
}

func (m *manager) reviewStatus(input string) (string, error) {
	logrus.WithFields(logrus.Fields{
		"content": input,
	}).Info("submitting content for moderation")

	resp, err := m.client.moderate(input)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"content": input,
		}).Warn("moderation request failed")
		return "", err
	}

	status, err := evaluateRiskLevel(resp)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"content": input,
		}).Warn("failed to evaluate moderation response")
		return "", err
	}

	logrus.WithFields(logrus.Fields{
		"content":    input,
		"risk_level": resp.ResultList[0].RiskLevel,
		"status":     status,
	}).Info("moderation completed")

	return status, nil
}

func (c *client) moderate(input string) (*moderationResponse, error) {
	body, err := json.Marshal(moderationRequest{
		Model: "moderation",
		Input: input,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("moderation request failed")
	}

	var payload moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func evaluateRiskLevel(resp *moderationResponse) (string, error) {
	if resp == nil || len(resp.ResultList) == 0 {
		return "", errors.New("empty moderation result")
	}
	if strings.EqualFold(resp.ResultList[0].RiskLevel, "PASS") {
		return model.ForumContentStatusApproved, nil
	}
	return model.ForumContentStatusManualReview, nil
}

func notifyModerationFailure(userID uint, postID uint, commentID uint, content string) {
	if userID == 0 {
		return
	}

	notificationType := "forum_post_moderation"
	title := "post_manual_review"
	if commentID != 0 {
		notificationType = "forum_comment_moderation"
		title = "comment_manual_review"
	}

	if err := db.CreateNotification(&model.Notification{
		UserID:           userID,
		Title:            title,
		Content:          content,
		Type:             notificationType,
		Read:             false,
		RelatedPostID:    postID,
		RelatedCommentID: commentID,
	}); err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"user_id":    userID,
			"post_id":    postID,
			"comment_id": commentID,
		}).Warn("failed to create moderation notification")
	}
}

type matcher struct {
	words []string
}

func newMatcher(words []string) matcher {
	normalized := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(strings.ToLower(word))
		if word == "" {
			continue
		}
		normalized = append(normalized, word)
	}
	return matcher{words: normalized}
}

func (m matcher) match(input string) (string, bool) {
	if len(m.words) == 0 {
		return "", false
	}

	normalizedInput := strings.ToLower(input)
	for _, word := range m.words {
		if strings.Contains(normalizedInput, word) {
			return word, true
		}
	}
	return "", false
}
