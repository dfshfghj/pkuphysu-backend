package db

import (
	"errors"

	"pkuphysu-backend/internal/model"

	"gorm.io/gorm"
)

// ErrAlreadyVoted 表示用户在该投票下已有投票记录（投票不可更改）
var ErrAlreadyVoted = errors.New("already voted")

// ForumPollView 是某个投票对某个查看者的呈现：是否已投票、能否看到占比、自己的选项与计数
type ForumPollView struct {
	Poll          *model.ForumPoll
	Voted         bool
	CanSeeResults bool
	Selected      map[uint]bool
	Counts        map[uint]int64
	VoterCount    int64
}

func orderedOptions(tx *gorm.DB) *gorm.DB {
	return tx.Order("position ASC, id ASC")
}

// GetForumPollByPostID 获取帖子的投票（含选项）
func GetForumPollByPostID(postID uint) (*model.ForumPoll, error) {
	var poll model.ForumPoll
	err := db.Preload("Options", orderedOptions).Where("post_id = ?", postID).First(&poll).Error
	return &poll, err
}

// GetForumPollViews 批量构建帖子投票的呈现，供帖子列表与详情复用，避免 N+1
func GetForumPollViews(postIDs []uint, viewerID uint) (map[uint]*ForumPollView, error) {
	views := make(map[uint]*ForumPollView)
	if len(postIDs) == 0 {
		return views, nil
	}

	var polls []model.ForumPoll
	if err := db.Preload("Options", orderedOptions).Where("post_id IN ?", postIDs).Find(&polls).Error; err != nil {
		return nil, err
	}
	if len(polls) == 0 {
		return views, nil
	}

	pollIDs := make([]uint, len(polls))
	for i := range polls {
		pollIDs[i] = polls[i].ID
	}

	authorByPost := make(map[uint]uint)
	var posts []model.ForumPost
	if err := db.Model(&model.ForumPost{}).Select("id", "user_id").Where("id IN ?", postIDs).Find(&posts).Error; err != nil {
		return nil, err
	}
	for _, post := range posts {
		authorByPost[post.ID] = post.UserID
	}

	selected := make(map[uint]map[uint]bool)
	votedPolls := make(map[uint]bool)
	var votes []model.ForumPollVote
	if err := db.Where("user_id = ? AND poll_id IN ?", viewerID, pollIDs).Find(&votes).Error; err != nil {
		return nil, err
	}
	for _, vote := range votes {
		if selected[vote.PollID] == nil {
			selected[vote.PollID] = make(map[uint]bool)
		}
		selected[vote.PollID][vote.OptionID] = true
		votedPolls[vote.PollID] = true
	}

	visiblePollIDs := make([]uint, 0, len(polls))
	for i := range polls {
		poll := &polls[i]
		if votedPolls[poll.ID] || authorByPost[poll.PostID] == viewerID {
			visiblePollIDs = append(visiblePollIDs, poll.ID)
		}
	}

	counts := make(map[uint]int64)
	voterCounts := make(map[uint]int64)
	if len(visiblePollIDs) > 0 {
		type countRow struct {
			OptionID uint
			Total    int64
		}
		var rows []countRow
		if err := db.Model(&model.ForumPollVote{}).
			Select("option_id, COUNT(*) AS total").
			Where("poll_id IN ?", visiblePollIDs).
			Group("option_id").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			counts[row.OptionID] = row.Total
		}

		type voterRow struct {
			PollID uint
			Total  int64
		}
		var voterRows []voterRow
		if err := db.Model(&model.ForumPollVote{}).
			Select("poll_id, COUNT(DISTINCT user_id) AS total").
			Where("poll_id IN ?", visiblePollIDs).
			Group("poll_id").
			Scan(&voterRows).Error; err != nil {
			return nil, err
		}
		for _, row := range voterRows {
			voterCounts[row.PollID] = row.Total
		}
	}

	for i := range polls {
		poll := &polls[i]
		view := &ForumPollView{
			Poll:          poll,
			Voted:         votedPolls[poll.ID],
			CanSeeResults: votedPolls[poll.ID] || authorByPost[poll.PostID] == viewerID,
			Selected:      selected[poll.ID],
		}
		if view.Selected == nil {
			view.Selected = make(map[uint]bool)
		}
		if view.CanSeeResults {
			view.Counts = counts
			view.VoterCount = voterCounts[poll.ID]
		} else {
			view.Counts = make(map[uint]int64)
		}
		views[poll.PostID] = view
	}

	return views, nil
}

// CreatePollVotes 记录一次投票；用户在同一投票下已有记录时返回 ErrAlreadyVoted
func CreatePollVotes(pollID, userID uint, optionIDs []uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.ForumPollVote{}).
			Where("poll_id = ? AND user_id = ?", pollID, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrAlreadyVoted
		}

		votes := make([]model.ForumPollVote, 0, len(optionIDs))
		for _, optionID := range optionIDs {
			votes = append(votes, model.ForumPollVote{
				PollID:   pollID,
				OptionID: optionID,
				UserID:   userID,
			})
		}
		return tx.Create(&votes).Error
	})
}
