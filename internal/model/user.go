package model

const (
	GENERAL = iota
	GUEST   // only one exists
	ADMIN
)

type User struct {
	ID       uint   `json:"id" gorm:"primaryKey"`                      // unique key
	Username string `json:"username" gorm:"unique" binding:"required"` // username
	Verified bool   `json:"verified"`                                  // whether the user has been verified
	Stuname  string `json:"stuname" gorm:"unique"`                     // student name
	Stuid    string `json:"stuid" gorm:"unique"`                       // student id
	PwdHash  string `json:"-"`                                         // password hash
	PwdTS    int64  `json:"-"`                                         // password timestamp
	Salt     string `json:"-"`                                         // unique salt
	Password string `json:"password"`                                  // password
	BasePath string `json:"base_path"`                                 // base path
	Role     int    `json:"role"`                                      // user's role
	Disabled bool   `json:"disabled"`
	Bio      string `json:"bio"`                                       // user's bio
}