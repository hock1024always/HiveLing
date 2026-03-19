# 慧备灵师 — AI 教育平台

面向教师的 AI 辅助教学系统，获中国大学生服务创新创业外包大赛**国家一等奖**（A03 AI备课应用赛道）。

## 版本历史

### v1.3 — NLP 查询优化与指代消解

本次迭代聚焦于多轮对话的**查询理解质量**，引入 jieba 中文分词、分层提示词管理，并完整实现了指代消解系统，从根本上解决多轮对话中代词和省略主语导致的检索偏差问题。

#### 新功能

**1. jieba 中文分词与 NLP 服务**

新增 `services/nlp/jieba_service.go`，基于 gojieba 实现专用于历史问答场景的中文语言处理能力：

- **精确分词 + 搜索引擎分词**：两种模式满足不同场景，搜索引擎模式对复合词进一步切分（如"春秋时期" → "春秋/时期/春秋时期"）
- **TF-IDF 关键词提取**：自动提取权重最高的关键词，用于查询扩展
- **历史实体识别（NER）**：基于内置词典识别历史人物（齐桓公、孔子…）、诸侯国（秦国、楚国…）、历史事件（商鞅变法、长平之战…）、年份（公元前770年…）
- **同义词 + 相关概念映射**：如"齐桓公"→同义词["桓公","小白"]，相关概念["管仲","春秋五霸","葵丘之盟"]

**2. 指代消解系统**

新增 `services/nlp/coreference.go` + `coreference_llm.go`，解决多轮对话中代词、省略主语导致的语义缺失问题：

```
第1轮：用户 → "介绍一下齐桓公"
第2轮：用户 → "他是怎么称霸的？"
                ↓ 指代消解
       "齐桓公是怎么称霸的？"  ✅ 检索准确
```

采用**规则优先 + LLM 兜底**双层架构：

- **规则消解器**：基于实体栈（EntityStack）跟踪对话中的人物/国家/事件/时间实体，内置代词词典（"他"→person、"该国"→state、"当时"→time 等），支持省略主语自动补全
- **LLM 消解器**：置信度低于阈值（<0.85）或检测到复杂指代（"他们两人"、"前者后者"）时，携带最近5轮对话调用 LLM 精确消解
- **置信度评估**：每次代词替换 ×0.9，省略补全 ×0.85，低于阈值自动升级到 LLM

**3. Query 优化器重构**

重构 `services/rag/query_optimizer.go`，指代消解作为流水线第一步，保证后续所有分析都基于完整语义：

```
用户原始 Query
    ↓ Step 1: 指代消解（还原代词/省略）
    ↓ Step 2: 实体提取（jieba NER）
    ↓ Step 3: 意图识别（9种意图）
    ↓ Step 4: 时间范围解析
    ↓ Step 5: 查询扩展（同义词+相关概念）
    ↓ Step 6: 分类推断
完整的 QueryAnalysis → RAG 检索
```

新增 `OptimizeQueryWithContext(query, dialogHistory, turn)` 接口，支持多轮对话上下文传入，每轮结束后自动更新实体栈供下一轮使用。

**4. 分层提示词管理系统**

新增 `prompts/` 目录，实现基于 YAML 的提示词统一管理：

- **YAML 模板**：支持变量声明（类型/必填/默认值）、import 机制（`{{import "system/historian.txt"}}`）、输出 Schema 定义、few-shot 示例
- **模板渲染**：基于 `text/template` 引擎，变量注入前自动验证必填项
- **子目录加载**：可按 `tasks/outline_gen`、`system/historian` 路径引用

已创建三个核心任务模板：

| 模板 | 路径 | 用途 |
|------|------|------|
| outline_gen | `prompts/tasks/outline_gen.yaml` | 课程大纲生成 |
| lesson_plan | `prompts/tasks/lesson_plan.yaml` | 详细教案生成 |
| material_gen | `prompts/tasks/material_gen.yaml` | 教学材料（PPT/习题/史料/时间轴）|

**5. 层级记忆压缩**

新增 `services/memory/advanced_compression.go`，实现树形层级记忆结构（HierarchicalMemory），在原有滑动窗口基础上增加：
- 重要性评分驱动的消息保留策略
- 长期摘要节点（对多轮历史做结构化压缩）
- 上下文构建时按 token 预算动态组装（近期重要消息 + 相关摘要）

#### 新增/修改文件

| 类型 | 文件 | 说明 |
|------|------|------|
| NLP 服务 | `backend/services/nlp/jieba_service.go` | jieba 分词 + 实体识别 |
| NLP 服务 | `backend/services/nlp/coreference.go` | 规则指代消解 + 实体栈 |
| NLP 服务 | `backend/services/nlp/coreference_llm.go` | LLM 增强指代消解 |
| RAG 服务 | `backend/services/rag/query_optimizer.go` | 重构，集成指代消解 |
| 记忆服务 | `backend/services/memory/advanced_compression.go` | 层级记忆压缩 |
| Prompt 模板 | `backend/prompts/manager.go` | 模板管理器 |
| Prompt 模板 | `backend/prompts/tasks/outline_gen.yaml` | 大纲生成模板 |
| Prompt 模板 | `backend/prompts/tasks/lesson_plan.yaml` | 教案生成模板 |
| Prompt 模板 | `backend/prompts/tasks/material_gen.yaml` | 教学材料模板 |
| Prompt 模板 | `backend/prompts/system/historian.txt` | 历史学家系统角色 |
| 技术文档 | `AIdocs/指代消解技术方案.md` | 完整方案文档 |
| 技术文档 | `AIdocs/技术方案详解_10个核心问题.md` | 10个核心技术问题解答 |

#### 技术栈更新

| 层 | v1.2 | v1.3 新增 |
|----|------|-----------|
| 中文 NLP | — | gojieba（分词 + TF-IDF + NER） |
| 指代消解 | — | 规则实体栈 + LLM 双层消解 |
| 提示词管理 | 硬编码字符串 | YAML 模板 + import 机制 |
| 记忆压缩 | 滑动窗口 | 树形层级压缩 + 重要性评分 |

---

### v1.2 — 前端完善与一键部署

基于 v1.1 强化的对话助手，本次迭代完善了**前端用户界面**，并提供了完整的**一键部署方案**。

#### 新功能

**1. 完善前端页面**

新增 3 个核心功能页面，覆盖平台全部模块：

- **知识库管理** (`frontend/knowledge.html`)：知识条目增删改查、批量导入（JSON/CSV/Markdown）、分类筛选、关键词搜索、统计面板
- **系统监控** (`frontend/dashboard.html`)：实时系统状态、Agent 性能统计、对话趋势图表、实时日志流
- **首页导航** (`frontend/index.html`)：六大功能模块入口卡片

**2. 统一前端配置管理**

新增 `frontend/config.js`，集中管理 API_BASE 配置，所有页面改为引用统一配置。

**3. 一键部署脚本**

新增 `deploy.sh`，支持 Docker 和本地两种部署方式，提供 `stop`/`restart`/`status`/`logs`/`update` 等管理命令。

**4. Docker 部署支持**

新增 `docker-compose.yml`，一键启动 MySQL + Redis + Milvus + 后端服务。

---

### v1.1 — 向量检索与记忆增强

#### 新功能

**1. 向量数据库支持（Milvus）**

引入 Milvus 向量数据库，实现语义检索，解决原有关键词检索无法理解同义词和用户意图的问题：

- `services/rag/embedding.go`：调用 `text-embedding-3-small` 接口转换向量，支持批量处理与重试
- `services/rag/milvus.go`：封装 Milvus Go SDK，ANN 搜索（AUTOINDEX + L2）
- `services/rag/retriever.go`：混合检索（向量权重 0.6 + 关键词权重 0.4）

**2. Redis 缓存层**

基于 `go-redis/v8` 封装缓存服务：Embedding 缓存（TTL 24h）+ 检索结果缓存（TTL 10min），响应时间从 500ms 降至 50ms。

**3. 时间线查询工具**

补全 `get_timeline` 工具实现，支持按年份范围、诸侯国、事件类型组合查询。

**4. 上下文记忆增强**

引入消息语义标注（Tag 11种 + 重要性评分 1–10 + 实体提取），智能压缩策略（保留最近5条 + 高重要性消息 + 结构化摘要）。

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
| 向量数据库 | Milvus 2.3（可选）|
| 缓存 | Redis 7（可选）|
| 中文 NLP | gojieba（分词 + TF-IDF + NER）|
| LLM | DeepSeek API（OpenAI 兼容）|
| 联网搜索 | Serper API |
| 数字人 | HeyGen / SadTalker / 自定义 ML 模型 |
| 前端 | 静态 HTML（`/frontend`）|
| 配置 | `.env` 环境变量 |

## 项目结构

```
AIEducation/
├── backend/
│   ├── main.go                        # 入口，端口 9090
│   ├── config/config.go               # 环境变量配置
│   ├── router/router.go               # 路由注册
│   ├── prompts/                       # 提示词模板（v1.3 新增）
│   │   ├── manager.go                 # 模板管理器（YAML + import）
│   │   ├── system/historian.txt       # 历史学家系统角色
│   │   └── tasks/
│   │       ├── outline_gen.yaml       # 大纲生成模板
│   │       ├── lesson_plan.yaml       # 教案生成模板
│   │       └── material_gen.yaml      # 教学材料模板
│   ├── controllers/
│   │   ├── dialog.go                  # 教学对话控制器（SSE）
│   │   ├── lessonprep.go              # 备课工作流控制器（SSE）
│   │   └── digital_human.go          # 数字人控制器
│   ├── services/
│   │   ├── nlp/                       # NLP 服务（v1.3 新增）
│   │   │   ├── jieba_service.go       # 分词 + TF-IDF + NER
│   │   │   ├── coreference.go         # 规则指代消解 + 实体栈
│   │   │   └── coreference_llm.go     # LLM 增强指代消解
│   │   ├── llm/client.go              # DeepSeek 客户端（流式/非流式）
│   │   ├── rag/
│   │   │   ├── retriever.go           # 混合检索（向量 + 关键词）
│   │   │   ├── query_optimizer.go     # 查询优化器（含指代消解）
│   │   │   ├── embedding.go           # Embedding 服务
│   │   │   ├── milvus.go              # Milvus 客户端
│   │   │   ├── document_parser.go     # 文档解析
│   │   │   └── importer.go            # 批量导入
│   │   ├── mcp/tools.go               # MCP 工具执行器（4 个工具）
│   │   ├── memory/
│   │   │   ├── session.go             # 会话管理（窗口 + 压缩）
│   │   │   └── advanced_compression.go # 层级记忆压缩（v1.3 新增）
│   │   ├── graph/query.go             # 知识图谱查询
│   │   ├── cache/cache.go             # Redis 缓存服务
│   │   ├── timeline/timeline.go       # 时间线查询
│   │   └── lessonprep/
│   │       ├── workflow.go            # 工作流状态机
│   │       ├── outline.go             # 大纲生成
│   │       ├── outline_edit.go        # 大纲对话式修改
│   │       ├── lessonplan.go          # 教案生成
│   │       └── pptgen.go              # PPT 生成（原生 OOXML）
│   ├── models/                        # GORM 数据模型
│   └── scripts/                       # 知识库/知识图谱导入脚本
├── frontend/                          # 静态前端页面
├── AIdocs/                            # 技术文档
│   ├── 指代消解技术方案.md             # v1.3 新增
│   ├── 技术方案详解_10个核心问题.md    # v1.3 新增
│   ├── 向量数据库实现详解.md
│   └── 上下文记忆系统设计.md
├── deploy.sh                          # 一键部署脚本
└── docker-compose.yml                 # Docker 编排
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
| `SERVER_HOST` | `0.0.0.0` | 服务监听地址 |
| `MILVUS_ENABLED` | `false` | 是否启用向量检索（可选）|
| `MILVUS_HOST` | `localhost` | Milvus 地址（可选）|
| `MILVUS_PORT` | `19530` | Milvus 端口（可选）|
| `SERPER_API_KEY` | — | 联网搜索（可选）|
| `HEYGEN_API_KEY` | — | HeyGen 数字人（可选）|

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
GET    /api/digital-human/avatars          获取可用形象列表
```

## 访问地址

部署完成后可通过以下地址访问：

- **首页**: `http://服务器IP:9090/app/index.html`
- **教学对话**: `http://服务器IP:9090/app/dialog.html`
- **备课工作流**: `http://服务器IP:9090/app/lessonprep.html`
- **知识库管理**: `http://服务器IP:9090/app/knowledge.html`
- **系统监控**: `http://服务器IP:9090/app/dashboard.html`
- **数字人视频**: `http://服务器IP:9090/app/digitalhuman.html`

## 文档

详细的技术方案与实现细节见 `AIdocs/` 目录：

- [`AIdocs/指代消解技术方案.md`](./AIdocs/指代消解技术方案.md)：指代消解方案、实体栈、规则+LLM双层架构（v1.3）
- [`AIdocs/技术方案详解_10个核心问题.md`](./AIdocs/技术方案详解_10个核心问题.md)：RAG、Agent、记忆、提示词等10个核心技术问题（v1.3）
- [`AIdocs/向量数据库实现详解.md`](./AIdocs/向量数据库实现详解.md)：向量原理、混合检索、缓存策略（v1.1）
- [`AIdocs/上下文记忆系统设计.md`](./AIdocs/上下文记忆系统设计.md)：标签体系、重要性评分、智能压缩（v1.1）
