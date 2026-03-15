package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/pkg/mail"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

type StudentController struct{}

func (s StudentController) Register(c *gin.Context) {
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
func (s StudentController) Verify(c *gin.Context) {
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
func (s StudentController) Login(c *gin.Context) {
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

func (s StudentController) Upload(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(200)
		return
	}

	c.Next()

	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file: " + err.Error()})
		return
	}

	// 获取文件名
	filename := filepath.Base(fileHeader.Filename)

	// 创建存储目录（如果不存在）
	uploadDir := config.RootPath + "/student/paper"
	//if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
	//	log.Printf("Creating upload directory: %s", uploadDir)
	//	err := os.MkdirAll(uploadDir, 0755)
	//	if err != nil {
	//		log.Printf("Error creating upload directory: %v", err)
	//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory: " + err.Error()})
	//		return
	//	}
	//}

	// 保存文件到指定目录
	filePath := filepath.Join(uploadDir, filename)
	log.Printf("Saving file to: %s", filePath)
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		log.Printf("Error saving file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"filename": filename,
		"filepath": filePath,
	})
}

func (s StudentController) UploadAnswer(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(200)
		return
	}

	c.Next()

	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file: " + err.Error()})
		return
	}

	// 获取文件名
	filename := filepath.Base(fileHeader.Filename)

	// 创建存储目录（如果不存在）
	uploadDir := config.RootPath + "/student/reply"
	//if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
	//	log.Printf("Creating upload directory: %s", uploadDir)
	//	err := os.MkdirAll(uploadDir, 0755)
	//	if err != nil {
	//		log.Printf("Error creating upload directory: %v", err)
	//		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory: " + err.Error()})
	//		return
	//	}
	//}

	// 保存文件到指定目录
	filePath := filepath.Join(uploadDir, filename)
	log.Printf("Saving file to: %s", filePath)
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		log.Printf("Error saving file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file: " + err.Error()})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"filename": filename,
		"filepath": filePath,
	})
}

func (s StudentController) GetScore(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"论文评分":   88,
		"答辩材料评分": 85,
		"评语":     "该作品选题紧扣人工智能时代背景，具有明确的现实意义。论文结构完整，逻辑清晰，对AI算法在农业预测中的应用分析较为深入，但数据样本量和多样性稍显不足，建议补充实地验证数据。答辩材料设计简洁，重点突出，但对技术细节的呈现可进一步可视化（如增加流程图、对比图表）。团队展现了扎实的技术能力和跨学科思维，若能在实际应用场景的可行性分析上加强则更佳。",
	})
}
