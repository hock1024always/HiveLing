# AIEducation 后端 — 启动与开发文档

> 本文档供开发者（含 AI 助手）快速了解项目结构、完成首次部署、进行后续开发使用。
> 项目地址：`/home/hyz/AI/AIEducation/backend`

---

## 一、项目概览

**慧备灵师** 是面向历史教师的 AI 辅助教学平台，后端基于 Go + Gin + MySQL + DeepSeek API 构建，当前实现两个核心功能：

| 功能模块 | 描述 | 入口路由 |
|----------|------|----------|
| **春秋战国对话助手** | RAG + 知识图谱 + 工具调用的历史问答 | `/api/dialog/*` |
| **AI 备课工作流** | 需求→大纲→教案→PPT 的交互式备课流程 | `/api/lessonprep/*` |

---

## 二、目录结构

```
backend/
├── main.go                          # 入口：加载 .env → 初始化 DB → 启动路由
├── .env                             # 敏感配置（不入 git，下方有模板）
├── go.mod / go.sum                  # Go 模块依赖
├── goedu                            # 已编译的历史版本二进制（可能与当前代码不对应）
│
├── config/
│   └── config.go                    # 统一配置加载（从环境变量读取）
│
├── dao/
│   ├── dao.go                       # MySQL 连接 + GORM AutoMigrate（建表）
│   └── user_dao.go                  # 用户 CRUD（从 models 拆出，消除循环依赖）
│
├── models/
│   ├── user.go                      # User / UserApi / JWT 工具函数
│   ├── session.go                   # Session / Message（对话历史）
│   ├── knowledge.go                 # KnowledgeChunk（RAG 知识块）
│   ├── graph.go                     # KGNode / KGEdge（知识图谱）
│   ├── workflow.go                  # LessonWorkflow / WorkflowMessage / 大纲结构体（备课工作流）
│   └── msg.go                       # LegacyMessage / AIRequest（旧版 OpenRouter 调用，保留兼容）
│
├── services/
│   ├── llm/
│   │   └── client.go                # DeepSeek API 客户端（Chat / StreamChat / BuildTools）
│   ├── rag/
│   │   └── retriever.go             # RAG 检索（FULLTEXT → LIKE 降级 → BM25 打分）
│   ├── memory/
│   │   └── session.go               # 会话管理（上下文构建 / 历史摘要压缩）
│   ├── graph/
│   │   └── query.go                 # 知识图谱查询
│   ├── mcp/
│   │   └── tools.go                 # MCP 工具执行器（search_knowledge / query_graph / get_timeline / web_search）
│   └── lessonprep/                  # 备课工作流（新模块）
│       ├── workflow.go              # 状态机 CRUD（Create / GetWorkflow / UpdateStatus / SaveOutline 等）
│       ├── outline.go               # 大纲生成（RAG 检索 + LLM 生成 JSON 大纲 / 流式）
│       ├── outline_edit.go          # 大纲对话式修改（多轮历史 + 流式）
│       ├── lessonplan.go            # 教案生成（基于大纲 + RAG / 流式 Markdown）
│       └── pptgen.go               # PPT 生成（纯 Go 写 OOXML ZIP，无第三方依赖）
│
├── controllers/
│   ├── dialog.go                    # 对话助手 SSE 控制器
│   ├── lessonprep.go                # 备课工作流 SSE 控制器（新）
│   └── ...                          # 原有控制器（user / ppt / ai / student / static）
│
├── router/
│   └── router.go                    # 路由注册（/api/dialog + /api/lessonprep）
│
├── knowledge/                       # 知识库 JSON 原始数据
│   ├── states.json                  # 12 个诸侯国
│   ├── people.json                  # 18 位历史人物
│   ├── events.json                  # 12 个重大事件/战役
│   ├── schools.json                 # 10 个思想流派
│   └── graph_seed.json              # 知识图谱种子（48 节点 + 50+ 关系边）
│
├── scripts/
│   ├── import_knowledge.go          # 一次性脚本：导入知识块到 MySQL
│   └── import_graph.go             # 一次性脚本：导入知识图谱到 MySQL
│
├── runtime/
│   └── ppt/                         # PPT 文件输出目录（运行时生成）
│
└── pkg/
    ├── cors/                         # 跨域中间件
    ├── logger/                       # 日志中间件
    └── mail/                         # 邮件发送工具
```

---

## 三、环境要求

| 组件 | 要求 | 当前状态 |
|------|------|----------|
| **Go** | >= 1.22 | 已安装 go1.22.5 于 `/usr/local/go` |
| **MySQL** | >= 5.7，支持 utf8mb4 | 需自行确认 |
| **DeepSeek API Key** | 有效 Key | 已在 `.env` 中配置 |
| **Serper API Key** | 联网搜索时需要 | 可选，不影响本地模式 |

> **每次新 shell 需要执行**（或写入 `~/.bashrc`）：
> ```bash
> export PATH=/usr/local/go/bin:$PATH
> ```

---

## 四、首次部署步骤

### 4.1 确认 Go 版本

```bash
export PATH=/usr/local/go/bin:$PATH
go version
# 期望输出：go version go1.22.5 linux/amd64
```

如果版本不对或没有 Go，执行：
```bash
wget https://golang.google.cn/dl/go1.22.5.linux-amd64.tar.gz -O /tmp/go.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz
```

### 4.2 配置 Go 模块代理

```bash
export PATH=/usr/local/go/bin:$PATH
go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,direct
go env -w GONOSUMDB='*'
go env -w GOFLAGS=-mod=mod
```

### 4.3 配置 .env

```bash
vi /home/hyz/AI/AIEducation/backend/.env
```

最小配置（必填项）：

```env
# DeepSeek API（已填好）
DEEPSEEK_API_KEY=sk-1384d1c471764322abdffc304dd4e769
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-chat

# MySQL（修改为实际密码）
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=你的MySQL密码
MYSQL_DATABASE=goedu

# Serper（联网搜索，可暂时留空）
SERPER_API_KEY=
SERPER_BASE_URL=https://google.serper.dev

# 服务器
SERVER_PORT=9090
GIN_MODE=debug
```

### 4.4 创建数据库

```bash
mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS goedu DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

### 4.5 下载依赖并编译

```bash
cd /home/hyz/AI/AIEducation/backend
export PATH=/usr/local/go/bin:$PATH
go mod tidy
go build -o goedu .
```

### 4.6 启动服务（首次）

```bash
cd /home/hyz/AI/AIEducation/backend
export PATH=/usr/local/go/bin:$PATH
go run main.go
```

启动时 GORM AutoMigrate 会自动创建所有表（包括 `lesson_workflows`、`workflow_messages`）。正常输出：
```
Database connected and migrated successfully
Server starting on port 9090...
[GIN-debug] POST   /api/dialog/chat          --> ...
[GIN-debug] POST   /api/lessonprep/start     --> ...
...
```

### 4.7 添加 FULLTEXT 全文索引（首次建表后执行一次）

```sql
USE goedu;

-- 知识库中文全文索引（需 MySQL 5.7+ 的 ngram 分词器）
ALTER TABLE knowledge_chunks
  ADD FULLTEXT INDEX ft_knowledge (title, content, keywords) WITH PARSER ngram;
```

> 如果 MySQL 版本不支持 ngram，去掉 `WITH PARSER ngram` 也可以；LIKE 降级搜索会作为兜底。

### 4.8 导入知识库数据（首次或数据更新后）

```bash
cd /home/hyz/AI/AIEducation/backend
export PATH=/usr/local/go/bin:$PATH

# 注意：scripts 目录下两个文件各有独立 main，需要分开运行
go run scripts/import_knowledge.go
go run scripts/import_graph.go
```

---

## 五、日常启动命令

```bash
# 进入目录并设置 PATH
cd /home/hyz/AI/AIEducation/backend
export PATH=/usr/local/go/bin:$PATH

# 方式一：直接运行源码（开发时推荐）
go run main.go

# 方式二：编译后运行（性能更好）
go build -o goedu . && ./goedu
```

---

## 六、API 接口速查

### 6.1 对话助手接口

```bash
# SSE 流式对话
curl -N -X POST http://localhost:9090/api/dialog/chat \
  -H "Content-Type: application/json" \
  -d '{"message":"商鞅变法的核心内容是什么？","session_id":"","mode":"auto"}'

# 获取会话历史
curl "http://localhost:9090/api/dialog/history?session_id=<uuid>"

# 获取会话列表
curl "http://localhost:9090/api/dialog/sessions"

# 知识库搜索
curl -X POST http://localhost:9090/api/dialog/search \
  -H "Content-Type: application/json" \
  -d '{"query":"孔子","category":"person","limit":5}'
```

### 6.2 备课工作流接口

**完整流程（按顺序调用）：**

```bash
# Step 1：启动工作流，生成大纲（SSE 流式）
curl -N -X POST http://localhost:9090/api/lessonprep/start \
  -H "Content-Type: application/json" \
  -d '{"topic":"商鞅变法","subject":"历史","grade":"初中","duration":45}'
# 响应：流式输出 + 最终 outline JSON + workflow_id（保存此 ID）

# Step 2（可选，可多轮）：对话式修改大纲（SSE 流式）
curl -N -X POST http://localhost:9090/api/lessonprep/outline/edit \
  -H "Content-Type: application/json" \
  -d '{"workflow_id":"<上一步返回的ID>","message":"把导入环节改为情境导入，时间增加到8分钟"}'

# Step 3：确认大纲
curl -X POST http://localhost:9090/api/lessonprep/outline/confirm \
  -H "Content-Type: application/json" \
  -d '{"workflow_id":"<ID>"}'

# Step 4：生成教案（SSE 流式）
curl -N -X POST http://localhost:9090/api/lessonprep/plan/generate \
  -H "Content-Type: application/json" \
  -d '{"workflow_id":"<ID>"}'

# Step 5：生成 PPT（SSE 流式）
curl -N -X POST http://localhost:9090/api/lessonprep/ppt/generate \
  -H "Content-Type: application/json" \
  -d '{"workflow_id":"<ID>"}'

# Step 6：下载 PPT 文件
curl -O -J "http://localhost:9090/api/lessonprep/ppt/download?workflow_id=<ID>"

# 随时查询状态
curl "http://localhost:9090/api/lessonprep/status?workflow_id=<ID>"

# 获取大纲详情
curl "http://localhost:9090/api/lessonprep/outline?workflow_id=<ID>"

# 获取教案详情
curl "http://localhost:9090/api/lessonprep/plan?workflow_id=<ID>"

# 列出所有工作流
curl "http://localhost:9090/api/lessonprep/list"
```

### 6.3 SSE 事件格式

所有 SSE 接口的响应格式统一：

```
data: {"type":"status","content":"..."}           # 进度提示
data: {"type":"text","content":"..."}             # LLM 流式文本（逐 token）
data: {"type":"outline","data":{...}}             # 大纲 JSON 结构
data: {"type":"plan","content":"# 教案..."}       # 教案 Markdown
data: {"type":"ppt","data":{"path":"...","download_url":"..."}} # PPT 路径
data: {"type":"error","content":"..."}            # 错误信息
data: {"type":"done","workflow_id":"...","content":"..."} # 流结束
```

---

## 七、数据库表结构速查

| 表名 | 用途 | 关键字段 |
|------|------|----------|
| `sessions` | 对话会话 | id, user_id, title, mode |
| `messages` | 对话消息 | session_id, role, content |
| `knowledge_chunks` | RAG 知识块 | title, content, category, period, keywords |
| `k_g_nodes` | 知识图谱节点 | name, type, description |
| `k_g_edges` | 知识图谱边 | from_node, to_node, relation |
| `lesson_workflows` | 备课工作流 | id, status, topic, outline_json, lesson_plan, ppt_path |
| `workflow_messages` | 工作流对话记录 | workflow_id, stage, role, content |
| `user` | 用户表 | username, password, email |

**工作流状态流转：**
```
init
 └─► outline_generating ─► outline_ready ◄─► outline_editing
                                │
                                ▼
                          plan_generating ─► plan_ready
                                                │
                                                ▼
                                         ppt_generating ─► done
                                   （任意步骤失败）─► failed
```

---

## 八、后续开发指引

### 8.1 新增备课工作流功能

- 新增服务：在 `services/lessonprep/` 新建 `.go` 文件
- 新增接口：在 `controllers/lessonprep.go` 添加 handler 方法
- 注册路由：在 `router/router.go` 的 `lessonPrep` 组添加路由
- 新增模型字段：在 `models/workflow.go` 修改结构体，重启后 AutoMigrate 自动加列

### 8.2 新增知识库内容

```bash
# 1. 编辑 knowledge/ 下的 JSON 文件（格式与现有保持一致）
# 2. 重新导入
cd /home/hyz/AI/AIEducation/backend
go run scripts/import_knowledge.go
```

### 8.3 已知 TODO 事项

| 功能 | 文件位置 | 说明 |
|------|----------|------|
| 工具调用二轮对话 | `controllers/dialog.go:161` | tool_call 结果需喂回 LLM 做第二轮生成 |
| 时间线查询实现 | `services/mcp/tools.go:122` | 当前返回占位数据，需查 knowledge_chunks 按年份筛选 |
| 上下文自动摘要 | `services/memory/session.go:114` | SummarizeHistory 已实现，未在 dialog.go 触发 |
| JWT 鉴权集成 | `controllers/lessonprep.go` | userID 当前写死为 0 |
| 教案在线编辑 | `controllers/lessonprep.go` | 当前教案只能查看，可参考大纲修改流程增加对话式改教案 |

### 8.4 编译验证命令

```bash
export PATH=/usr/local/go/bin:$PATH
cd /home/hyz/AI/AIEducation/backend

# 编译主包（排除 scripts 目录的双 main 冲突）
go build -o /dev/null .

# 静态检查
go vet ./models/... ./services/lessonprep/... ./controllers/... ./dao/... ./router/...
```

---

## 九、常见问题

| 问题 | 原因 | 解决 |
|------|------|------|
| `go: command not found` | PATH 未设置 | `export PATH=/usr/local/go/bin:$PATH` |
| `dial tcp ... i/o timeout` | 无法访问 golang.org | `go env -w GOPROXY=https://goproxy.cn,direct` |
| `Error 1049: Unknown database` | 数据库未创建 | 执行 4.4 节的 CREATE DATABASE |
| `import cycle not allowed` | models/user.go 旧版引用 dao | 已修复，确认用最新代码 |
| `PPT 文件目录不存在` | runtime/ppt 未创建 | `mkdir -p runtime/ppt`（启动时自动创建） |
| `failed to decode JSON outline` | LLM 返回非 JSON | LLM 偶发问题，重试即可；提示词已加强约束 |
