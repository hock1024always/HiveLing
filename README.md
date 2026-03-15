# 慧备灵师 — AI 教育平台

面向教师的 AI 辅助教学系统，获中国大学生服务创新创业外包大赛**国家一等奖**（A03 AI备课应用赛道）。

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
