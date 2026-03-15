package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/models"
	"os"
)

type ReportRequst struct {
	Filename string `json:"filename"`
}

func (a AIController) Upload(c *gin.Context) {
	// 获取文件名
	//fileName := c.DefaultPostForm("filename", "")
	var req ReportRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.Filename
	// 设置文件路径
	filePath := config.RootPath + "/LearningAnalysis/AnalysisReport/pdf/" + fileName + ".pdf"

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, filePath)
		return
	}
	// 读取文件并发送
	c.File(filePath)
}

func (a AIController) Optimize(c *gin.Context) {
	// 获取文件名
	//fileName := c.DefaultPostForm("filename", "")
	var req ReportRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.Filename
	// 设置文件路径
	filePath := config.RootPath + "/LearningAnalysis/ImprovementReport/pdf/" + fileName + ".pdf"

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)
}
