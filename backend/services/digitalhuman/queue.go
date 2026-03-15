package digitalhuman

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// Job 任务队列中的一个工作单元
type Job struct {
	VideoID string
}

// Queue 数字人视频生成任务队列
type Queue struct {
	jobs       chan Job
	workers    int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	apiClients map[string]APIClient // provider -> client
}

// APIClient 定义外部 API 客户端接口
type APIClient interface {
	// SubmitTask 提交生成任务，返回外部任务 ID
	SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (externalID string, err error)
	// QueryStatus 查询任务状态，返回新状态、视频 URL（完成时）
	QueryStatus(ctx context.Context, externalID string) (status string, videoURL string, err error)
}

var (
	globalQueue *Queue
	once        sync.Once
)

// GetQueue 获取全局任务队列（单例）
func GetQueue() *Queue {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalQueue = &Queue{
			jobs:       make(chan Job, 100),
			workers:    3,
			ctx:        ctx,
			cancel:     cancel,
			apiClients: make(map[string]APIClient),
		}
		// 注册 API 客户端
		globalQueue.apiClients[models.APIProviderHeyGen] = NewHeyGenClient()
		globalQueue.apiClients[models.APIProviderSadTalker] = NewSadTalkerClient()
		globalQueue.apiClients[models.APIProviderCustom] = NewCustomClient()
		globalQueue.start()
	})
	return globalQueue
}

// RegisterClient 注册或替换 API 客户端（用于测试或动态替换）
func (q *Queue) RegisterClient(provider string, client APIClient) {
	q.apiClients[provider] = client
}

// start 启动 worker goroutine
func (q *Queue) start() {
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// Stop 停止队列
func (q *Queue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// Enqueue 将视频任务入队
func (q *Queue) Enqueue(videoID string) {
	select {
	case q.jobs <- Job{VideoID: videoID}:
		log.Printf("[Queue] Enqueued job: %s", videoID)
	default:
		log.Printf("[Queue] Queue full, job dropped: %s", videoID)
	}
}

// worker 工作协程：处理任务
func (q *Queue) worker(id int) {
	defer q.wg.Done()
	log.Printf("[Queue] Worker %d started", id)

	for {
		select {
		case job := <-q.jobs:
			q.processJob(job)
		case <-q.ctx.Done():
			log.Printf("[Queue] Worker %d stopped", id)
			return
		}
	}
}

// processJob 处理单个任务
func (q *Queue) processJob(job Job) {
	log.Printf("[Queue] Processing job: %s", job.VideoID)

	var video models.DigitalHumanVideo
	if err := dao.Db.Where("id = ?", job.VideoID).First(&video).Error; err != nil {
		log.Printf("[Queue] Failed to load video %s: %v", job.VideoID, err)
		return
	}

	// 跳过已完成或已取消的任务
	if video.Status == models.VideoStatusCompleted || video.Status == models.VideoStatusCancelled {
		return
	}

	client, ok := q.apiClients[video.Provider]
	if !ok {
		log.Printf("[Queue] Unknown provider: %s for video %s", video.Provider, job.VideoID)
		q.markFailed(&video, fmt.Sprintf("unknown provider: %s", video.Provider))
		return
	}

	// 更新为处理中
	dao.Db.Model(&video).Updates(map[string]interface{}{
		"status":     models.VideoStatusProcessing,
		"updated_at": time.Now(),
	})

	ctx, cancel := context.WithTimeout(q.ctx, 10*time.Minute)
	defer cancel()

	// 提交外部任务（如尚未提交）
	if video.ExternalID == "" {
		externalID, err := client.SubmitTask(ctx, &video)
		if err != nil {
			log.Printf("[Queue] SubmitTask failed for %s: %v", job.VideoID, err)
			q.handleRetry(&video, fmt.Sprintf("submit failed: %v", err))
			return
		}
		video.ExternalID = externalID
		dao.Db.Model(&video).Update("external_id", externalID)
	}

	// 轮询状态（最多等待 10 分钟）
	q.pollStatus(ctx, &video, client)
}

// pollStatus 轮询外部任务状态
func (q *Queue) pollStatus(ctx context.Context, video *models.DigitalHumanVideo, client APIClient) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Queue] Polling timeout for video %s", video.ID)
			q.handleRetry(video, "polling timeout")
			return
		case <-ticker.C:
			status, videoURL, err := client.QueryStatus(ctx, video.ExternalID)
			if err != nil {
				log.Printf("[Queue] QueryStatus failed for %s: %v", video.ID, err)
				continue
			}

			log.Printf("[Queue] Video %s status: %s", video.ID, status)

			switch status {
			case models.VideoStatusCompleted:
				now := time.Now()
				dao.Db.Model(video).Updates(map[string]interface{}{
					"status":     models.VideoStatusCompleted,
					"video_url":  videoURL,
					"updated_at": now,
				})
				log.Printf("[Queue] Video %s completed: %s", video.ID, videoURL)
				return
			case models.VideoStatusFailed:
				q.markFailed(video, "external API reported failure")
				return
			}
			// pending/processing 继续轮询
		}
	}
}

// handleRetry 处理重试逻辑
func (q *Queue) handleRetry(video *models.DigitalHumanVideo, reason string) {
	if video.RetryCount < video.MaxRetry {
		video.RetryCount++
		dao.Db.Model(video).Updates(map[string]interface{}{
			"status":      models.VideoStatusPending,
			"retry_count": video.RetryCount,
			"error_msg":   reason,
			"external_id": "", // 清空外部 ID，重新提交
			"updated_at":  time.Now(),
		})
		// 延迟重试（指数退避：30s, 60s, 120s）
		delay := time.Duration(30*(1<<(video.RetryCount-1))) * time.Second
		log.Printf("[Queue] Retrying video %s in %v (attempt %d/%d)", video.ID, delay, video.RetryCount, video.MaxRetry)
		go func() {
			time.Sleep(delay)
			q.Enqueue(video.ID)
		}()
	} else {
		q.markFailed(video, reason)
	}
}

// markFailed 标记任务为失败
func (q *Queue) markFailed(video *models.DigitalHumanVideo, reason string) {
	dao.Db.Model(video).Updates(map[string]interface{}{
		"status":     models.VideoStatusFailed,
		"error_msg":  reason,
		"updated_at": time.Now(),
	})
	log.Printf("[Queue] Video %s failed: %s", video.ID, reason)
}

// CreateAndEnqueue 创建视频任务并加入队列
func CreateAndEnqueue(userID uint, title, text, avatarID, voiceID, language, provider string) (*models.DigitalHumanVideo, error) {
	if provider == "" {
		provider = models.APIProviderHeyGen
	}
	if language == "" {
		language = "zh-CN"
	}

	video := &models.DigitalHumanVideo{
		ID:         uuid.New().String(),
		UserID:     userID,
		Title:      title,
		Text:       text,
		AvatarID:   avatarID,
		VoiceID:    voiceID,
		Language:   language,
		Provider:   provider,
		Status:     models.VideoStatusPending,
		MaxRetry:   3,
		RetryCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := dao.Db.Create(video).Error; err != nil {
		return nil, fmt.Errorf("创建视频任务失败: %v", err)
	}

	GetQueue().Enqueue(video.ID)
	return video, nil
}
