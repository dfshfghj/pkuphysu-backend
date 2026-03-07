package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"pkuphysu-backend/internal/config"

	"github.com/pkg/errors"
	"gopkg.in/gomail.v2"
)

func GenerateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", errors.Wrap(err, "failed to generate verification code")
	}
	code := fmt.Sprintf("%06d", n)
	return code, nil
}

func SendVerificationEmail(toEmail, code string) error {
	conf := config.Conf.Email

	if conf.Host == "" || conf.Username == "" || conf.Password == "" {
		return errors.New("email configuration is not set")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", conf.From)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "邮箱验证码")
	m.SetBody("text/plain", fmt.Sprintf("您的验证码是：%s，请在10分钟内使用。", code))

	d := gomail.NewDialer(conf.Host, conf.Port, conf.Username, conf.Password)
	d.SSL = true

	if err := d.DialAndSend(m); err != nil {
		return errors.Wrap(err, "failed to send email")
	}

	return nil
}
