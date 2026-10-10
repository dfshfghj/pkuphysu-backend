package model

import "time"

const (
	SurveyStatusOpen   = "open"
	SurveyStatusClosed = "closed"

	SurveyBlockMarkdown = "markdown"
	SurveyBlockQuestion = "question"

	SurveyQuestionSingle     = "single"
	SurveyQuestionMultiple   = "multiple"
	SurveyQuestionText       = "text"
	SurveyQuestionRanking    = "ranking"
	SurveyQuestionSlider     = "slider"
	SurveyQuestionProportion = "proportion"
	SurveyQuestionScale      = "scale"
)

type ForumSurvey struct {
	ID            uint   `gorm:"primaryKey"`
	PostID        uint   `gorm:"uniqueIndex;constraint:OnDelete:CASCADE;"`
	UserID        uint   `gorm:"index"`
	Status        string `gorm:"type:varchar(16);default:'open'"`
	AllowMultiple bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Blocks        []ForumSurveyBlock `gorm:"foreignKey:SurveyID"`
}

type ForumSurveyBlock struct {
	ID          uint   `gorm:"primaryKey"`
	SurveyID    uint   `gorm:"index;constraint:OnDelete:CASCADE;"`
	Kind        string `gorm:"type:varchar(16)"`
	Content     string `gorm:"type:text"`
	ContentHTML string `gorm:"type:text"`
	Type        string `gorm:"type:varchar(24)"`
	Required    bool
	Position    int
	Config      string `gorm:"type:jsonb"`
	CreatedAt   time.Time
}

type ForumSurveyResponse struct {
	ID        uint `gorm:"primaryKey"`
	SurveyID  uint `gorm:"index;constraint:OnDelete:CASCADE;"`
	UserID    uint `gorm:"index;constraint:OnDelete:CASCADE;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	User      *User               `gorm:"foreignKey:UserID"`
	Answers   []ForumSurveyAnswer `gorm:"foreignKey:ResponseID"`
}

type ForumSurveyAnswer struct {
	ID         uint   `gorm:"primaryKey"`
	ResponseID uint   `gorm:"index;constraint:OnDelete:CASCADE;"`
	BlockID    uint   `gorm:"index;constraint:OnDelete:CASCADE;"`
	Value      string `gorm:"type:jsonb"`
	CreatedAt  time.Time
}
