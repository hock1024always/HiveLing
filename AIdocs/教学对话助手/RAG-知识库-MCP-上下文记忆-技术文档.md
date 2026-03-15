# 教学对话助手技术文档 - RAG / 知识库 / MCP / 上下文记忆

## 一、系统总体架构

```
用户请求 (POST /api/dialog/chat)
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│                  DialogController                        │
│  1. 解析请求 → 2. 获取/创建会话 → 3. 构建上下文           │
│  4. 拼接 System Prompt → 5. 注册 Tools → 6. 调用 LLM    │
│  7. 处理流式响应 → 8. 执行工具调用 → 9. 保存消息           │
└────────────┬────────────────────────────────────────────┘
             │ SSE 流式传输
             ▼
┌──────────────────┐  ┌──────────────┐  ┌────────────────┐
│  SessionManager  │  │  ToolExecutor │  │   LLM Client   │
│  (上下文记忆)     │  │  (MCP 工具)   │  │  (DeepSeek SSE) │
└────────┬─────────┘  └──────┬───────┘  └───────┬────────┘
         │                   │                   │
    ┌────▼────┐     ┌───────▼────────┐    ┌────▼────────┐
    │  MySQL  │     │  Retriever     │    │  DeepSeek   │
    │ sessions│     │  GraphQuery    │    │  API        │
    │ messages│     │  Serper API    │    │  (SSE流式)   │
    └─────────┘     └───────┬────────┘    └─────────────┘
                            │
                    ┌───────▼────────┐
                    │     MySQL      │
                    │ knowledge_chunks│
                    │ kg_nodes       │
                    │ kg_edges       │
                    └────────────────┘
```

### 一次完整请求的数据流

以用户提问 **"商鞅变法的内容和影响是什么？"** 为例：

```
1. POST 请求到达 → 解析出 message="商鞅变法的内容和影响是什么？", mode="auto"

2. 会话管理：
   - session_id 为空 → 创建新会话，生成 UUID，写入 sessions 表
   - 将用户消息写入 messages 表 (role=user)

3. 构建上下文：
   - 从 messages 表读取该 session 最近 20 条消息
   - 转换为 []llm.Message 格式

4. 拼接 Prompt：
   messages = [
     {role: "system", content: "你是春秋战国历史教学助手...当前模式：智能模式"},
     {role: "user",   content: "商鞅变法的内容和影响是什么？"}
   ]

5. 注册 Tools（mode=auto 时注册 4 个工具）

6. 调用 DeepSeek API（stream=true）
   → DeepSeek 判断需要调用 search_knowledge 工具
   → 返回 tool_call: {name: "search_knowledge", arguments: {query: "商鞅变法", category: "event"}}

7. 执行工具：
   → ToolExecutor.ExecuteTool("search_knowledge", args)
   → Retriever.Search("商鞅变法", "event", 5)
   → MySQL FULLTEXT 搜索 knowledge_chunks 表
   → 返回匹配的知识块（商鞅变法的详细内容）

8. 工具结果通过 SSE 发送给前端（展示"正在搜索知识库..."）

9. 将工具结果拼入上下文，再次调用 DeepSeek 生成最终回答

10. 流式输出：DeepSeek 逐 token 返回 → 逐 chunk SSE 推送给前端

11. 完成后保存 assistant 消息到 messages 表
```

---

## 二、RAG 检索增强生成 - 具体实现

### 2.1 RAG 是什么（一句话）

用户提问 → **先从本地知识库检索相关内容** → 把检索结果作为上下文塞进 Prompt → 再让大模型生成回答。这样大模型的回答就基于我们的教材数据，而不是完全靠自己"编"。

### 2.2 知识库数据结构

知识库以 JSON 文件形式存储原始数据，导入到 MySQL 的 `knowledge_chunks` 表：

```sql
CREATE TABLE knowledge_chunks (
    id          INT AUTO_INCREMENT PRIMARY KEY,
    title       VARCHAR(255),       -- "商鞅变法"
    content     TEXT,               -- 300-800字的详细描述
    category    VARCHAR(50),        -- person/event/battle/school/state/culture
    keywords    VARCHAR(500),       -- "秦孝公,废井田,军功爵,连坐法,立木为信"
    period      VARCHAR(100),       -- "战国"
    year_start  INT,                -- -356 (公元前356年)
    year_end    INT,                -- -338
    source      VARCHAR(255),       -- 来源文献
    created_at  DATETIME,
    updated_at  DATETIME,
    deleted_at  DATETIME,
    FULLTEXT INDEX ft_knowledge (title, content, keywords) WITH PARSER ngram
);
```

目前知识库规模：12 诸侯国 + 18 人物 + 12 事件 + 10 思想流派 = 52 条知识块。

### 2.3 检索流程（两级降级策略）

代码位置：`services/rag/retriever.go`

```go
func (r *Retriever) Search(query string, category string, limit int) ([]SearchResult, error) {
    // ===== 第一级：MySQL FULLTEXT 全文搜索 =====
    // 使用 MySQL 内置的全文索引，ngram 分词器支持中文
    // NATURAL LANGUAGE MODE 会自动按 TF-IDF 算法计算相关性
    db.Where("MATCH(title, content, keywords) AGAINST(? IN NATURAL LANGUAGE MODE)", query)

    // ===== 第二级：如果全文搜索无结果，降级为 LIKE 模糊搜索 =====
    // 1. 提取关键词（去停用词）
    // 2. 对每个关键词在 title/content/keywords 三个字段做 LIKE 匹配
    // 3. 条件之间用 OR 连接，保证召回率

    // ===== 最终：BM25 简化版打分排序 =====
    // 对所有命中的知识块计算分数：
    //   标题命中 → +3.0 分
    //   keywords 命中 → +2.0 分
    //   内容命中 → +0.5 分/次
    // 按分数降序排列，取 Top-K
}
```

**为什么这样设计**：
- FULLTEXT + ngram 是 MySQL 原生能力，不需要额外部署向量数据库
- 对于历史知识这种**实体名称明确**的领域，关键词匹配的效果非常好（"商鞅变法"这种词是精确匹配）
- 两级降级保证：全文索引未建立或数据量小时，LIKE 搜索也能兜底
- BM25 打分给不同字段赋予不同权重，标题命中比内容命中更重要

### 2.4 检索结果如何注入 Prompt

不是直接拼到 user message 里，而是通过 **MCP 工具调用** 的方式：

```
1. DeepSeek 看到用户问题 + search_knowledge 工具定义
2. DeepSeek 自己决定要不要调用 search_knowledge
3. 调用后，检索结果作为 tool_result 返回
4. DeepSeek 基于 tool_result 中的知识库内容生成回答
```

这比直接拼 Prompt 的优势：
- **LLM 自主决策**：不是每个问题都需要检索（比如闲聊就不需要）
- **可观测**：前端可以看到"正在搜索知识库..."的过程
- **可扩展**：同一套 Tool 框架可以接更多数据源

### 2.5 后续升级路径：向量检索

当前是 FULLTEXT 关键词检索。后续要加向量检索需要：

```
1. 接入 Embedding 模型（如 Jina Embeddings API，免费支持中文）
2. 将每个知识块生成 embedding 向量，存入新增的 vector 列（JSON 格式存 MySQL，或用 Qdrant/Milvus）
3. 用户提问时也生成 embedding
4. 计算余弦相似度，与 FULLTEXT 分数加权混合排序

混合检索的好处：
- 关键词检索处理精确匹配（"商鞅"）
- 向量检索处理语义匹配（"秦国为什么变强了" → 匹配到商鞅变法）
```

---

## 三、知识库构建 - 具体实现

### 3.1 数据来源与整理

知识库内容基于公开史料整理，覆盖春秋战国核心知识：

| 分类 | 文件 | 条数 | 典型内容 |
|------|------|------|----------|
| 诸侯国 | states.json | 12 | 齐楚秦晋魏赵韩燕越吴鲁宋 |
| 历史人物 | people.json | 18 | 孔孟老庄、商鞅韩非、孙武白起等 |
| 重大事件 | events.json | 12 | 三家分晋、商鞅变法、围魏救赵、长平之战等 |
| 思想流派 | schools.json | 10 | 儒道法墨兵纵横名阴阳杂农 |

每条知识块的字段设计：

```json
{
  "title": "商鞅变法",                    // 用于标题匹配（权重最高）
  "content": "商鞅变法是战国时期...",       // 300-800 字的结构化描述
  "category": "event",                   // 分类，用于检索时缩小范围
  "period": "战国",                      // 时期
  "year_start": -356,                    // 年份范围（负数=公元前）
  "year_end": -338,
  "keywords": "秦孝公,废井田,军功爵,连坐法"  // 关键词标签（权重次高）
}
```

### 3.2 数据导入流程

代码位置：`scripts/import_knowledge.go`

```
1. 读取 knowledge/*.json 文件
2. 解析为 []KnowledgeData 结构
3. 对每条数据：
   - 按 title + category 做唯一性检查
   - 不存在 → INSERT
   - 已存在 → UPDATE（支持重复导入不产生重复数据）
4. 写入 knowledge_chunks 表
```

### 3.3 知识图谱

代码位置：`services/graph/query.go`

知识图谱用 MySQL 两张关系表实现（不依赖 Neo4j）：

```sql
-- 节点表（48 个实体）
kg_nodes (id, name, type, description, properties, year_start, year_end)

-- 边表（50+ 条关系）
kg_edges (id, from_node_id, to_node_id, relation_type, description, properties)
```

**节点类型**：person / state / event / battle / school / concept

**关系类型**及含义：

| 关系 | 含义 | 示例 |
|------|------|------|
| FOUNDED | 创立 | 孔子 → 儒家 |
| BELONGS_TO | 属于 | 屈原 → 楚国 |
| SERVED | 仕于 | 商鞅 → 秦国 |
| PARTICIPATED | 参与 | 白起 → 长平之战 |
| CAUSED | 导致 | 三家分晋 → 晋国灭亡 |
| INFLUENCED | 影响 | 商鞅变法 → 秦国强大 |
| OPPOSED | 对立 | 苏秦 ↔ 张仪 |
| TAUGHT | 教导 | 荀子 → 韩非子 |

**查询实现**：

```go
// 查询 "孔子" 的所有关系
func (s *QueryService) QueryByName(name string) (*QueryResult, error) {
    // 1. 按 name 查 kg_nodes，拿到 node.ID
    // 2. 查出边：SELECT * FROM kg_edges WHERE from_node_id = node.ID
    //    对每条边，再查目标节点名称
    // 3. 查入边：SELECT * FROM kg_edges WHERE to_node_id = node.ID
    //    对每条边，再查来源节点名称
    // 4. 组装成 QueryResult{Entity, Type, Description, Relations[]}
}
```

返回示例：
```json
{
  "entity": "孔子",
  "type": "person",
  "description": "儒家学派创始人，至圣先师",
  "relations": [
    {"target": "儒家", "type": "FOUNDED", "direction": "out", "description": "创立儒家学派"},
    {"target": "鲁国", "type": "BELONGS_TO", "direction": "out", "description": "鲁国人"},
    {"target": "孟子", "type": "INFLUENCED", "direction": "in", "description": "继承发展孔子思想"}
  ]
}
```

---

## 四、MCP 工具调用 - 具体实现

### 4.1 MCP 在本项目中的含义

MCP（Model Context Protocol）是让大模型能够调用外部系统的协议。在本项目中，我们利用 DeepSeek 的 **Function Calling** 能力实现类 MCP 的工具调用机制。

核心思路：
```
1. 向 DeepSeek 注册一组工具（JSON Schema 描述每个工具的名称、参数、用途）
2. DeepSeek 根据用户问题，自主决定是否调用工具、调用哪个工具、传什么参数
3. 后端收到 tool_call → 执行对应逻辑 → 将结果返回给 DeepSeek
4. DeepSeek 基于工具结果生成最终回答
```

### 4.2 工具注册

代码位置：`services/llm/client.go` 的 `BuildTools()` 函数

```go
func BuildTools(mode string) []Tool {
    tools := []Tool{
        // 始终注册：本地知识库搜索
        {
            Type: "function",
            Function: ToolFunction{
                Name:        "search_knowledge",
                Description: "搜索春秋战国历史知识库...",
                Parameters:  { query: string, category: enum, limit: int }
            },
        },
        // 始终注册：知识图谱查询
        {Name: "query_graph", ...},
        // 始终注册：时间线查询
        {Name: "get_timeline", ...},
    }

    // 只有 online/auto 模式才注册联网搜索工具
    if mode == "online" || mode == "auto" {
        tools = append(tools, Tool{Name: "web_search", ...})
    }

    return tools
}
```

**mode 对工具注册的影响**：

| 模式 | search_knowledge | query_graph | get_timeline | web_search |
|------|:---:|:---:|:---:|:---:|
| local | Y | Y | Y | **N** |
| online | Y | Y | Y | Y |
| auto | Y | Y | Y | Y |

这就是你提到的"两个模式"的实现方式——不是靠 if/else 切换逻辑，而是**通过控制工具注册列表来约束 LLM 的行为**。local 模式下 LLM 根本看不到 `web_search` 这个工具，自然不会尝试联网。

### 4.3 四个工具的具体实现

代码位置：`services/mcp/tools.go`

#### 工具 1：search_knowledge（本地知识库搜索）

```go
func (e *ToolExecutor) executeSearchKnowledge(arguments json.RawMessage) (string, error) {
    // 解析参数：query, category, limit
    // 调用 rag.Retriever.Search()
    // 返回格式化的 JSON 结果
}
```

调用链：`ToolExecutor → Retriever.Search() → MySQL FULLTEXT → BM25 打分 → Top-K`

#### 工具 2：query_graph（知识图谱查询）

```go
func (e *ToolExecutor) executeQueryGraph(arguments json.RawMessage) (string, error) {
    // 解析参数：entity, relation_type
    // 先精确查询 graphQuery.QueryByName(entity)
    // 如果找不到 → 模糊搜索 graphQuery.SearchEntities(entity, 5)
    //   返回搜索建议让 LLM 选择
    // 找到了 → 如果指定 relation_type，过滤关系
    // 返回实体信息 + 所有关系的 JSON
}
```

调用链：`ToolExecutor → GraphQuery.QueryByName() → MySQL kg_nodes + kg_edges → 格式化`

#### 工具 3：get_timeline（时间线查询）

按年份范围和国家筛选事件，用于回答"公元前5世纪秦国发生了什么"这类问题。

#### 工具 4：web_search（联网搜索）

```go
func (e *ToolExecutor) executeWebSearch(arguments json.RawMessage) (string, error) {
    // 解析参数：query, source
    // 检查 Serper API Key 是否配置
    // POST https://google.serper.dev/search
    //   Header: X-API-KEY
    //   Body: {"q": "商鞅变法 最新考古发现"}
    // 解析返回的 organic results
    // 格式化为 [{title, url, snippet}, ...] 返回给 LLM
}
```

### 4.4 工具调用在 SSE 中的呈现

用户在前端看到的过程：

```
"正在分析问题..."          ← type: status
"正在思考..."             ← type: status
"正在调用 search_knowledge..." ← type: status（工具调用中间态）
[工具调用详情]             ← type: tool_call（展示调了什么工具、什么参数）
[工具返回结果]             ← type: tool_result（展示检索到了什么）
"商鞅变法是..."           ← type: text（逐字流式输出最终回答）
"...奠定了基础。"          ← type: text
[完成]                   ← type: done
```

### 4.5 auto 模式下 LLM 的决策逻辑

auto 模式下，所有 4 个工具都注册给 DeepSeek。DeepSeek 通过 System Prompt 中的指引和工具描述来自主决策：

- 用户问"商鞅变法" → DeepSeek 调用 `search_knowledge`（本地知识库能回答）
- 用户问"孔子和老子什么关系" → DeepSeek 调用 `query_graph`（知识图谱更适合）
- 用户问"2024年最新的长平之战考古发现" → DeepSeek 调用 `web_search`（需要联网）
- 用户问"你好" → DeepSeek 不调用任何工具（直接回答）

**这种设计的好处**：不需要我们写规则判断"什么时候该联网"，把这个决策权交给 LLM 自己，通过 Tool Description 中的描述来引导。

---

## 五、上下文记忆 - 具体实现

### 5.1 记忆的存储结构

代码位置：`models/session.go`, `services/memory/session.go`

```
sessions 表（一个用户可以有多个会话）
┌──────────────────────────────────┐
│ id (UUID)  │ user_id │ mode     │
│ title      │ created_at          │
└──────────────────────────────────┘
         │ 1:N
         ▼
messages 表（一个会话有多条消息）
┌──────────────────────────────────────────┐
│ id │ session_id │ role       │ content   │
│    │            │ tool_calls │ tool_result│
│    │            │ created_at             │
└──────────────────────────────────────────┘
```

每条消息记录包含：
- `role`: user / assistant / system
- `content`: 消息文本
- `tool_calls`: JSON 格式存储 LLM 的工具调用（哪个工具、什么参数）
- `tool_result`: 工具执行的返回结果

### 5.2 上下文构建流程

代码位置：`services/memory/session.go` 的 `BuildContext()`

```go
func (m *SessionManager) BuildContext(sessionID string, maxMessages int) ([]llm.Message, error) {
    // 1. 从 messages 表查询该 session 的消息，按时间正序排列
    // 2. 限制最多取最近 maxMessages 条（默认 20 条）
    // 3. 转换为 []llm.Message{Role, Content} 格式
    // 4. 返回给调用方
}
```

最终发给 DeepSeek 的 messages 数组：

```json
[
  {"role": "system",    "content": "你是春秋战国历史教学助手..."},   // 固定的系统提示
  {"role": "user",      "content": "商鞅是谁？"},                // 第1轮用户消息
  {"role": "assistant", "content": "商鞅是战国时期..."},          // 第1轮助手回复
  {"role": "user",      "content": "他的变法内容有哪些？"},        // 第2轮用户消息（当前）
]
```

DeepSeek 看到完整的对话历史，所以能理解"他"指的是商鞅。

### 5.3 滑动窗口策略

**问题**：对话越来越长，messages 数组越来越大，Token 消耗快速增长。

**当前方案**：`BuildContext(sessionID, 20)` 限制最多取最近 20 条消息。

**工作原理**：
```
会话有 50 条消息时：

完整历史：[msg1, msg2, ..., msg48, msg49, msg50]
实际发送：[system_prompt, msg31, msg32, ..., msg49, msg50]
                          ↑ 只取最近 20 条
```

### 5.4 超长对话摘要压缩

代码位置：`services/memory/session.go` 的 `SummarizeHistory()`

当对话超过 10 轮时，可以触发摘要压缩：

```go
func (m *SessionManager) SummarizeHistory(sessionID string) (string, error) {
    // 1. 获取所有消息
    // 2. 如果 <= 10 条，不需要压缩
    // 3. 取前面的消息（保留最近 5 条不动）
    // 4. 将前面的消息拼成文本
    // 5. 调用 DeepSeek（非流式）：
    //    "请将以下历史对话内容压缩成简洁的摘要，保留关键信息：\n\n{历史文本}"
    // 6. 将摘要作为 system 消息存入：
    //    "[对话摘要] 用户之前询问了商鞅变法的背景和内容，助手详细讲解了..."
    // 7. 下次构建上下文时，这条摘要消息就代替了前面的多轮对话
}
```

**压缩前后对比**：
```
压缩前发给 LLM 的消息：
  system + msg1 + msg2 + ... + msg20  （20条消息，约 4000 tokens）

压缩后发给 LLM 的消息：
  system + [摘要] + msg16 + ... + msg20  （6条消息，约 1000 tokens）
```

### 5.5 会话生命周期管理

```go
// 创建会话（首次对话时自动创建）
CreateSession(userID, mode) → UUID

// 后续对话携带 session_id，复用同一会话
// 前端保存 session_id 到 localStorage

// 自动生成标题（取用户首条消息的前 20 个字符）
GenerateTitle(sessionID, "商鞅变法的核心内容...") → "商鞅变法的核心内容..."

// 列出历史会话
ListSessions(userID, 20) → [{id, title, mode, updated_at}, ...]

// 删除会话（级联删除所有消息）
DeleteSession(sessionID)
```

---

## 六、SSE 流式传输 - 具体实现

### 6.1 为什么用 SSE 而不是 WebSocket

| | SSE | WebSocket |
|------|------|------|
| 方向 | 服务器 → 客户端（单向） | 双向 |
| 协议 | 普通 HTTP | 独立协议 ws:// |
| 前端 | 原生 EventSource / fetch | 需要 WebSocket API |
| 适用场景 | LLM 流式输出（天然单向） | 实时聊天室（需要双向） |

LLM 的流式输出本质上就是服务器单向推送，SSE 最合适。

### 6.2 后端实现

代码位置：`controllers/dialog.go`

```go
func (d *DialogController) Chat(c *gin.Context) {
    // 设置 SSE 响应头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    // ...省略前置逻辑...

    // 流式调用 DeepSeek
    d.llmClient.StreamChat(messages, tools, func(chunk *StreamChunk) error {
        if chunk.Choices[0].Delta.Content != "" {
            // 每收到一个 token，立即通过 SSE 推送给前端
            d.sendSSE(c, SSEEvent{Type: "text", Content: chunk.Choices[0].Delta.Content})
        }
        return nil
    })
}

func (d *DialogController) sendSSE(c *gin.Context, event SSEEvent) {
    data, _ := json.Marshal(event)
    fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))   // SSE 协议格式
    c.Writer.(http.Flusher).Flush()                        // 立即刷出，不等缓冲区满
}
```

### 6.3 DeepSeek SSE 的解析

代码位置：`services/llm/client.go` 的 `doStreamRequest()`

```go
func (c *Client) doStreamRequest(chatReq ChatRequest, onChunk func(*StreamChunk) error) {
    // 发送 HTTP POST 请求，Header: Accept: text/event-stream
    // DeepSeek 返回的格式：
    //   data: {"choices":[{"delta":{"content":"商"}}]}
    //   data: {"choices":[{"delta":{"content":"鞅"}}]}
    //   data: {"choices":[{"delta":{"content":"变"}}]}
    //   data: [DONE]

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data: ") { continue }

        data := strings.TrimPrefix(line, "data: ")
        if data == "[DONE]" { break }

        var chunk StreamChunk
        json.Unmarshal([]byte(data), &chunk)

        // 回调：每解析出一个 chunk，立即调用 onChunk
        // onChunk 内部会通过 sendSSE 推送给前端
        onChunk(&chunk)
    }
}
```

**整个流式链路**：

```
DeepSeek API 输出 token
    → HTTP SSE 推送到我们的后端
    → bufio.Scanner 逐行解析
    → onChunk 回调
    → sendSSE 写入 gin.Writer
    → HTTP SSE 推送到前端浏览器
    → 前端 JS 逐字显示
```

延迟：从 DeepSeek 生成到用户看到，只有网络传输延迟，**没有等完整回复再发送的延迟**。

---

## 七、面试常见问题与回答

### Q1: RAG 检索为什么不用向量数据库？

当前阶段使用 MySQL FULLTEXT + ngram 分词器做关键词检索，原因：

1. **领域特点**：历史知识的实体名称很明确（"商鞅变法"、"长平之战"），关键词匹配的精确度足够高
2. **零额外依赖**：不需要部署 Milvus/Qdrant/pgvector，也不需要 Embedding 模型
3. **两级降级**：FULLTEXT 失败时降级为 LIKE，保证可用性
4. **后续升级路径已规划**：接入 Embedding API + 混合排序（关键词分数 + 向量分数加权）

### Q2: MCP 工具调用的循环是怎么做的？

利用 DeepSeek 的 Function Calling 能力：
1. 请求时传入 tools 数组（JSON Schema 格式描述工具）
2. 响应中如果 `finish_reason = "tool_calls"`，说明 LLM 决定调用工具
3. 后端解析 tool_call，执行对应逻辑，拿到结果
4. 将 tool_result 追加到 messages 数组，再次调用 LLM
5. LLM 基于工具结果生成最终回答

```
用户 → LLM → tool_call(search_knowledge) → 后端执行 → tool_result → LLM → 最终回答
```

### Q3: 上下文窗口满了怎么办？

三层策略：
1. **滑动窗口**：只取最近 20 条消息，丢弃更早的
2. **摘要压缩**：超过 10 轮后，调用 LLM 把早期对话压缩成一段摘要
3. **DeepSeek 本身支持 64K 上下文**：20 轮对话通常不会超限

### Q4: 本地模式和联网模式具体怎么切换的？

不是 if/else 硬编码，而是**通过控制工具列表来约束 LLM 行为**：
- `local` 模式：只注册 search_knowledge / query_graph / get_timeline
- `online/auto` 模式：额外注册 web_search

LLM 看不到的工具就无法调用。auto 模式下 LLM 自己判断是否需要联网。

### Q5: 知识图谱为什么不用 Neo4j？

1. **数据规模小**：48 个节点 + 50 条边，MySQL 关系表完全够用
2. **查询复杂度低**：主要是单实体关系查询和两实体关系查询，SQL JOIN 就能解决
3. **减少运维成本**：不需要额外部署和维护 Neo4j 实例

如果后续知识图谱规模扩大到万级节点、需要多跳路径查询，再考虑迁移到 Neo4j。

### Q6: 如何保证回答的准确性？

1. **RAG 检索**：回答基于本地知识库的史料内容，不是 LLM 凭空生成
2. **System Prompt 约束**：明确告诉 LLM "准确性优先，基于史料"
3. **工具结果可追溯**：每次工具调用的参数和结果都通过 SSE 暴露给前端，用户可以看到引用了哪些资料
4. **消息持久化**：所有对话和工具调用都存入 MySQL，支持审计和排查
