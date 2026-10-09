package db

import (
	"errors"

	"pkuphysu-backend/internal/model"

	"gorm.io/gorm"
)

// ErrAlreadySurveyed 表示用户在该问卷下已有答卷（且问卷不允许重复提交）
var ErrAlreadySurveyed = errors.New("already surveyed")

func orderedSurveyBlocks(tx *gorm.DB) *gorm.DB {
	return tx.Order("position ASC, id ASC")
}

// GetForumSurveyByID 获取问卷（含按顺序排列的区块）
func GetForumSurveyByID(surveyID uint) (*model.ForumSurvey, error) {
	var survey model.ForumSurvey
	err := db.Preload("Blocks", orderedSurveyBlocks).Where("id = ?", surveyID).First(&survey).Error
	return &survey, err
}

// GetForumSurveyByPostID 获取帖子附带的问卷（含区块）
func GetForumSurveyByPostID(postID uint) (*model.ForumSurvey, error) {
	var survey model.ForumSurvey
	err := db.Preload("Blocks", orderedSurveyBlocks).Where("post_id = ?", postID).First(&survey).Error
	return &survey, err
}

// GetForumSurveysByPostIDs 批量获取帖子的问卷，供帖子列表摘要复用，避免 N+1
func GetForumSurveysByPostIDs(postIDs []uint) (map[uint]*model.ForumSurvey, error) {
	result := make(map[uint]*model.ForumSurvey)
	if len(postIDs) == 0 {
		return result, nil
	}

	var surveys []model.ForumSurvey
	if err := db.Preload("Blocks", orderedSurveyBlocks).Where("post_id IN ?", postIDs).Find(&surveys).Error; err != nil {
		return nil, err
	}
	for i := range surveys {
		result[surveys[i].PostID] = &surveys[i]
	}
	return result, nil
}

// CreateForumSurvey 创建问卷及其区块
func CreateForumSurvey(survey *model.ForumSurvey) error {
	return db.Create(survey).Error
}

// GetSurveyResponseCounts 批量统计问卷的答卷数
func GetSurveyResponseCounts(surveyIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64)
	if len(surveyIDs) == 0 {
		return counts, nil
	}

	type row struct {
		SurveyID uint
		Total    int64
	}
	var rows []row
	if err := db.Model(&model.ForumSurveyResponse{}).
		Select("survey_id, COUNT(*) AS total").
		Where("survey_id IN ?", surveyIDs).
		Group("survey_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		counts[r.SurveyID] = r.Total
	}
	return counts, nil
}

// GetSurveyResponseCount 统计单个问卷的答卷数
func GetSurveyResponseCount(surveyID uint) (int64, error) {
	var count int64
	err := db.Model(&model.ForumSurveyResponse{}).Where("survey_id = ?", surveyID).Count(&count).Error
	return count, err
}

// HasUserSurveyResponse 判断用户是否已提交过该问卷
func HasUserSurveyResponse(surveyID, userID uint) (bool, error) {
	var count int64
	err := db.Model(&model.ForumSurveyResponse{}).
		Where("survey_id = ? AND user_id = ?", surveyID, userID).
		Count(&count).Error
	return count > 0, err
}

// CreateSurveyResponse 记录一次答卷及其答案；问卷不允许重复提交时，已有记录返回 ErrAlreadySurveyed
func CreateSurveyResponse(survey *model.ForumSurvey, userID uint, answers []model.ForumSurveyAnswer) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if !survey.AllowMultiple {
			var count int64
			if err := tx.Model(&model.ForumSurveyResponse{}).
				Where("survey_id = ? AND user_id = ?", survey.ID, userID).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrAlreadySurveyed
			}
		}

		response := model.ForumSurveyResponse{SurveyID: survey.ID, UserID: userID}
		if err := tx.Create(&response).Error; err != nil {
			return err
		}

		for i := range answers {
			answers[i].ResponseID = response.ID
		}
		if len(answers) == 0 {
			return nil
		}
		return tx.Create(&answers).Error
	})
}

// GetSurveyResponses 获取问卷的全部答卷（含答案与答题人），供统计与导出
func GetSurveyResponses(surveyID uint) ([]model.ForumSurveyResponse, error) {
	var responses []model.ForumSurveyResponse
	err := db.Preload("Answers").Preload("User").
		Where("survey_id = ?", surveyID).
		Order("id ASC").
		Find(&responses).Error
	return responses, err
}

// UpdateSurveyMeta 更新问卷的开关与限答设置
func UpdateSurveyMeta(surveyID uint, status string, allowMultiple bool) error {
	return db.Model(&model.ForumSurvey{}).Where("id = ?", surveyID).Updates(map[string]interface{}{
		"status":         status,
		"allow_multiple": allowMultiple,
	}).Error
}

// ReplaceSurveyBlocks 用新的区块列表替换问卷的全部区块（仅在问卷尚无答卷时调用）
func ReplaceSurveyBlocks(surveyID uint, blocks []model.ForumSurveyBlock) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("survey_id = ?", surveyID).Delete(&model.ForumSurveyBlock{}).Error; err != nil {
			return err
		}
		for i := range blocks {
			blocks[i].ID = 0
			blocks[i].SurveyID = surveyID
		}
		if len(blocks) == 0 {
			return nil
		}
		return tx.Create(&blocks).Error
	})
}
