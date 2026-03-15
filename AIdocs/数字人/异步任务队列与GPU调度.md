# 数字人 — 异步任务队列与重试调度实现

## 一、整体方案选型

本项目采用**进程内 Go channel 队列 + 多 worker goroutine**，而非 Redis Streams / Kafka：

| 方案 | 优势 | 劣势 | 本项目选择理由 |
|------|------|------|---------------|
| Go channel queue | 零依赖，部署极简 | 服务重启任务丢失 | 任务已持久化到 DB，重启后可扫描 pending 任务重新入队 |
| Redis Streams | 持久化、跨进程 | 需额外组件 | 小规模不必要 |
| Kafka | 高吞吐 | 运维成本高 | 远超当前规模需求 |

> 持久化由 MySQL 保证，channel 只是调度缓冲层。

---

## 二、Queue 实现细节（`services/digitalhuman/queue.go`）

### 初始化

```go
type Queue struct {
    jobs       chan Job                    // 容量 100 的 buffered channel
    workers    int                         // 并发 worker 数，默认 3
    ctx        context.Context             // 用于优雅停止
    cancel     context.CancelFunc
    apiClients map[string]APIClient        // provider → 具体客户端
}
```

三种 Provider 客户端在单例初始化时注册：

```go
globalQueue.apiClients[models.APIProviderHeyGen]    = NewHeyGenClient()
globalQueue.apiClients[models.APIProviderSadTalker] = NewSadTalkerClient()
globalQueue.apiClients[models.APIProviderCustom]    = NewCustomClient()
```

### Worker goroutine

```go
func (q *Queue) worker(id int) {
    for {
        select {
        case job := <-q.jobs:
            q.processJob(job)
        case <-q.ctx.Done():
            return
        }
    }
}
```

`Stop()` 调用 `cancel()` + `wg.Wait()` 实现优雅关闭。

---

## 三、任务处理流程（`processJob`）

```
读取 DB 中的 DigitalHumanVideo
    ↓
已 completed/cancelled → 跳过
    ↓
根据 provider 选取 APIClient（策略模式）
    ↓
状态置 processing
    ↓
ExternalID 为空 → SubmitTask → 写 external_id 到 DB
    ↓
pollStatus 轮询（每 10s，context 超时 10min）
    ├─ completed → 写 video_url，状态置 completed
    ├─ failed    → handleRetry
    └─ pending/processing → 继续等下一次 tick
```

### 轮询实现

```go
ticker := time.NewTicker(10 * time.Second)
for {
    select {
    case <-ctx.Done():      // 10min 超时 → handleRetry
        ...
    case <-ticker.C:
        status, videoURL, _ := client.QueryStatus(ctx, video.ExternalID)
        switch status {
        case "completed": 保存 video_url，return
        case "failed":    markFailed，return
        // pending/processing 继续轮询
        }
    }
}
```

---

## 四、重试策略

### 自动重试（指数退避）

```go
func (q *Queue) handleRetry(video *models.DigitalHumanVideo, reason string) {
    if video.RetryCount < video.MaxRetry {
        video.RetryCount++
        // 延迟: 30s → 60s → 120s
        delay := time.Duration(30*(1<<(video.RetryCount-1))) * time.Second
        // 清空 external_id，重新走提交流程
        dao.Db.Model(video).Updates(map[string]interface{}{
            "status":      "pending",
            "retry_count": video.RetryCount,
            "external_id": "",
        })
        go func() {
            time.Sleep(delay)
            q.Enqueue(video.ID)
        }()
    } else {
        q.markFailed(video, reason)
    }
}
```

### 手动重试

`POST /api/digital-human/videos/:id/retry`（控制器 `controllers/digital_human.go:162`）：

```go
dao.Db.Model(&video).Updates(map[string]interface{}{
    "status":      "pending",
    "retry_count": 0,         // 重置计数
    "external_id": "",
})
digitalhuman.GetQueue().Enqueue(video.ID)
```

---

## 五、幂等性保证

- 每次 `SubmitTask` 前检查 `ExternalID` 是否已存在，若存在则跳过提交直接进入轮询
- `handleRetry` 重置 `external_id` = `""`，确保重试时重新提交（避免沿用已失效的外部 ID）
- 任务状态已落库，服务重启后可扫表将 `processing` 状态的任务重新入队

---

## 六、APIClient 策略接口

```go
type APIClient interface {
    SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (externalID string, err error)
    QueryStatus(ctx context.Context, externalID string) (status string, videoURL string, err error)
}
```

支持 `RegisterClient` 动态替换客户端（便于测试 Mock 和热切换 Provider）：

```go
func (q *Queue) RegisterClient(provider string, client APIClient) {
    q.apiClients[provider] = client
}
```

---

## 七、观测指标建议

| 指标 | 来源 | 说明 |
|------|------|------|
| 队列积压深度 | `len(q.jobs)` | 实时 channel 长度 |
| 任务成功率 | DB 聚合 | `completed / total` 按时间窗口 |
| 平均端到端耗时 | DB `updated_at - created_at` | 按 provider 分组 |
| 重试次数分布 | DB `retry_count` 字段 | 识别特定 Provider 稳定性 |
| Worker 阻塞 | goroutine 监控 | 3 个 worker 是否长期占满 |
