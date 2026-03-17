# 慧备灵师 — AI 教育平台

面向教师的 AI 辅助教学系统，获中国大学生服务创新创业外包大赛**国家一等奖**（A03 AI备课应用赛道）。

## 版本历史

### v1.1 — 对话助手增强

基于 v1.0 的三大核心模块，本次迭代重点强化了**教学对话助手**的检索质量与记忆能力。

#### 新功能

**1. 向量数据库支持（Milvus）**

引入 Milvus 向量数据库，实现语义检索能力，解决原有关键词检索无法理解同义词和用户意图的问题。

- 新增 `services/rag/embedding.go`：调用 OpenAI `text-embedding-3-small` 接口将文本转为 1536 维向量，支持批量处理与带重试机制的单条获取
- 新增 `services/rag/milvus.go`：封装 Milvus Go SDK，实现集合管理、向量插入、ANN 搜索（AUTOINDEX + L2 距离）和按分类过滤
- 改造 `services/rag/retriever.go`：在原有关键词检索基础上增加向量检索路径，实现**混合检索**（向量权重 0.6 + 关键词权重 0.4），Milvus 未启用时自动回退至关键词检索
- 新增 `scripts/import_vectors.go`：一次性将 MySQL 知识库批量向量化并导入 Milvus，支持 `-batch`、`-drop`、`-dry-run` 参数
- 新增 `docker-compose.milvus.yml`：Milvus Standalone 一键部署配置（含 etcd + MinIO + Milvus）

**2. Redis 缓存层**

新增 `services/cache/cache.go`，基于 `go-redis/v8` 封装通用缓存服务，并应用于两处高频操作：

- **Embedding 缓存**（TTL 24 小时）：相同文本不重复调用 Embedding API，节省调用成本
- **检索结果缓存**（TTL 10 分钟）：相同查询直接命中缓存，平均响应时间从 500ms 降至 50ms 以内
- Redis 不可用时自动降级，不影响主流程
- 新增 `controllers/cache.go` 及 `/api/cache/*` 路由，提供命中率统计、手动清除等管理接口

**3. 时间线查询工具**

补全了原 `get_timeline` MCP 工具的实现（此前返回空结果）。

- 新增 `services/timeline/timeline.go`：基于 `KnowledgeChunk` 表中的 `year_start`/`year_end` 字段，实现按年份范围、诸侯国、事件类型的组合查询，支持按时期（春秋/战国）全量拉取，结果按年份升序排列
- 更新 `services/mcp/tools.go`：将 `executeGetTimeline` 替换为真实查询逻辑，支持 `start_year`、`end_year`、`state`、`category`、`period` 五个参数

**4. 上下文记忆增强**

全面重构 `services/memory/session.go`，引入消息语义标注体系：

- **消息标签（Tag）**：每条消息在存储时自动分类为 `question`/`answer`/`follow_up`/`tool_call`/`summary`/`greeting`/`thanks` 等 11 种标签
- **重要性评分（Importance）**：基于标签类型、内容长度、关键实体命中，为每条消息评分 1–10，问题类消息默认 8 分，问候/感谢类默认 2 分
- **实体提取（Entities）**：自动识别消息中的历史人物、诸侯国、战役名称和年份，以 JSON 存储于消息记录
- **智能压缩**：超过 10 条消息时触发压缩，优先保留最近 5 条 + 高重要性消息，对其余消息生成结构化摘要（主要话题 / 关键问题 / 重要结论 / 涉及实体），历史消息标记 `is_summarized = true`
- **标题自动生成**：首条消息存在可识别实体时，自动生成"关于 XX 的讨论"格式标题

#### 数据库变更

`sessions` 表新增列：`summary`、`topics`、`key_entities`、`message_count`

`messages` 表新增列：`tag`、`importance`、`topics`、`entities`、`is_summarized`

（通过 GORM AutoMigrate 自动执行，无需手动迁移）

#### 技术栈更新

| 层 | v1.0 | v1.1 新增 |
|----|------|-----------|
| 向量检索 | — | Milvus 2.3 + text-embedding-3-small |
| 缓存 | — | Redis 7 (go-redis/v8) |
| 时间线查询 | 占位符 | 完整实现 |
| 上下文记忆 | 窗口截取 | 标签 + 重要性 + 实体 + 智能压缩 |

#### 新增环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MILVUS_HOST` | `localhost` | Milvus 地址 |
| `MILVUS_PORT` | `19530` | Milvus 端口 |
| `MILVUS_COLLECTION` | `knowledge_vectors` | 集合名 |
| `MILVUS_DIMENSION` | `1536` | 向量维度 |
| `MILVUS_ENABLED` | `false` | 是否启用向量检索 |
| `EMBEDDING_PROVIDER` | `openai` | Embedding 提供商 |
| `EMBEDDING_MODEL` | `text-embedding-3-small` | Embedding 模型 |

#### 新增文档

- [`AIdocs/向量数据库实现详解.md`](./AIdocs/向量数据库实现详解.md)：向量原理、字段设计、混合检索、缓存策略全解
- [`AIdocs/上下文记忆系统设计.md`](./AIdocs/上下文记忆系统设计.md)：标签体系、重要性评分、智能压缩流程设计

---

### v1.0 — 初始版本

## 项目描述

系统包含三个核心模块，均已有完整后端实现：

1. **教学对话助手**：基于春秋战国历史知识库做 RAG 检索与大模型生成，支持多轮对话、知识图谱查询和联网搜索
2. **备课工作流**：完成大纲生成与对话式修改、教案生成、PPT 生成的端到端备课流水线
3. **数字人视频生成**：对接 HeyGen / SadTalker / 自定义模型，通过异步任务队列生成讲课视频

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.21 + Gin |
| 数据库 | MySQL（GORM，FULLTEXT 索引）|
| LLM | DeepSeek API（OpenAI 兼容）|
| 联网搜索 | Serper API |
| 数字人 | HeyGen / SadTalker / 自定义 ML 模型 |
| 前端 | 静态 HTML（`/frontend`）|
| 配置 | `.env` 环境变量 |

## 项目结构

```
AIEducation/
├── backend/
│   ├── main.go                   # 入口，端口 9090
│   ├── config/config.go          # 环境变量配置
│   ├── router/router.go          # 路由注册
│   ├── controllers/
│   │   ├── dialog.go             # 教学对话助手控制器（SSE）
│   │   ├── lessonprep.go         # 备课工作流控制器（SSE）
│   │   └── digital_human.go     # 数字人控制器
│   ├── services/
│   │   ├── llm/client.go         # DeepSeek 客户端（流式/非流式）
│   │   ├── rag/retriever.go      # RAG 检索（FULLTEXT + LIKE + BM25）
│   │   ├── mcp/tools.go          # MCP 工具执行器（4 个工具）
│   │   ├── memory/session.go     # 会话管理（窗口记忆 + 摘要压缩）
│   │   ├── graph/query.go        # 知识图谱查询
│   │   ├── lessonprep/
│   │   │   ├── workflow.go       # 工作流管理器（DB 状态机）
│   │   │   ├── outline.go        # 大纲生成（RAG + LLM 流式）
│   │   │   ├── outline_edit.go   # 大纲对话式修改
│   │   │   ├── lessonplan.go     # 教案生成（Markdown）
│   │   │   └── pptgen.go         # PPT 生成（原生 OOXML）
│   │   └── digitalhuman/
│   │       ├── queue.go          # 任务队列（Go channel + 3 worker）
│   │       └── clients.go        # HeyGen/SadTalker/Custom 客户端
│   ├── models/                   # GORM 数据模型
│   └── scripts/                  # 知识库/知识图谱导入脚本
├── frontend/                     # 静态前端页面
└── AIdocs/                       # 模块技术文档
    ├── 教学对话助手/
    ├── 备课工作流/
    └── 数字人/
```

## 快速启动

```bash
# 复制配置
cp .env.example .env
# 填写必要配置（见下方环境变量说明）

cd backend
go mod tidy
go run main.go
# 服务启动在 :9090
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DEEPSEEK_API_KEY` | — | **必填**，DeepSeek API Key |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com` | API 地址（可换兼容接口）|
| `DEEPSEEK_MODEL` | `deepseek-chat` | 使用的模型 |
| `MYSQL_HOST` | `localhost` | MySQL 主机 |
| `MYSQL_PORT` | `3306` | MySQL 端口 |
| `MYSQL_USER` | `root` | 用户名 |
| `MYSQL_PASSWORD` | — | 密码 |
| `MYSQL_DATABASE` | `goedu` | 数据库名 |
| `SERPER_API_KEY` | — | 联网搜索（可选）|
| `HEYGEN_API_KEY` | — | HeyGen 数字人（可选）|
| `SADTALKER_BASE_URL` | `http://localhost:10364` | SadTalker 服务地址（可选）|
| `CUSTOM_DH_BASE_URL` | `http://localhost:8080` | 自定义 ML 模型地址（可选）|

## 核心 API

### 教学对话助手

```
POST /api/dialog/chat          SSE 流式问答（支持工具调用）
GET  /api/dialog/history       会话历史
GET  /api/dialog/sessions      会话列表
POST /api/dialog/search        直接搜索知识库
```

### 备课工作流

```
POST /api/lessonprep/start              输入主题 → SSE 流式生成大纲
POST /api/lessonprep/outline/edit       对话式修改大纲（SSE）
POST /api/lessonprep/outline/confirm    确认大纲
POST /api/lessonprep/plan/generate      生成教案（SSE）
POST /api/lessonprep/ppt/generate       生成 PPT（SSE）
GET  /api/lessonprep/ppt/download       下载 .pptx 文件
GET  /api/lessonprep/status             工作流状态
```

### 数字人视频生成

```
POST   /api/digital-human/videos           提交文本，触发生成任务
GET    /api/digital-human/videos/:id       查询任务状态 + 视频 URL
POST   /api/digital-human/videos/:id/retry 手动重试
DELETE /api/digital-human/videos/:id       取消任务
POST   /api/digital-human/webhook          外部回调（HeyGen 等）
GET    /api/digital-human/avatars          获取可用形象列表
```

## 项目职责

1. **教学对话助手**：负责 SSE 对话接口（`POST /api/dialog/chat`）与整体请求链路编排——串联 MCP 工具（`search_knowledge`/`query_graph`/`web_search`）、MySQL FULLTEXT RAG 检索、DeepSeek LLM 流式调用；实现基于 DB 的会话管理（窗口截取 20 条 + LLM 摘要压缩）与对话日志落库，支持 local/online/auto 三种模式切换。

2. **备课工作流**：负责大纲生成（RAG + LLM 流式）、对话式大纲修改（多轮上下文）、教案生成（Markdown，含教师话术与史料引用）、PPT 生成（LLM 规划页面结构 + 原生 OOXML 构建 .pptx）；使用 DB 状态机管理工作流（9 个状态，UUID 主键），通过 SSE 实时推送进度。

3. **数字人视频生成**：负责「文本 → 数字人讲课视频」的工程化链路；对接 HeyGen（商业）/ SadTalker（开源）/ Custom（ML 团队）三种 Provider，通过 Go channel 队列（容量 100，3 个 worker）+ 10s 轮询 + 指数退避重试（30s/60s/120s，最多 3 次）管理异步任务，支持 Webhook 回调和手动重试。

## 文档

详细的技术方案与实现细节见 `AIdocs/` 目录：

- [`AIdocs/教学对话助手/`](./AIdocs/教学对话助手/README.md)：对话链路、RAG 实战、MCP 工具、上下文记忆、LLM 交互
- [`AIdocs/备课工作流/`](./AIdocs/备课工作流/README.md)：状态机设计、大纲/教案/PPT 生成实现
- [`AIdocs/数字人/`](./AIdocs/数字人/README.md)：三种 Provider 实现、任务队列、重试策略
