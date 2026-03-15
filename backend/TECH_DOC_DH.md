# 数字人视频生成模块 - 技术文档

> 本文档详细阐述数字人视频生成模块的技术实现，适用于技术评审和面试讲解。

---

## 一、功能概述

### 1.1 业务需求

实现「文本 → 数字人讲课视频」的工程化链路：
- 用户输入讲课文本
- 系统调用外部 AI 服务生成数字人视频
- 支持任务状态追踪、重试、下载
- 可对接多种 API 提供商（商业/开源/自研）

### 1.2 核心能力

| 能力 | 实现方式 |
|------|---------|
| 异步任务处理 | Go channel + worker goroutine |
| 状态轮询 | 定时器 + 外部 API 查询 |
| 失败重试 | 指数退避策略 |
| 多 API 适配 | 接口抽象 + 策略模式 |
| 前端实时更新 | 轮询 + 状态徽章动画 |

---

## 二、系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Frontend (HTML/JS)                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐ │
│  │ 表单提交  │  │ 任务列表  │  │ 视频播放  │  │ 状态轮询 (8s interval) │ │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └──────────┬───────────┘ │
└───────┼─────────────┼─────────────┼───────────────────┼─────────────┘
        │             │             │                   │
        ▼             ▼             ▼                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Backend (Go + Gin)                            │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    Controller Layer                           │   │
│  │  CreateVideo │ GetVideo │ ListVideos │ RetryVideo │ Webhook  │   │
│  └──────────────────────────────┬───────────────────────────────┘   │
│                                 │                                    │
│  ┌──────────────────────────────▼───────────────────────────────┐   │
│  │                    Service Layer                              │   │
│  │  ┌─────────────────────────────────────────────────────────┐ │   │
│  │  │              Task Queue (Singleton)                      │ │   │
│  │  │  ┌─────────┐  ┌─────────┐  ┌─────────┐                  │ │   │
│  │  │  │Worker 0 │  │Worker 1 │  │Worker 2 │                  │ │   │
│  │  │  └────┬────┘  └────┬────┘  └────┬────┘                  │ │   │
│  │  │       │            │            │                        │ │   │
│  │  │       └────────────┼────────────┘                        │ │   │
│  │  │                    ▼                                     │ │   │
│  │  │  ┌──────────────────────────────────────────────────┐   │ │   │
│  │  │  │              API Clients (Strategy)               │   │ │   │
│  │  │  │  HeyGenClient │ SadTalkerClient │ CustomClient   │   │ │   │
│  │  │  └──────────────────────────────────────────────────┘   │ │   │
│  │  └─────────────────────────────────────────────────────────┘ │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                 │                                    │
│  ┌──────────────────────────────▼───────────────────────────────┐   │
│  │                    Data Access Layer                          │   │
│  │                    GORM + MySQL                               │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     External AI Services                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────┐  │
│  │   HeyGen    │  │  SadTalker  │  │  Custom ML Model (Team)     │  │
│  │  (Commercial)│  │  (Open Source)│  │  (Self-hosted)              │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流图

```
用户提交文本
    │
    ▼
┌─────────────────┐
│ Controller      │ ← 参数校验
│ CreateVideo()   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 创建 DB 记录    │ → status: pending
│ 生成 video_id   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 任务入队        │ → channel Job{video_id}
│ Queue.Enqueue() │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Worker 消费     │ ← 3 个并发 worker
│ processJob()    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 调用外部 API    │ → HeyGen/SadTalker/Custom
│ SubmitTask()    │ → 返回 external_id
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 更新 DB 状态    │ → status: processing
│ 存储 external_id│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 轮询外部状态    │ ← 每 10 秒查询一次
│ QueryStatus()   │ ← 超时 10 分钟
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────┐
│完成   │ │失败   │
└───┬───┘ └───┬───┘
    │         │
    ▼         ▼
┌───────┐ ┌───────────┐
│存URL  │ │重试/标记  │
│完成   │ │失败       │
└───────┘ └───────────┘
```

---

## 三、核心代码实现

### 3.1 数据模型设计

**文件：** `models/digital_human.go`

```go
// 任务状态常量
const (
    VideoStatusPending    = "pending"    // 等待处理
    VideoStatusProcessing = "processing" // 生成中
    VideoStatusCompleted  = "completed"  // 生成完成
    VideoStatusFailed     = "failed"     // 生成失败
)

// 数字人视频任务表
type DigitalHumanVideo struct {
    ID          string  `gorm:"primaryKey;type:varchar(64)"`  // UUID
    UserID      uint    `gorm:"index"`                         // 用户ID
    Title       string  `gorm:"type:varchar(255)"`             // 视频标题
    Text        string  `gorm:"type:longtext"`                 // 驱动文本
    AvatarID    string  `gorm:"type:varchar(128)"`             // 数字人形象ID
    VoiceID     string  `gorm:"type:varchar(128)"`             // 音色ID
    Provider    string  `gorm:"type:varchar(32);default:'heygen'"` // API提供商
    ExternalID  string  `gorm:"type:varchar(128);index"`       // 外部API任务ID
    Status      string  `gorm:"type:varchar(32);default:'pending'"`
    VideoURL    string  `gorm:"type:varchar(1024)"`            // 成品视频URL
    ErrorMsg    string  `gorm:"type:text"`                     // 错误信息
    RetryCount  int     `gorm:"default:0"`                     // 已重试次数
    MaxRetry    int     `gorm:"default:3"`                     // 最大重试次数
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   gorm.DeletedAt `gorm:"index"`  // 软删除
}
```

**设计要点：**
- 使用 UUID 作为主键，避免 ID 被枚举
- `ExternalID` 存储外部 API 返回的任务 ID，用于状态查询和 webhook 匹配
- `Provider` 字段支持多 API 切换
- `RetryCount` + `MaxRetry` 实现可控重试
- 软删除保留历史记录

### 3.2 任务队列实现

**文件：** `services/digitalhuman/queue.go`

```go
// 任务队列（单例模式）
type Queue struct {
    jobs       chan Job                  // 任务通道，缓冲 100
    workers    int                       // worker 数量
    wg         sync.WaitGroup            // 等待组，用于优雅关闭
    ctx        context.Context           // 上下文，用于取消
    cancel     context.CancelFunc       // 取消函数
    apiClients map[string]APIClient     // API 客户端映射
}

// 获取全局队列（懒加载单例）
var (
    globalQueue *Queue
    once        sync.Once
)

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
        globalQueue.start()  // 启动 worker
    })
    return globalQueue
}

// 启动 worker goroutine
func (q *Queue) start() {
    for i := 0; i < q.workers; i++ {
        q.wg.Add(1)
        go q.worker(i)  // 启动 3 个 worker
    }
}

// worker 工作循环
func (q *Queue) worker(id int) {
    defer q.wg.Done()
    for {
        select {
        case job := <-q.jobs:        // 从通道消费任务
            q.processJob(job)        // 处理任务
        case <-q.ctx.Done():         // 收到取消信号
            return                   // 退出
        }
    }
}
```

**设计要点：**
- **单例模式**：全局只有一个队列实例，避免资源竞争
- **Channel 缓冲**：100 容量，允许短暂的任务堆积
- **多 Worker 并发**：3 个 goroutine 并行处理，提高吞吐量
- **优雅关闭**：通过 context + WaitGroup 实现

### 3.3 任务处理流程

```go
func (q *Queue) processJob(job Job) {
    // 1. 加载任务记录
    var video models.DigitalHumanVideo
    dao.Db.Where("id = ?", job.VideoID).First(&video)
    
    // 2. 获取对应的 API 客户端
    client := q.apiClients[video.Provider]
    
    // 3. 提交外部任务（如尚未提交）
    if video.ExternalID == "" {
        externalID, err := client.SubmitTask(ctx, &video)
        video.ExternalID = externalID
        dao.Db.Model(&video).Update("external_id", externalID)
    }
    
    // 4. 轮询状态
    q.pollStatus(ctx, &video, client)
}

// 轮询外部 API 状态
func (q *Queue) pollStatus(ctx context.Context, video *models.DigitalHumanVideo, client APIClient) {
    ticker := time.NewTicker(10 * time.Second)  // 每 10 秒查询
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            q.handleRetry(video, "polling timeout")
            return
        case <-ticker.C:
            status, videoURL, _ := client.QueryStatus(ctx, video.ExternalID)
            
            switch status {
            case models.VideoStatusCompleted:
                // 完成：存储视频 URL
                dao.Db.Model(video).Updates(map[string]interface{}{
                    "status": models.VideoStatusCompleted,
                    "video_url": videoURL,
                })
                return
            case models.VideoStatusFailed:
                q.markFailed(video, "external API failed")
                return
            // 其他状态继续轮询
            }
        }
    }
}
```

### 3.4 重试机制

```go
func (q *Queue) handleRetry(video *models.DigitalHumanVideo, reason string) {
    if video.RetryCount < video.MaxRetry {
        video.RetryCount++
        
        // 重置状态，清空 external_id
        dao.Db.Model(video).Updates(map[string]interface{}{
            "status": models.VideoStatusPending,
            "retry_count": video.RetryCount,
            "external_id": "",  // 清空，重新提交
        })
        
        // 指数退避：30s → 60s → 120s
        delay := time.Duration(30*(1<<(video.RetryCount-1))) * time.Second
        
        // 延迟后重新入队
        go func() {
            time.Sleep(delay)
            q.Enqueue(video.ID)
        }()
    } else {
        q.markFailed(video, reason)
    }
}
```

**重试策略：**
- 最大重试 3 次
- 指数退避：30s → 60s → 120s
- 每次重试清空 external_id，重新提交
- 超过最大次数标记为失败

### 3.5 API 客户端接口（策略模式）

```go
// APIClient 定义统一接口
type APIClient interface {
    SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (externalID string, err error)
    QueryStatus(ctx context.Context, externalID string) (status string, videoURL string, err error)
}

// HeyGen 实现
type HeyGenClient struct {
    apiKey  string
    baseURL string
}

func (c *HeyGenClient) SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (string, error) {
    // 构造请求体
    reqBody := heygenVideoRequest{
        VideoInputs: []heygenVideoInput{{
            Character: heygenCharacter{
                Type: "avatar",
                AvatarID: video.AvatarID,
            },
            Voice: heygenVoice{
                Type: "text",
                InputText: video.Text,
                VoiceID: video.VoiceID,
            },
        }},
    }
    
    // 发送 HTTP 请求
    resp, err := c.http.Do(req)
    // 解析返回 video_id
    return result.Data.VideoID, nil
}

func (c *HeyGenClient) QueryStatus(ctx context.Context, externalID string) (string, string, error) {
    // GET /v1/video_status.get?video_id=xxx
    // 返回 status, video_url
}
```

**设计优势：**
- 新增 API 只需实现接口，无需修改队列逻辑
- 不同 API 的差异封装在各自客户端内
- 便于单元测试（可 mock APIClient）

### 3.6 Controller 接口实现

```go
// POST /api/digital-human/videos
func (ctrl *DigitalHumanController) CreateVideo(c *gin.Context) {
    var req createVideoReq
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 参数校验
    if req.Text == "" {
        c.JSON(400, gin.H{"error": "text is required"})
        return
    }
    
    // 创建任务并入库
    video, err := digitalhuman.CreateAndEnqueue(
        userID, req.Title, req.Text, 
        req.AvatarID, req.VoiceID, req.Language, req.Provider,
    )
    
    c.JSON(201, gin.H{
        "message": "任务已提交",
        "video": toVideoResp(video),
    })
}

// GET /api/digital-human/videos/:id
func (ctrl *DigitalHumanController) GetVideo(c *gin.Context) {
    id := c.Param("id")
    var video models.DigitalHumanVideo
    dao.Db.Where("id = ?", id).First(&video)
    c.JSON(200, toVideoResp(&video))
}

// POST /api/digital-human/webhook
func (ctrl *DigitalHumanController) Webhook(c *gin.Context) {
    var payload struct {
        EventType string `json:"event_type"`
        EventData struct {
            VideoID  string `json:"video_id"`
            Status   string `json:"status"`
            VideoURL string `json:"video_url"`
        } `json:"event_data"`
    }
    c.ShouldBindJSON(&payload)
    
    // 通过 external_id 查找本地任务
    var video models.DigitalHumanVideo
    dao.Db.Where("external_id = ?", payload.EventData.VideoID).First(&video)
    
    // 更新状态
    dao.Db.Model(&video).Updates(map[string]interface{}{
        "status": models.VideoStatusCompleted,
        "video_url": payload.EventData.VideoURL,
    })
    
    c.JSON(200, gin.H{"message": "ok"})
}
```

---

## 四、前端实现

### 4.1 页面结构

```
┌─────────────────────────────────────────────────────────────┐
│                        导航栏                                │
├──────────────────┬──────────────────────────────────────────┤
│                  │                                          │
│   生成表单        │           视频播放器                      │
│   - 标题输入      │           (16:9 播放区)                  │
│   - 文本输入      │                                          │
│   - 形象选择      ├──────────────────────────────────────────┤
│   - 音色选择      │                                          │
│   - 语言选择      │           统计面板                        │
│   - API 选择      │   [等待中] [生成中] [已完成] [失败]       │
│   - 提交按钮      ├──────────────────────────────────────────┤
│                  │                                          │
│                  │           任务列表                        │
│   使用说明        │   - 筛选标签                             │
│                  │   - 任务卡片列表                          │
│                  │     - 缩略图 / 标题 / 状态                │
│                  │     - 播放 / 重试 / 删除 按钮             │
│                  │                                          │
└──────────────────┴──────────────────────────────────────────┘
```

### 4.2 状态轮询实现

```javascript
// 轮询任务状态
function startPolling(videoID) {
    if (pollingTimers[videoID]) return;  // 避免重复轮询
    
    pollingTimers[videoID] = setInterval(async () => {
        const resp = await fetch(`${API}/videos/${videoID}`);
        const v = await resp.json();
        
        if (v.status === 'completed' || v.status === 'failed') {
            clearInterval(pollingTimers[videoID]);
            delete pollingTimers[videoID];
            
            await loadTasks();  // 刷新列表
            
            if (v.status === 'completed') {
                showToast(`"${v.title}" 生成完成！`, 'success');
                playVideo(v.id);  // 自动播放
            }
        } else {
            // 更新状态徽章
            updateStatusBadge(videoID, v.status);
        }
    }, 8000);  // 每 8 秒轮询
}
```

### 4.3 关键交互

| 交互 | 实现方式 |
|------|---------|
| 提交任务 | fetch POST → 入库 → 入队 → 开始轮询 |
| 状态更新 | 定时轮询 + 徽章动画 |
| 视频播放 | HTML5 video + blob URL |
| 删除确认 | 模态框 + 二次确认 |
| 错误提示 | Toast 通知 |

---

## 五、关键技术决策

### 5.1 为什么用 Go channel 而不是 Redis Queue？

| 方案 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| Go channel | 零依赖、简单、低延迟 | 单机、不持久化 | 中小规模、快速验证 |
| Redis Queue | 分布式、持久化、可视 | 运维复杂 | 大规模、高可用 |

**决策：** 当前阶段使用 Go channel，后续可无缝迁移到 Redis。

### 5.2 为什么用轮询而不是 WebSocket？

| 方案 | 优点 | 缺点 |
|------|------|------|
| 轮询 | 简单可靠、兼容性好 | 有延迟、浪费带宽 |
| WebSocket | 实时、省带宽 | 复杂、需要维护连接 |

**决策：** 视频生成是分钟级任务，8 秒轮询延迟可接受，且实现简单。

### 5.3 为什么支持多 API 提供商？

1. **商业 API（HeyGen）**：快速上线，质量稳定
2. **开源方案（SadTalker）**：可控成本，数据安全
3. **自定义模型**：对接 ML 团队，持续优化

通过策略模式，切换 API 只需修改 `provider` 参数。

---

## 六、性能与扩展性

### 6.1 性能指标

| 指标 | 当前值 | 优化方向 |
|------|--------|---------|
| 任务入队延迟 | < 1ms | - |
| Worker 并发数 | 3 | 可配置化 |
| 轮询间隔 | 10s | 可根据 API 调整 |
| 队列容量 | 100 | 可扩容 |

### 6.2 扩展方向

1. **水平扩展**：
   - 将 channel 替换为 Redis Queue
   - 多实例部署，共享队列

2. **功能扩展**：
   - 支持用户上传自定义形象
   - 支持批量生成
   - 支持视频编辑

3. **监控告警**：
   - 队列积压告警
   - 失败率监控
   - API 调用限流

---

## 七、测试用例

### 7.1 单元测试

```go
func TestQueue_Enqueue(t *testing.T) {
    queue := GetQueue()
    
    // 创建测试任务
    video := &models.DigitalHumanVideo{
        ID: uuid.New().String(),
        Title: "Test",
        Text: "Test content",
        Status: models.VideoStatusPending,
    }
    dao.Db.Create(video)
    
    // 入队
    queue.Enqueue(video.ID)
    
    // 验证任务被处理
    time.Sleep(2 * time.Second)
    var updated models.DigitalHumanVideo
    dao.Db.First(&updated, "id = ?", video.ID)
    assert.NotEqual(t, models.VideoStatusPending, updated.Status)
}
```

### 7.2 集成测试

```bash
# 提交任务
curl -X POST http://localhost:9090/api/digital-human/videos \
  -H "Content-Type: application/json" \
  -d '{"title":"Test","text":"Hello world","provider":"heygen"}'

# 查询状态
curl http://localhost:9090/api/digital-human/videos/{id}

# 重试
curl -X POST http://localhost:9090/api/digital-human/videos/{id}/retry

# 删除
curl -X DELETE http://localhost:9090/api/digital-human/videos/{id}
```

---

## 八、总结

### 8.1 技术亮点

1. **异步任务队列**：Go channel + worker 模式，简洁高效
2. **策略模式**：多 API 提供商无缝切换
3. **重试机制**：指数退避，提高成功率
4. **Webhook 支持**：支持外部 API 主动推送
5. **前端体验**：实时状态更新、自动播放

### 8.2 可改进点

1. 添加用户认证与权限控制
2. 视频本地存储与 CDN 分发
3. 任务优先级队列
4. 更完善的监控与告警

---

## 附录：文件清单

| 文件路径 | 代码行数 | 说明 |
|---------|---------|------|
| `models/digital_human.go` | ~70 | 数据模型 |
| `services/digitalhuman/queue.go` | ~180 | 任务队列 |
| `services/digitalhuman/clients.go` | ~250 | API 客户端 |
| `controllers/digital_human.go` | ~200 | HTTP 接口 |
| `frontend/digitalhuman.html` | ~700 | 前端页面 |
| `router/router.go` | +15 | 路由注册 |
| `dao/dao.go` | +2 | 数据库迁移 |
