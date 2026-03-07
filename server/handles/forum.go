package handles

import (
	"strconv"

	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
)

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

	isFollow := 0
	followed, err := db.GetUserFollowStatus(userID, uint(pid))
	if err == nil && followed {
		isFollow = 1
	}

	isLike := 0
	liked, err := db.GetUserLikeStatus(userID, uint(pid))
	if err == nil && liked {
		isLike = 1
	}

	postData := map[string]interface{}{
		"id":        post.ID,
		"text":      post.Content,
		"timestamp": post.CreatedAt.Unix(),
		"follownum": post.Follownum,
		"likenum":   post.Likenum,
		"reply":     post.Reply,
		"tag":       post.Tag,
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

	tag := c.DefaultQuery("tag", "")
	keyword := c.DefaultQuery("keyword", "")

	var posts []model.ForumPost

	posts, err = db.GetForumPosts(cursor, limit, tag, keyword)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID

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

		postData[i] = map[string]interface{}{
			"id":        post.ID,
			"text":      post.Content,
			"type":      post.Type,
			"timestamp": post.CreatedAt.Unix(),
			"follownum": post.Follownum,
			"likenum":   post.Likenum,
			"reply":     post.Reply,
			"tag":       post.Tag,
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

	comments, err := db.GetForumComments(id, cursor, limit, sort)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	userID := c.MustGet("CurrentUser").(*model.User).ID

	commentData := make([]map[string]interface{}, len(comments))
	for i, comment := range comments {
		quoteInfo := gin.H{}
		if comment.Quote != nil {
			quoteInfo = gin.H{
				"cid":      comment.Quote.ID,
				"username": comment.Quote.User.Username,
				"text":     comment.Quote.Content,
			}
		} else {
			quoteInfo = nil
		}

		// 检查当前用户是否点赞了该评论
		isLike := 0
		liked, err := db.GetUserCommentLikeStatus(userID, comment.ID)
		if err == nil && liked {
			isLike = 1
		}

		commentData[i] = map[string]interface{}{
			"cid":       comment.ID,
			"pid":       comment.PostID,
			"text":      comment.Content,
			"quote":     quoteInfo,
			"timestamp": comment.CreatedAt.Unix(),
			"userid":    comment.User.ID,
			"username":  comment.User.Username,
			"likenum":   comment.Likenum,
			"is_like":   isLike,
		}
	}

	utils.RespondSuccess(c, commentData)
}

func SubmitComment(c *gin.Context) {
	var req struct {
		Pid   uint `json:"pid"`
		Text  string `json:"text"`
		Quote *uint  `json:"quote"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	comment := model.ForumComment{
		Content: req.Text,
		Likenum: 0, // 初始化点赞数量为0
		PostID:  0,
		UserID:  currentUser.ID,
	}

	if req.Quote != nil {
		comment.QuoteID = req.Quote
	}

	comment.PostID = uint(req.Pid)

	err := db.CreateForumComment(&comment)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	// 获取当前帖子的 Reply 并加1
	post, err := db.GetForumPostByID(int(req.Pid))
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}
	post.Reply += 1
	err = db.UpdateForumPostReplyNum(uint(req.Pid), post.Reply)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "评论提交成功"})
}

func SubmitPost(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
		Tag  string `json:"tag"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, 400, "InvalidParams", err)
		return
	}

	currentUser := c.MustGet("CurrentUser").(*model.User)

	post := model.ForumPost{
		Content: req.Text,
		UserID:  currentUser.ID,
		Tag:     req.Tag,
	}

	err := db.CreateForumPost(&post)
	if err != nil {
		utils.RespondError(c, 500, "ServerError", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"message": "帖子发布成功"})
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
			postData[i] = map[string]interface{}{
			"id":        post.ID,
			"text":      post.Content,
			"type":      post.Type,
			"timestamp": post.CreatedAt.Unix(),
			"follownum": post.Follownum,
			"likenum":   post.Likenum,
			"reply":     post.Reply,
			"tag":       post.Tag,
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
		"message":   message,
		"likenum":   newLikenum,
		"is_liked":  !isLiked,
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
		"message":   message,
		"likenum":   newLikenum,
		"is_liked":  !isLiked,
	})
}