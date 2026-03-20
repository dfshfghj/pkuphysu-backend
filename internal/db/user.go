package db

import (
	"pkuphysu-backend/internal/model"

	"github.com/pkg/errors"
)

func GetUserByRole(role int) (*model.User, error) {
	user := model.User{Role: role}
	if err := db.Where(user).Take(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByName(username string) (*model.User, error) {
	user := model.User{Username: username}
	if err := db.Where(user).First(&user).Error; err != nil {
		return nil, errors.Wrapf(err, "failed find user")
	}
	return &user, nil
}

func GetUserByStuid(stuid string) (*model.User, error) {
	user := model.User{Stuid: stuid}
	if err := db.Where(user).First(&user).Error; err != nil {
		return nil, errors.Wrapf(err, "failed find user by stuid")
	}
	return &user, nil
}

func GetUserById(id uint) (*model.User, error) {
	var u model.User
	if err := db.First(&u, id).Error; err != nil {
		return nil, errors.Wrapf(err, "failed get old user")
	}
	return &u, nil
}

func CreateUser(u *model.User) error {
	return errors.WithStack(db.Create(u).Error)
}

func UpdateUser(u *model.User) error {
	return errors.WithStack(db.Save(u).Error)
}

func DeleteUserById(id uint) error {
	return errors.WithStack(db.Delete(&model.User{}, id).Error)
}

func GetUsers() ([]model.User, error) {
	var users []model.User
	if err := db.Select("id, username, verified, stuname, stuid, role, disabled, bio").Find(&users).Error; err != nil {
		return nil, errors.Wrapf(err, "failed to get users")
	}
	return users, nil
}

func GetUsersByRole(role int) ([]model.User, error) {
	var users []model.User
	if err := db.Select("id, username, verified, stuname, stuid, role, disabled, bio").Where("role = ?", role).Find(&users).Error; err != nil {
		return nil, errors.Wrapf(err, "failed to get users by role")
	}
	return users, nil
}

func GetAllAdmins() ([]model.User, error) {
	return GetUsersByRole(model.ADMIN)
}
