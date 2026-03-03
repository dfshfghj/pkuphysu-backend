package db

import (
	"pkuphysu-backend/internal/model"
	"time"

	"github.com/pkg/errors"
)

func CreateEmailVerification(email, code string) error {
	// 先删除该邮箱的所有现有验证码记录
	if err := db.Where("email = ?", email).Delete(&model.EmailVerification{}).Error; err != nil {
		return errors.Wrapf(err, "failed to delete existing email verifications")
	}

	// 创建新记录
	verification := &model.EmailVerification{
		Email:     email,
		Code:      code,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(10 * time.Minute), // 10分钟过期
	}
	return errors.WithStack(db.Create(verification).Error)
}

func GetEmailVerificationByEmail(email string) (*model.EmailVerification, error) {
	var verification model.EmailVerification
	// 获取未使用且未过期的验证码（应该只有一条）
	if err := db.Where("email = ? AND used = false AND expires_at > ?", email, time.Now()).
		First(&verification).Error; err != nil {
		return nil, errors.Wrapf(err, "failed to find email verification")
	}
	return &verification, nil
}

func MarkEmailVerificationAsUsed(email string) error {
	return errors.WithStack(db.Model(&model.EmailVerification{}).
		Where("email = ? AND used = false", email).
		Update("used", true).Error)
}
