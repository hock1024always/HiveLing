package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AIController struct{}

func (a AIController) Digital(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Range")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
	c.Next()

	name := c.Param("name")
	sourcePath := config.RootPath + "/Videos"
	videoPath := filepath.Join(sourcePath, name)

	file, err := os.Open(videoPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		c.Header("Content-Length", strconv.FormatInt(fileSize, 10))
		c.Header("Accept-Ranges", "bytes")
		io.Copy(c.Writer, file)
		return
	}

	rangeValues := strings.Split(rangeHeader, "=")
	if len(rangeValues) != 2 {
		c.Status(http.StatusBadRequest)
		return
	}
	rangeValues = strings.Split(rangeValues[1], "-")
	start, err := strconv.ParseInt(rangeValues[0], 10, 64)
	if err != nil || start > fileSize {
		c.Status(http.StatusBadRequest)
		return
	}
	end := fileSize - 1
	if len(rangeValues) > 1 && rangeValues[1] != "" {
		end, err = strconv.ParseInt(rangeValues[1], 10, 64)
		if err != nil || end > fileSize {
			c.Status(http.StatusBadRequest)
			return
		}
	}

	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Accept-Ranges", "bytes")
	c.Status(http.StatusPartialContent)
	file.Seek(start, io.SeekStart)
	io.CopyN(c.Writer, file, end-start+1)
}

func (a AIController) VideoList(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Range")
	c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
	sourcePath := config.RootPath + "/Videos"
	files, err := os.ReadDir(sourcePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read videos directory"})
		return
	}

	var videoList []string
	for _, file := range files {
		if !file.IsDir() {
			videoList = append(videoList, file.Name())
		}
	}

	c.JSON(http.StatusOK, gin.H{"videos": videoList})

}
