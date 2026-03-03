package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"crypto/rand"

	"github.com/pkg/errors"
)

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
	Role     int    `json:"role"`                                      // user's role
	Disabled bool   `json:"disabled"`
	Bio      string `json:"bio"` // user's bio
}

func (u *User) IsGuest() bool {
	return u.Role == GUEST
}

func (u *User) IsAdmin() bool {
	return u.Role == ADMIN
}

func (u *User) SetPassword(pwdStaticHash string) *User {
	u.Salt, _ = RandomSalt(16)
	u.PwdHash = HashPassword(pwdStaticHash, u.Salt)
	u.PwdTS = time.Now().Unix()
	return u
}

func RandomSalt(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func HashPassword(pwdStaticHash string, salt string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", pwdStaticHash, salt)))
	return hex.EncodeToString(hash[:])
}

func (u *User) ValidatePassword(pwdStaticHash string) error {
	if u.PwdHash != HashPassword(pwdStaticHash, u.Salt) {
		return errors.New("invalid password")
	}
	return nil
}
