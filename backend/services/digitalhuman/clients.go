package digitalhuman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/hock1024always/GoEdu/models"
)

// ============================================================
// HeyGen API Client
// 文档参考: https://docs.heygen.com/reference/generate-video-v2
// ============================================================

// HeyGenClient HeyGen 商业 API 客户端
type HeyGenClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewHeyGenClient() *HeyGenClient {
	return &HeyGenClient{
		apiKey:  os.Getenv("HEYGEN_API_KEY"),
		baseURL: "https://api.heygen.com",
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// heygenVideoRequest HeyGen 创建视频请求体
type heygenVideoRequest struct {
	VideoInputs []heygenVideoInput `json:"video_inputs"`
	Dimension   heygenDimension    `json:"dimension"`
	AspectRatio string             `json:"aspect_ratio,omitempty"`
}

type heygenVideoInput struct {
	Character heygenCharacter `json:"character"`
	Voice     heygenVoice     `json:"voice"`
	Background *heygenBG      `json:"background,omitempty"`
}

type heygenCharacter struct {
	Type     string `json:"type"`     // "avatar"
	AvatarID string `json:"avatar_id"`
	AvatarStyle string `json:"avatar_style,omitempty"` // "normal", "circle"
}

type heygenVoice struct {
	Type     string `json:"type"`      // "text"
	InputText string `json:"input_text"`
	VoiceID  string `json:"voice_id"`
	Speed    float64 `json:"speed,omitempty"`
}

type heygenBG struct {
	Type  string `json:"type"`  // "color"
	Value string `json:"value"` // "#FAFAFA"
}

type heygenDimension struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type heygenCreateResp struct {
	Code int    `json:"code"`
	Data struct {
		VideoID string `json:"video_id"`
	} `json:"data"`
	Message string `json:"message"`
}

type heygenStatusResp struct {
	Code int    `json:"code"`
	Data struct {
		VideoID  string `json:"video_id"`
		Status   string `json:"status"` // "pending","processing","completed","failed"
		VideoURL string `json:"video_url"`
		Error    *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	} `json:"data"`
}

func (c *HeyGenClient) SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (string, error) {
	avatarID := video.AvatarID
	if avatarID == "" {
		avatarID = os.Getenv("HEYGEN_DEFAULT_AVATAR_ID")
	}
	voiceID := video.VoiceID
	if voiceID == "" {
		voiceID = os.Getenv("HEYGEN_DEFAULT_VOICE_ID")
	}

	reqBody := heygenVideoRequest{
		VideoInputs: []heygenVideoInput{
			{
				Character: heygenCharacter{
					Type:        "avatar",
					AvatarID:    avatarID,
					AvatarStyle: "normal",
				},
				Voice: heygenVoice{
					Type:      "text",
					InputText: video.Text,
					VoiceID:   voiceID,
					Speed:     1.0,
				},
				Background: &heygenBG{
					Type:  "color",
					Value: "#FAFAFA",
				},
			},
		},
		Dimension: heygenDimension{Width: 1280, Height: 720},
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v2/video/generate", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("heygen request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result heygenCreateResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("heygen parse response failed: %v, body: %s", err, string(respBody))
	}
	if result.Code != 100 {
		return "", fmt.Errorf("heygen API error: code=%d, message=%s", result.Code, result.Message)
	}
	return result.Data.VideoID, nil
}

func (c *HeyGenClient) QueryStatus(ctx context.Context, externalID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/video_status.get?video_id="+externalID, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("heygen status request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result heygenStatusResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("heygen parse status failed: %v", err)
	}

	// 状态映射
	switch result.Data.Status {
	case "completed":
		return models.VideoStatusCompleted, result.Data.VideoURL, nil
	case "failed":
		return models.VideoStatusFailed, "", nil
	case "processing":
		return models.VideoStatusProcessing, "", nil
	default:
		return models.VideoStatusPending, "", nil
	}
}

// ============================================================
// SadTalker API Client
// 开源项目: github.com/kenwaytis/faster-SadTalker-API
// 默认部署: http://localhost:10364
// ============================================================

// SadTalkerClient SadTalker 开源 API 客户端
type SadTalkerClient struct {
	baseURL string
	http    *http.Client
}

func NewSadTalkerClient() *SadTalkerClient {
	baseURL := os.Getenv("SADTALKER_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:10364"
	}
	return &SadTalkerClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type sadtalkerSubmitResp struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type sadtalkerStatusResp struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`   // "pending","running","done","failed"
	VideoURL string `json:"video_url"`
	Error    string `json:"error,omitempty"`
}

func (c *SadTalkerClient) SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (string, error) {
	// SadTalker 接受 avatar 图片 URL + 文本，内部做 TTS 后驱动
	payload := map[string]interface{}{
		"source_image": video.AvatarID, // 直接传图片 URL
		"driven_text":  video.Text,
		"language":     video.Language,
		"still":        false,
		"enhancer":     "gfpgan",
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("sadtalker request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result sadtalkerSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("sadtalker parse response failed: %v, body: %s", err, string(respBody))
	}
	if result.TaskID == "" {
		return "", fmt.Errorf("sadtalker returned empty task_id, message: %s", result.Message)
	}
	return result.TaskID, nil
}

func (c *SadTalkerClient) QueryStatus(ctx context.Context, externalID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/status/"+externalID, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sadtalker status request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result sadtalkerStatusResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("sadtalker parse status failed: %v", err)
	}

	switch result.Status {
	case "done":
		return models.VideoStatusCompleted, result.VideoURL, nil
	case "failed":
		return models.VideoStatusFailed, "", nil
	case "running":
		return models.VideoStatusProcessing, "", nil
	default:
		return models.VideoStatusPending, "", nil
	}
}

// ============================================================
// Custom ML Model API Client
// 对接机器学习团队自训练的模型 API
// 配置通过环境变量: CUSTOM_DH_BASE_URL, CUSTOM_DH_API_KEY
// ============================================================

// CustomClient 自定义模型 API 客户端（ML 团队模型）
type CustomClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewCustomClient() *CustomClient {
	baseURL := os.Getenv("CUSTOM_DH_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080" // 默认本地 ML 服务
	}
	return &CustomClient{
		baseURL: baseURL,
		apiKey:  os.Getenv("CUSTOM_DH_API_KEY"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type customSubmitResp struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

type customStatusResp struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`   // "queued","running","success","error"
	VideoURL string `json:"video_url"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

func (c *CustomClient) SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (string, error) {
	// 通用接口设计：text + avatar_id + voice_id
	payload := map[string]interface{}{
		"text":      video.Text,
		"avatar_id": video.AvatarID,
		"voice_id":  video.VoiceID,
		"language":  video.Language,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/digital-human/generate", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("custom API request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result customSubmitResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("custom API parse response failed: %v, body: %s", err, string(respBody))
	}
	if result.TaskID == "" {
		return "", fmt.Errorf("custom API returned empty task_id, message: %s", result.Message)
	}
	return result.TaskID, nil
}

func (c *CustomClient) QueryStatus(ctx context.Context, externalID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/digital-human/task/"+externalID, nil)
	if err != nil {
		return "", "", err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("custom API status request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result customStatusResp
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("custom API parse status failed: %v", err)
	}

	switch result.Status {
	case "success":
		return models.VideoStatusCompleted, result.VideoURL, nil
	case "error":
		return models.VideoStatusFailed, "", nil
	case "running":
		return models.VideoStatusProcessing, "", nil
	default:
		return models.VideoStatusPending, "", nil
	}
}
