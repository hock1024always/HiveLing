package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/models"
	"os"
)

type StaticController struct{}

type StaticRequst struct {
	PhotoName  string `json:"photo"`
	PaperName  string `json:"paper"`
	Suggestion string `json:"suggestion"`
	Path       string `json:"path"`
	FileName   string `json:"filename"`
}

func (s StaticController) MindMap(c *gin.Context) {
	var req StaticRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.PhotoName
	// 设置文件路径
	filePath := config.RootPath + "/Source/MindMap/" + fileName + ".png"
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)

}

func (s StaticController) Exam(c *gin.Context) {
	var req StaticRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.PaperName
	// 设置文件路径
	filePath := config.RootPath + "/Source/TestPaper/" + fileName + ".pdf"
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)
}

func (s StaticController) Teach(c *gin.Context) {
	var req StaticRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.Suggestion
	// 设置文件路径
	filePath := config.RootPath + "/Source/TeacherSuggestion/" + fileName + ".pdf"
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)
}

func (s StaticController) Anything(c *gin.Context) {
	var req StaticRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	path := req.Path
	filename := req.FileName
	// 设置文件路径
	filePath := config.RootPath + path + filename
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)
}
