package utils

import (
	"errors"
	"pkuphysu-backend/internal/config"
	"pkuphysu-backend/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type UserClaims struct {
	UserID uint  `json:"user_id"`
	PwdTS  int64 `json:"pwd_ts"`
	jwt.RegisteredClaims
}

func GenerateToken(user *model.User) (tokenString string, err error) {
	claims := UserClaims{
		UserID: user.ID,
		PwdTS:  user.PwdTS,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Conf.TokenExpire) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err = token.SignedString([]byte(config.Conf.JwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, err
}

func ParseToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Conf.JwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
