package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/models"
	"os"
)

type PptController struct{}
type PptRequst struct {
	Message  string `json:"message"`
	Filename string `json:"filename"`
}

// 生成大纲
func (p PptController) PptReview(c *gin.Context) {
	//鉴权也不搞了
	var req PptRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}
	message := req.Message
	switch message {
	case "请为我生成机器学习概述的PPT大纲":
		models.ReturnJson(c, "19", config.PptRoMLJSON)
	case "请为我生成计算机视觉的PPT大纲":
		models.ReturnJson(c, "21", config.PptCVJSON)
	case "请为我生成计算机神经网络的PPT大纲":
		models.ReturnJson(c, "21", config.PptNLPJSON)
	case "请为我生成大预言模型的PPT大纲":
		models.ReturnJson(c, "19", config.PptLLMJSON)
	case "请为我生成联邦学习的PPT大纲":
		models.ReturnJson(c, "19", config.PptFLJSON)
	default:
		models.ReturnJson(c, "19", "ber实力太差，请重新输入字段喵")
	}
}

func (p PptController) PptResource(c *gin.Context) {
	// 获取文件名
	//fileName := c.DefaultPostForm("filename", "")
	var req PptRequst
	if err := c.ShouldBindJSON(&req); err != nil {
		models.ReturnError(c, 4001, "无效的请求数据")
		return
	}

	fileName := req.Filename
	// 设置文件路径
	filePath := config.RootPath + "/PPT/marchain/" + fileName + ".pdf"

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		models.ReturnError2(c, "文件不存在")
		return
	}
	// 读取文件并发送
	c.File(filePath)
	//models.ReturnError2(c, "资源生成成功")
}
