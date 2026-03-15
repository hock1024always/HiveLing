package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/pkg/mail"
	"net/http"
	"time"
)

// 实现关于用户的功能
type UserController struct{}
type UserLoginApi struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
}

type RegisterRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	Email           string `json:"email"`
}

var verificationCodes = make(map[string]string)

func (u UserController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	//接受用户名 密码以及确认密码
	username := req.Username
	password := req.Password
	confirmPassword := req.ConfirmPassword
	email := req.Email

	//验证 输入是否存在某项为空 密码和确认密码是否一致 是否已经存在该用户
	if username == "" || password == "" || confirmPassword == "" || email == "" {
		models.ReturnError(c, 4001, email+confirmPassword+password+username)
		return
	}
	//if password != confirmPassword {
	//	models.ReturnError(c, 4002, "密码和确认密码不一致")
	//	return
	//}
	//user1, _ := models.CheckUserExist(username)
	//if user1.Id != 0 {
	//	models.ReturnError(c, 4003, "该用户已存在")
	//	return
	//}

	// 生成验证码
	code := mail.GenerateVerificationCode()
	verificationCodes[email] = code

	// 发送验证码
	err := mail.SendMail(email, "您的验证码", fmt.Sprintf("您的验证码是：%s，有效期5分钟。", code))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "验证码发送失败"})
		return
	}

	// 设置验证码有效期
	time.AfterFunc(5*time.Minute, func() {
		delete(verificationCodes, email)
	})

	c.JSON(http.StatusOK, gin.H{"message": "验证码已发送，请在5分钟内完成验证"})

}

// 验证用户
func (u UserController) Verify(c *gin.Context) {
	//// 定义接收JSON数据的结构体
	//var data struct {
	//	Username string `json:"username"`
	//	Password string `json:"password"`
	//	Email    string `json:"email"`
	//	Code     string `json:"confirm_code"`
	//}
	//// 绑定JSON数据到结构体
	//if err := c.ShouldBindJSON(&data); err != nil {
	//	// 如果绑定失败，返回错误信息
	//	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "无效的JSON数据"})
	//	return
	//}
	//
	//// 检查验证码
	//// 从验证码字典中获取邮箱对应的验证码
	//code, ok := verificationCodes[data.Email]
	//// 如果验证码不存在或验证码不匹配，返回错误信息
	//if !ok || code != data.Code {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误或已过期"})
	//	return
	//}
	//
	//// 注册用户
	//// 创建用户结构体
	//user := models.User{
	//	Username: data.Username,
	//	Password: data.Password, // 实际开发中应加密存储
	//	Email:    data.Email,
	//}
	//
	////创建用户
	//_, err2 := models.AddUser(user.Username, user.Password, user.Email)
	//if err2 != nil {
	//	models.ReturnError(c, 4004, err2.Error())
	//	return
	//}
	//
	// 清理验证码
	//// 从验证码字典中删除已使用的验证码
	//delete(verificationCodes, data.Email)

	// 返回成功信息
	c.JSON(http.StatusOK, gin.H{"isRegistered": "true"})

}

// 登陆
func (u UserController) Login(c *gin.Context) {
	//接受用户名 密码
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}
	username := req.Username
	password := req.Password

	//验证 用户名或者密码为空 用户名不存在 密码错误
	if username == "" || password == "" {
		models.ReturnError(c, 4011, "用户名或密码为空")
		return
	}
	//user1, err := models.CheckUserExist(username)
	//if err != nil {
	//	models.ReturnError(c, 4012, "用户名不存在")
	//	return
	//}
	//
	//if password != user1.Password {
	//	models.ReturnError(c, 4013, "密码错误")
	//	return
	//}

	token, err := models.GenerateToken(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// 返回 Token
	c.JSON(http.StatusOK, gin.H{"isAuthenticated": "true", "token": token})
}

// 注销用户
func (u UserController) UserDelete(c *gin.Context) {
	//接受Token
	token := c.DefaultPostForm("token", "")
	//鉴权
	username := models.ValidateToken(token)
	user1, _ := dao.CheckUserExist(username)
	if user1.Id == 0 {
		models.ReturnError(c, 4012, "用户名不存在")
		return
	}

	//删除用户
	err2 := dao.DeleteUserByUsername(user1.Username)
	if err2 != nil {
		models.ReturnError(c, 4014, "删除用户失败")
		return
	}

	//删除用户
	err4 := dao.DeleteUserByUsername(user1.Username)
	if err4 != nil {
		models.ReturnError(c, 4014, "删除用户失败")
		return
	}
	//返回删除信息
	//models.ReturnSuccess(c, 0, "删除成功", nil, 1)
	c.JSON(http.StatusOK, gin.H{"isAuthenticated": "true"})
}

//func (u UserController) GetChatRecords(c *gin.Context) {
//	//接受Token
//	token := c.DefaultPostForm("token", "")
//	//鉴权
//	username := models.ValidateToken(token)
//	user1, _ := models.CheckUserExist(username)
//	if user1.Id == 0 {
//		models.ReturnError(c, 4012, "用户名不存在")
//		return
//	}
//
//	//获取聊天记录
//
//	//获取投票列表
//	voteList, err2 := models.GetVoteList(user1.Id, "id desc")
//	if err2 != nil {
//		ReturnError(c, 4014, "获取投票列表失败")
//		return
//	}
//	ReturnSuccess(c, 0, "获取投票列表成功", voteList, 1)
//}

//// 修改用户密码
//func (u UserController) ModifyPassword(c *gin.Context) {
//	//接受用户名 密码
//	username := c.DefaultPostForm("username", "")
//	password := c.DefaultPostForm("password", "")
//	newPassword := c.DefaultPostForm("new_password", "")
//	confirmNewPassword := c.DefaultPostForm("confirm_new_password", "")
//
//	//验证 用户名或者密码为空 用户名不存在 密码错误
//	if username == "" || password == "" {
//		ReturnError(c, 4011, "用户名或密码为空")
//		return
//	}
//	user1, err := models.CheckUserExist(username)
//	if err != nil {
//		ReturnError(c, 4012, "用户名不存在")
//		return
//	}
//	if user1.Password != password {
//		ReturnError(c, 4013, "密码错误")
//		return
//	}
//	if newPassword == "" || confirmNewPassword == "" {
//		ReturnError(c, 4016, "新密码或确认新密码为空")
//		return
//	}
//	if newPassword != confirmNewPassword {
//		ReturnError(c, 4015, "新密码与确认新密码不一致")
//		return
//	}
//
//	//修改密码
//	updatePassword, err2 := models.UpdateUserPassword(username, newPassword)
//	if err2 != nil {
//		ReturnError(c, 4014, "修改密码失败")
//		return
//	}
//	//返回修改信息
//	ReturnSuccess(c, 0, "修改密码成功", "新密码是:"+updatePassword, 1)
//}
