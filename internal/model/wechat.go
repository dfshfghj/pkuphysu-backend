package model

type WechatArticle struct {
	ID          uint `gorm:"primaryKey"`
	Title       string
	Description string
	MpName      string `gorm:"column:mp_name"`
	URL         string `gorm:"column:url"`
	PublishTime int64  `gorm:"column:publish_time"`
}

type WechatCookie struct {
	ID      uint `gorm:"primaryKey"`
	Name    string
	Value   string
	Domain  string
	Path    string
	Expires int64
}
