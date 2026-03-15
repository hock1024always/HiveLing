package controllers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/digitalhuman"
)

// DigitalHumanController 数字人视频生成控制器
type DigitalHumanController struct{}

func NewDigitalHumanController() *DigitalHumanController {
	// 启动任务队列
	digitalhuman.GetQueue()
	return &DigitalHumanController{}
}

// ---- 请求/响应结构 ----

type createVideoReq struct {
	Title    string `json:"title" binding:"required"`
	Text     string `json:"text" binding:"required"`
	AvatarID string `json:"avatar_id"`
	VoiceID  string `json:"voice_id"`
	Language string `json:"language"`
	Provider string `json:"provider"` // heygen / sadtalker / custom
}

type videoResp struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Status     string     `json:"status"`
	Provider   string     `json:"provider"`
	VideoURL   string     `json:"video_url,omitempty"`
	Duration   float64    `json:"duration,omitempty"`
	Resolution string     `json:"resolution,omitempty"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	RetryCount int        `json:"retry_count"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func toVideoResp(v *models.DigitalHumanVideo) videoResp {
	return videoResp{
		ID:         v.ID,
		Title:      v.Title,
		Status:     v.Status,
		Provider:   v.Provider,
		VideoURL:   v.VideoURL,
		Duration:   v.Duration,
		Resolution: v.Resolution,
		ErrorMsg:   v.ErrorMsg,
		RetryCount: v.RetryCount,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
	}
}

// ---- 接口实现 ----

// CreateVideo POST /api/digital-human/videos
// 提交文本，触发数字人视频生成任务
func (ctrl *DigitalHumanController) CreateVideo(c *gin.Context) {
	var req createVideoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 演示用：固定 userID = 0（正式接入 JWT 中间件后替换）
	var userID uint = 0
	if uid, exists := c.Get("user_id"); exists {
		if v, ok := uid.(uint); ok {
			userID = v
		}
	}

	video, err := digitalhuman.CreateAndEnqueue(
		userID,
		req.Title,
		req.Text,
		req.AvatarID,
		req.VoiceID,
		req.Language,
		req.Provider,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "任务已提交，正在生成中",
		"video":   toVideoResp(video),
	})
}

// GetVideo GET /api/digital-human/videos/:id
// 查询单个视频任务状态
func (ctrl *DigitalHumanController) GetVideo(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var video models.DigitalHumanVideo
	if err := dao.Db.Where("id = ? AND deleted_at IS NULL", id).First(&video).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, toVideoResp(&video))
}

// ListVideos GET /api/digital-human/videos?page=1&size=10&status=xxx
// 获取视频列表
func (ctrl *DigitalHumanController) ListVideos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}
	offset := (page - 1) * size

	query := dao.Db.Model(&models.DigitalHumanVideo{}).Where("deleted_at IS NULL")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var videos []models.DigitalHumanVideo
	query.Order("created_at DESC").Offset(offset).Limit(size).Find(&videos)

	resp := make([]videoResp, 0, len(videos))
	for i := range videos {
		resp = append(resp, toVideoResp(&videos[i]))
	}

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"page":  page,
		"size":  size,
		"items": resp,
	})
}

// RetryVideo POST /api/digital-human/videos/:id/retry
// 手动重试失败的任务
func (ctrl *DigitalHumanController) RetryVideo(c *gin.Context) {
	id := c.Param("id")
	var video models.DigitalHumanVideo
	if err := dao.Db.Where("id = ? AND deleted_at IS NULL", id).First(&video).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	if video.Status != models.VideoStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只有失败的任务可以重试"})
		return
	}

	// 重置状态并重新入队
	dao.Db.Model(&video).Updates(map[string]interface{}{
		"status":      models.VideoStatusPending,
		"retry_count": 0,
		"error_msg":   "",
		"external_id": "",
		"updated_at":  time.Now(),
	})
	digitalhuman.GetQueue().Enqueue(video.ID)

	c.JSON(http.StatusOK, gin.H{"message": "任务已重新加入队列"})
}

// CancelVideo DELETE /api/digital-human/videos/:id
// 取消/软删除任务
func (ctrl *DigitalHumanController) CancelVideo(c *gin.Context) {
	id := c.Param("id")
	var video models.DigitalHumanVideo
	if err := dao.Db.Where("id = ? AND deleted_at IS NULL", id).First(&video).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 只有 pending/failed 可以取消
	if video.Status == models.VideoStatusProcessing {
		c.JSON(http.StatusBadRequest, gin.H{"error": "生成中的任务无法取消"})
		return
	}

	dao.Db.Model(&video).Update("status", models.VideoStatusCancelled)
	dao.Db.Delete(&video)

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// Webhook POST /api/digital-human/webhook
// 接收外部 API 的回调通知（HeyGen 等支持 webhook 的服务）
func (ctrl *DigitalHumanController) Webhook(c *gin.Context) {
	var payload struct {
		EventType string `json:"event_type"` // "avatar_video.success" / "avatar_video.fail"
		EventData struct {
			VideoID  string `json:"video_id"`
			Status   string `json:"status"`
			VideoURL string `json:"video_url"`
			Error    string `json:"error,omitempty"`
		} `json:"event_data"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 通过 external_id 查找本地任务
	var video models.DigitalHumanVideo
	if err := dao.Db.Where("external_id = ?", payload.EventData.VideoID).First(&video).Error; err != nil {
		// 任务不存在，忽略回调（可能是重复推送）
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
		return
	}

	switch payload.EventType {
	case "avatar_video.success":
		dao.Db.Model(&video).Updates(map[string]interface{}{
			"status":     models.VideoStatusCompleted,
			"video_url":  payload.EventData.VideoURL,
			"updated_at": time.Now(),
		})
	case "avatar_video.fail":
		dao.Db.Model(&video).Updates(map[string]interface{}{
			"status":     models.VideoStatusFailed,
			"error_msg":  payload.EventData.Error,
			"updated_at": time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// ListAvatars GET /api/digital-human/avatars
// 获取可用的数字人形象列表
func (ctrl *DigitalHumanController) ListAvatars(c *gin.Context) {
	var avatars []models.DigitalHumanAvatar
	dao.Db.Where("is_active = ?", true).Order("id ASC").Find(&avatars)

	// 如果数据库为空，返回内置默认形象
	if len(avatars) == 0 {
		avatars = []models.DigitalHumanAvatar{
			{
				AvatarID:    "default_teacher_male",
				Name:        "男教师（默认）",
				Description: "专业男性教师形象，适合历史、数学等学科",
				Provider:    models.APIProviderHeyGen,
				IsActive:    true,
			},
			{
				AvatarID:    "default_teacher_female",
				Name:        "女教师（默认）",
				Description: "专业女性教师形象，适合语文、英语等学科",
				Provider:    models.APIProviderHeyGen,
				IsActive:    true,
			},
		}
	}

	c.JSON(http.StatusOK, gin.H{"avatars": avatars})
}
