package model

type Post struct {
	ID           uint   `gorm:"primaryKey"`
	Title        string
	Description  string
	MpName       string `gorm:"column:mp_name"`
	URL          string `gorm:"column:url"`
	PublishTime  int64  `gorm:"column:publish_time"`
}

func (Post) TableName() string {
	return "wechat_posts"
}