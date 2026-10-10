package handles

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	minPollOptions      = 2
	maxPollOptions      = 10
	maxPollOptionLength = 100
)

// normalizePollInput 清洗投票入参：去空白、去重，并校验选项数量与长度
func normalizePollInput(multiple bool, options []string) (bool, []string, error) {
	normalized := make([]string, 0, len(options))
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		text := strings.TrimSpace(option)
		if text == "" {
			continue
		}
		if utf8.RuneCountInString(text) > maxPollOptionLength {
			return false, nil, fmt.Errorf("选项最多 %d 个字符", maxPollOptionLength)
		}
		if seen[text] {
			continue
		}
		seen[text] = true
		normalized = append(normalized, text)
	}

	if len(normalized) < minPollOptions {
		return false, nil, fmt.Errorf("投票至少需要 %d 个选项", minPollOptions)
	}
	if len(normalized) > maxPollOptions {
		return false, nil, fmt.Errorf("投票最多 %d 个选项", maxPollOptions)
	}
	return multiple, normalized, nil
}

func pollPercent(count int64, voterCount int64) int {
	if voterCount <= 0 {
		return 0
	}
	return int(math.Round(float64(count) / float64(voterCount) * 100))
}

// buildPollPayload 构建投票响应；占比仅在 can_see_results（已投票或作者）时下发
func buildPollPayload(view *db.ForumPollView) map[string]interface{} {
	if view == nil || view.Poll == nil {
		return nil
	}

	options := make([]map[string]interface{}, 0, len(view.Poll.Options))
	for _, option := range view.Poll.Options {
		item := map[string]interface{}{
			"id":   option.ID,
			"text": option.Text,
		}
		if view.CanSeeResults {
			count := view.Counts[option.ID]
			item["count"] = count
			item["percent"] = pollPercent(count, view.VoterCount)
		}
		if view.Voted {
			item["selected"] = view.Selected[option.ID]
		}
		options = append(options, item)
	}

	payload := map[string]interface{}{
		"id":              view.Poll.ID,
		"multiple":        view.Poll.Multiple,
		"voted":           view.Voted,
		"can_see_results": view.CanSeeResults,
		"options":         options,
	}
	if view.CanSeeResults {
		payload["total_votes"] = view.VoterCount
	}
	return payload
}

// resolveVoteOptionIDs 校验并去重投票选项：必须存在、非空，单选只能选一个
func resolveVoteOptionIDs(poll *model.ForumPoll, requested []uint) ([]uint, error) {
	if poll == nil {
		return nil, errors.New("投票不存在")
	}

	valid := make(map[uint]bool, len(poll.Options))
	for _, option := range poll.Options {
		valid[option.ID] = true
	}

	seen := make(map[uint]bool, len(requested))
	result := make([]uint, 0, len(requested))
	for _, optionID := range requested {
		if optionID == 0 || seen[optionID] {
			continue
		}
		if !valid[optionID] {
			return nil, errors.New("选项不属于该投票")
		}
		seen[optionID] = true
		result = append(result, optionID)
	}

	if len(result) == 0 {
		return nil, errors.New("请选择至少一个选项")
	}
	if !poll.Multiple && len(result) != 1 {
		return nil, errors.New("该投票只能选择一个选项")
	}
	return result, nil
}

// attachPolls 把投票数据按帖子顺序注入响应，无投票的帖子置为 null
func attachPolls(postData []map[string]interface{}, posts []model.ForumPost, viewerID uint) {
	if len(postData) == 0 || len(posts) == 0 {
		return
	}

	postIDs := make([]uint, len(posts))
	for i, post := range posts {
		postIDs[i] = post.ID
	}

	views, err := db.GetForumPollViews(postIDs, viewerID)
	if err != nil {
		logrus.WithError(err).Warn("failed to load forum polls")
		return
	}

	for i := range postData {
		if i >= len(posts) {
			break
		}
		if view, ok := views[posts[i].ID]; ok {
			postData[i]["poll"] = buildPollPayload(view)
		} else {
			postData[i]["poll"] = nil
		}
	}
}

func VotePost(c *gin.Context) {
	pid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondError(c, 400, "InvalidParam", err)
		return
	}

	var req struct {
		OptionIDs []uint `json:"option_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	post, err := db.GetForumPostByID(pid)
	if err != nil {
		utils.RespondError(c, 404, "NotFound", err)
		return
	}
	if !canViewForumContent(post.Status, post.UserID, currentUser.ID) {
		utils.RespondError(c, 404, "NotFound", nil)
		return
	}

	poll, err := db.GetForumPollByPostID(post.ID)
	if err != nil {
		utils.RespondError(c, 404, "PollNotFound", err)
		return
	}

	optionIDs, err := resolveVoteOptionIDs(poll, req.OptionIDs)
	if err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	if err := db.CreatePollVotes(poll.ID, currentUser.ID, optionIDs); err != nil {
		if errors.Is(err, db.ErrAlreadyVoted) {
			utils.RespondError(c, 409, "AlreadyVoted", errors.New("您已经投过票了"))
			return
		}
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	views, err := db.GetForumPollViews([]uint{post.ID}, currentUser.ID)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"message": "投票成功",
		"poll":    buildPollPayload(views[post.ID]),
	})
}
