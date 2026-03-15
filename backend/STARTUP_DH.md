# 数字人视频生成模块 - 启动与开发指南

> 本文档用于快速启动数字人视频生成功能，以及后续开发的参考。

---

## 一、环境要求

| 组件 | 版本要求 | 说明 |
|------|---------|------|
| Go | 1.24+ | 后端运行环境 |
| MySQL | 5.7+ / 8.0 | 数据存储 |
| Node.js | 16+ | 前端开发（可选） |

---

## 二、目录结构

```
backend/
├── models/
│   └── digital_human.go          # 数据模型定义
├── controllers/
│   └── digital_human.go          # HTTP 接口控制器
├── services/
│   └── digitalhuman/
│       ├── queue.go              # 任务队列与 worker
│       └── clients.go            # API 客户端封装
├── dao/
│   └── dao.go                    # 数据库迁移（已添加新表）
└── router/
    └── router.go                 # 路由注册（已添加新路由）

frontend/
└── digitalhuman.html             # 前端页面
```

---

## 三、配置说明

### 3.1 环境变量

在 `backend/.env` 文件中添加以下配置：

```bash
# ===== HeyGen API（商业方案）=====
HEYGEN_API_KEY=your_heygen_api_key
HEYGEN_DEFAULT_AVATAR_ID=your_default_avatar_id
HEYGEN_DEFAULT_VOICE_ID=your_default_voice_id

# ===== SadTalker API（开源方案）=====
SADTALKER_BASE_URL=http://localhost:10364

# ===== 自定义模型 API（ML 团队）=====
CUSTOM_DH_BASE_URL=http://your-ml-api:8080
CUSTOM_DH_API_KEY=your_custom_api_key

# ===== 数据库（已有配置）=====
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=your_password
MYSQL_DATABASE=goedu
```

### 3.2 数据库表

启动后自动创建以下表：

```sql
-- 数字人视频任务表
CREATE TABLE `digital_human_videos` (
  `id` varchar(64) PRIMARY KEY,
  `user_id` int unsigned,
  `title` varchar(255),
  `text` longtext,
  `avatar_id` varchar(128),
  `voice_id` varchar(128),
  `language` varchar(20) DEFAULT 'zh-CN',
  `provider` varchar(32) DEFAULT 'heygen',
  `external_id` varchar(128),
  `status` varchar(32) DEFAULT 'pending',
  `video_url` varchar(1024),
  `local_path` varchar(512),
  `duration` float,
  `resolution` varchar(32),
  `file_size` bigint,
  `error_msg` text,
  `retry_count` int DEFAULT 0,
  `max_retry` int DEFAULT 3,
  `callback_url` varchar(512),
  `metadata` json,
  `created_at` datetime,
  `updated_at` datetime,
  `deleted_at` datetime,
  INDEX `idx_user_id` (`user_id`),
  INDEX `idx_external_id` (`external_id`),
  INDEX `idx_deleted_at` (`deleted_at`)
);

-- 数字人形象表
CREATE TABLE `digital_human_avatars` (
  `id` int PRIMARY KEY AUTO_INCREMENT,
  `avatar_id` varchar(128) UNIQUE,
  `name` varchar(100),
  `description` varchar(500),
  `preview_url` varchar(1024),
  `provider` varchar(32),
  `is_active` tinyint(1) DEFAULT 1,
  `created_at` datetime,
  `updated_at` datetime
);
```

---

## 四、启动步骤

### 4.1 启动后端

```bash
# 进入后端目录
cd /home/hyz/AI/AIEducation/backend

# 安装依赖（首次）
go mod tidy

# 启动服务
go run main.go

# 或编译后运行
go build -o goedu && ./goedu
```

服务启动后监听 `:9090` 端口。

### 4.2 访问前端

浏览器打开：`http://localhost:9090/app/digitalhuman.html`

### 4.3 验证服务

```bash
# 检查 API 是否正常
curl http://localhost:9090/api/digital-human/avatars

# 检查视频列表
curl http://localhost:9090/api/digital-human/videos

# 提交测试任务
curl -X POST http://localhost:9090/api/digital-human/videos \
  -H "Content-Type: application/json" \
  -d '{
    "title": "测试视频",
    "text": "同学们好，今天我们来学习春秋战国时期的历史。",
    "avatar_id": "default_teacher_male",
    "voice_id": "zh_male_1",
    "language": "zh-CN",
    "provider": "heygen"
  }'
```

---

## 五、API 接口清单

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/digital-human/videos` | 提交生成任务 |
| GET | `/api/digital-human/videos` | 任务列表（分页） |
| GET | `/api/digital-human/videos/:id` | 查询单任务 |
| POST | `/api/digital-human/videos/:id/retry` | 重试失败任务 |
| DELETE | `/api/digital-human/videos/:id` | 取消/删除任务 |
| POST | `/api/digital-human/webhook` | 接收外部 API 回调 |
| GET | `/api/digital-human/avatars` | 获取数字人形象列表 |

### 5.1 提交任务请求体

```json
{
  "title": "视频标题",
  "text": "讲课文本内容",
  "avatar_id": "数字人形象ID",
  "voice_id": "音色ID",
  "language": "zh-CN",
  "provider": "heygen"
}
```

### 5.2 任务状态流转

```
pending → processing → completed
    ↓          ↓
  failed    failed
    ↓
  cancelled
```

---

## 六、切换 API 提供商

### 6.1 使用 HeyGen（商业）

1. 注册 HeyGen 账号：https://www.heygen.com
2. 获取 API Key
3. 配置环境变量 `HEYGEN_API_KEY`
4. 提交任务时 `provider: "heygen"`

### 6.2 使用 SadTalker（开源）

1. 部署 SadTalker 服务：
```bash
git clone https://github.com/kenwaytis/faster-SadTalker-API
cd faster-SadTalker-API
docker-compose up -d
```

2. 配置环境变量 `SADTALKER_BASE_URL=http://localhost:10364`
3. 提交任务时 `provider: "sadtalker"`

### 6.3 接入 ML 团队自定义模型

1. ML 团队需提供符合以下接口规范的 API：

```
POST /api/v1/digital-human/generate
请求体: { "text": "...", "avatar_id": "...", "voice_id": "...", "language": "..." }
响应: { "task_id": "xxx" }

GET /api/v1/digital-human/task/:task_id
响应: { "task_id": "xxx", "status": "success|error|running|queued", "video_url": "..." }
```

2. 配置环境变量：
```bash
CUSTOM_DH_BASE_URL=http://your-ml-api:8080
CUSTOM_DH_API_KEY=your_key
```

3. 提交任务时 `provider: "custom"`

---

## 七、常见问题

### Q1: 任务一直处于 pending 状态？

检查：
1. 任务队列是否启动（查看日志是否有 `[Queue] Worker X started`）
2. API 客户端是否配置正确（环境变量）
3. 外部 API 是否可达

### Q2: 任务失败如何排查？

1. 查看数据库 `digital_human_videos.error_msg` 字段
2. 查看后端日志
3. 手动调用外部 API 验证

### Q3: 如何增加 worker 数量？

修改 `services/digitalhuman/queue.go`:

```go
globalQueue = &Queue{
    jobs:       make(chan Job, 100),
    workers:    5,  // 改为 5 个 worker
    // ...
}
```

### Q4: 如何修改重试策略？

修改 `models/digital_human.go` 中的默认值：

```go
MaxRetry: 3,  // 最大重试次数
```

或修改 `queue.go` 中的退避时间：

```go
delay := time.Duration(30*(1<<(video.RetryCount-1))) * time.Second
// 30s → 60s → 120s
```

---

## 八、后续开发指南

### 8.1 添加新的 API 提供商

1. 在 `clients.go` 中实现 `APIClient` 接口：

```go
type MyNewClient struct { ... }

func (c *MyNewClient) SubmitTask(ctx context.Context, video *models.DigitalHumanVideo) (string, error) {
    // 调用新 API
}

func (c *MyNewClient) QueryStatus(ctx context.Context, externalID string) (string, string, error) {
    // 查询状态
}
```

2. 在 `queue.go` 的 `GetQueue()` 中注册：

```go
globalQueue.apiClients["mynew"] = NewMyNewClient()
```

3. 在 `models/digital_human.go` 添加常量：

```go
const APIProviderMyNew = "mynew"
```

### 8.2 添加用户认证

在 `router.go` 中添加中间件：

```go
dh := r.Group("/api/digital-human")
dh.Use(authMiddleware())  // 添加认证中间件
{
    dh.POST("/videos", dhCtrl.CreateVideo)
    // ...
}
```

修改 `controllers/digital_human.go` 获取用户 ID：

```go
func (ctrl *DigitalHumanController) CreateVideo(c *gin.Context) {
    userID := c.GetUint("user_id")  // 从中间件获取
    // ...
}
```

### 8.3 添加视频本地存储

在 `queue.go` 的 `pollStatus` 中添加下载逻辑：

```go
case models.VideoStatusCompleted:
    // 下载视频到本地
    localPath, err := downloadVideo(video.VideoURL, video.ID)
    if err == nil {
        dao.Db.Model(video).Update("local_path", localPath)
    }
```

---

## 九、日志与监控

### 9.1 关键日志

```
[Queue] Worker 0 started              # Worker 启动
[Queue] Enqueued job: xxx             # 任务入队
[Queue] Processing job: xxx           # 开始处理
[Queue] Video xxx status: processing  # 状态轮询
[Queue] Video xxx completed: http://  # 生成完成
[Queue] Video xxx failed: ...         # 生成失败
```

### 9.2 监控指标

可监控以下指标：
- 队列长度：`len(queue.jobs)`
- 处理中任务数：数据库 `status='processing'` count
- 成功率：`completed / total`
- 平均生成时间：`completed_at - created_at`

---

## 十、快速命令参考

```bash
# 编译
cd /home/hyz/AI/AIEducation/backend && go build $(go list ./... | grep -v "scripts")

# 检查代码
go vet $(go list ./... | grep -v "scripts")

# 查看数据库
mysql -u root -p -e "SELECT id, title, status, provider FROM goedu.digital_human_videos ORDER BY created_at DESC LIMIT 10;"

# 清空测试数据
mysql -u root -p -e "DELETE FROM goedu.digital_human_videos;"

# 查看日志
tail -f /home/hyz/AI/AIEducation/backend/runtime/logs/*.log
```
