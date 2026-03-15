# 教学对话助手 — MCP 实战

## 一、本项目中 MCP 的落地方式

本项目采用**Function Call 形式实现 MCP 工具**，在 `services/llm/client.go` 中注册工具定义，在 `services/mcp/tools.go` 中实现工具执行器，通过 `controllers/dialog.go` 编排调用流程。

---

## 二、工具注册（`services/llm/client.go:251`）

`BuildTools(mode)` 返回 OpenAI Function Calling 格式的工具列表，按 mode 动态组合：

```go
type Tool struct {
    Type     string       `json:"type"`      // "function"
    Function ToolFunction `json:"function"`
}

type ToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}
```

---

## 三、四个工具的实现（`services/mcp/tools.go`）

### `search_knowledge` — 知识库全文检索

```go
// 输入参数
{
    "query":    "搜索关键词或问题",
    "category": "person|event|battle|school|state|culture|all",
    "limit":    5
}
// 执行
results, _ := e.retriever.Search(args.Query, args.Category, args.Limit)
return rag.FormatResults(results)  // JSON 数组
```

输出示例：
```json
[{"id": 1, "title": "商鞅变法", "category": "event", "content": "...", "score": 4.5}]
```

### `query_graph` — 知识图谱查询

```go
// 输入参数
{
    "entity":        "孔子",
    "relation_type": "FOUNDED"  // 可选
}
// 执行（精确匹配优先，未命中返回搜索建议）
result, err := e.graphQuery.QueryByName(args.Entity)
if err != nil {
    nodes, _ := e.graphQuery.SearchEntities(args.Entity, 5)
    // 返回 {"found": false, "suggestions": [...], "message": "未找到精确匹配，您是否想查询..."}
}
// 支持 relation_type 过滤
```

关系类型枚举：`BELONGS_TO`（属于）/ `FOUNDED`（创立）/ `PARTICIPATED`（参与）/ `DEFEATED`（击败）/ `ALLIED`（结盟）

### `get_timeline` — 时间线查询

```go
// 输入参数
{
    "start_year": -770,  // 公元前770年
    "end_year":   -221,
    "state":      "秦"   // 可选
}
// 当前返回占位 JSON，功能开发中
```

### `web_search` — 联网搜索（online/auto 模式）

```go
// 输入参数
{
    "query":  "搜索关键词",
    "source": "general|academic"
}
// 执行：调用 Serper API
req.Header.Set("X-API-KEY", apiKey)
POST {BaseURL}/search  {"q": query}

// 返回 organic 搜索结果的 title/url/snippet
```

配置：`config.AppConfig.Serper.APIKey`，未配置时返回错误，不影响本地工具正常工作。

---

## 四、工具执行器（`services/mcp/tools.go:31`）

```go
func (e *ToolExecutor) ExecuteTool(toolName string, arguments json.RawMessage) (string, error) {
    switch toolName {
    case "search_knowledge": return e.executeSearchKnowledge(arguments)
    case "query_graph":      return e.executeQueryGraph(arguments)
    case "get_timeline":     return e.executeGetTimeline(arguments)
    case "web_search":       return e.executeWebSearch(arguments)
    default:                 return "", fmt.Errorf("unknown tool: %s", toolName)
    }
}
```

---

## 五、工具调用在对话中的触发流程（`controllers/dialog.go:107`）

```go
// StreamChat 的 onChunk 回调中处理工具调用
if len(delta.ToolCalls) > 0 {
    for _, tc := range delta.ToolCalls {
        // 1. 告知前端"正在调用工具"
        sendSSE(c, SSEEvent{Type: "tool_call", Tool: tc.Function.Name, Arguments: tc.Function.Arguments})
        sendSSE(c, SSEEvent{Type: "status", Content: "正在调用 " + tc.Function.Name + "..."})

        // 2. 执行工具
        result, err := d.toolExecutor.ExecuteTool(tc.Function.Name, tc.Function.Arguments)
        if err != nil {
            result = `{"error": "` + err.Error() + `"}`
        }

        // 3. 将结果推送给前端
        sendSSE(c, SSEEvent{Type: "tool_result", Tool: tc.Function.Name, Result: result})
        
        // 4. 记录工具调用（toolCalls slice）
        toolCalls = append(toolCalls, tc)
    }
}
```

---

## 六、MCP 工具 vs SKILL 的区别

| | MCP 工具（本项目）| SKILL（扩展方向）|
|--|-------------------|-----------------|
| 粒度 | 原子操作（查一次数据库/API）| 多步工作流（大纲→教案→PPT）|
| 状态 | 无状态 | 有状态（工作流 ID、步骤状态）|
| 触发方式 | LLM Function Call | API 接口或工作流编排 |
| 典型例子 | `search_knowledge` | `LessonPrepController.Start` |

本项目备课工作流采用 Controller + Service 直接编排，实现了类似 SKILL 的能力，并通过 SSE 与前端实时交互。
