package models

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type PptSlide struct {
	SlideNumber  string `json:"slide_number"`
	SlideContent string `json:"slide_content"`
}

func ReturnJson(c *gin.Context, pageNum string, pptContext string) {
	pptSlide := &PptSlide{SlideNumber: pageNum, SlideContent: pptContext}
	c.JSON(http.StatusOK, pptSlide)
}
