package dao

import (
	"github.com/hock1024always/GoEdu/models"
)

// CheckUserExist 判断用户名是否已经存在
func CheckUserExist(username string) (models.User, error) {
	var user models.User
	err := Db.Where("username =?", username).First(&user).Error
	return user, err
}

// AddUser 保存用户
func AddUser(username, password string, email string) (models.UserApi, error) {
	user := models.User{
		Username: username,
		Password: password,
		Email:    email,
	}
	err := Db.Create(&user).Error
	userapi := models.UserApi{Username: username, Userid: user.Id}
	return userapi, err
}

// CheckUserById 通过Id来查找用户
func CheckUserById(id int) (models.User, error) {
	var user models.User
	err := Db.Where("id =?", id).First(&user).Error
	return user, err
}

// DeleteUserByUsername 删除通过用户名用户
func DeleteUserByUsername(username string) error {
	var user models.User
	err := Db.Where("username =?", username).Delete(&user).Error
	return err
}

// UpdateUserPassword 更新密码
func UpdateUserPassword(username string, password string) (string, error) {
	var user models.User
	err := Db.Model(&user).Where("username =?", username).UpdateColumn("password", password).Error
	return password, err
}
