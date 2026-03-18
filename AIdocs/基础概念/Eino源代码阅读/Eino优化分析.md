# Eino 框架优化分析

> **更新状态**：P0 优化已实施完成（2026-03-18）

## 项目现状分析

### 对话助手模块

**当前实现**（`controllers/dialog.go`）：

```go
// 当前的问题：
// 1. 工具调用循环未完成（第167行 TODO）
if len(toolCalls) > 0 {
    // TODO: 实现完整的工具调用循环
}

// 2. SSE 手动处理
func (d *DialogController) sendSSE(c *gin.Context, event SSEEvent) {
    data, _ := json.Marshal(event)
    fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
    c.Writer.(http.Flusher).Flush()
}

// 3. 组件耦合
type DialogController struct {
    llmClient    *llm.Client      // 直接依赖具体实现
    sessionMgr   *memory.SessionManager
    toolExecutor *mcp.ToolExecutor
    retriever    *rag.Retriever
}
```

### 备课工作流模块

**当前实现**（`services/lessonprep/workflow.go`）：

```go
// 状态机硬编码
const (
    StateInit             = "init"
    StateOutlineGenerating = "outline_generating"
    StateOutlineReady     = "outline_ready"
    // ... 9 个状态
)

// 状态转换分散在多个函数中
func (w *WorkflowManager) TransitionToState(workflowID string, newState string) error {
    // 手动状态转换逻辑
}
```

### 可观测性缺失

```
当前状态：
├── 无链路追踪
├── 无性能指标
├── 无执行日志结构化
└── 调试困难
```

## Eino 优化方案

### 方案一：Agent 编排优化（推荐）

**优化目标**：使用 Eino ADK 重构对话助手的工具调用循环

**当前问题**：
- 工具调用循环未完成
- Agent 逻辑分散在 controller
- 工具执行结果无法正确回传 LLM

**Eino 解决方案**：

```go
// 使用 Eino ADK 重构
package eino_integration

import (
    "context"
    "github.com/cloudwego/eino/components/chat_model"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/adk"
)

// 定义工具
type SearchKnowledgeTool struct {
    retriever *rag.Retriever
}

func (t *SearchKnowledgeTool) Info() *tool.ToolInfo {
    return &tool.ToolInfo{
        Name:        "search_knowledge",
        Description: "搜索春秋战国历史知识库",
        Parameters: &tool.Parameters{
            Type: "object",
            Properties: map[string]tool.Property{
                "query":    {Type: "string", Description: "搜索关键词"},
                "category": {Type: "string", Description: "分类（person/event/battle等）"},
            },
            Required: []string{"query"},
        },
    }
}

func (t *SearchKnowledgeTool) Run(ctx context.Context, input string) (string, error) {
    var args struct {
        Query    string `json:"query"`
        Category string `json:"category"`
    }
    json.Unmarshal([]byte(input), &args)
    
    results, err := t.retriever.Search(args.Query, args.Category, 5)
    if err != nil {
        return "", err
    }
    return rag.FormatResults(results), nil
}

// 创建 Agent
func NewDialogAgent(chatModel chat_model.ChatModel, retriever *rag.Retriever) (adk.Agent, error) {
    return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Model: chatModel,
        ToolsConfig: &adk.ToolsConfig{
            Tools: []tool.Tool{
                &SearchKnowledgeTool{retriever: retriever},
                &QueryGraphTool{},
                &GetTimelineTool{},
                &WebSearchTool{},
            },
        },
    })
}

// 使用 Agent
func (c *DialogController) ChatWithEino(ctx context.Context, message string) error {
    runner := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent: c.agent,
    })
    
    iter, err := runner.Run(ctx, &adk.RunInput{
        Messages: []*schema.Message{
            {Role: schema.User, Content: message},
        },
    })
    
    // 自动处理工具调用循环
    for {
        event, ok := iter.Next()
        if !ok {
            break
        }
        
        switch event.Type {
        case adk.EventMessage:
            // 发送文本内容
            c.sendSSE(SSEEvent{Type: "text", Content: event.Message.Content})
        case adk.EventToolCall:
            // 工具调用通知
            c.sendSSE(SSEEvent{Type: "tool_call", Tool: event.ToolCall.Name})
        case adk.EventToolResult:
            // 工具结果通知
            c.sendSSE(SSEEvent{Type: "tool_result", Result: event.ToolResult})
        }
    }
    
    return nil
}
```

**优化效果**：
- ✅ 完整的工具调用循环
- ✅ 自动处理多轮工具调用
- ✅ 代码更简洁、可维护

### 方案二：工作流编排优化

**优化目标**：使用 Eino Graph 重构备课工作流

**当前问题**：
- 状态机硬编码
- 状态转换逻辑分散
- 难以扩展和修改

**Eino 解决方案**：

```go
// 使用 Eino Graph 重构备课工作流
func NewLessonPrepGraph(chatModel chat_model.ChatModel, retriever *rag.Retriever) *compose.Graph {
    graph := compose.NewGraph[*LessonPrepInput, *LessonPrepOutput]()
    
    // 添加节点
    graph.AddLambdaNode("validate", validateInput)
    graph.AddChatModelNode("generate_outline", chatModel)
    graph.AddLambdaNode("parse_outline", parseOutline)
    graph.AddLambdaNode("generate_plan", generateLessonPlan)
    graph.AddLambdaNode("generate_ppt", generatePPT)
    
    // 添加边
    graph.AddEdge(compose.START, "validate")
    graph.AddEdge("validate", "generate_outline")
    graph.AddEdge("generate_outline", "parse_outline")
    
    // 条件边：大纲是否需要修改
    graph.AddConditionalEdge("parse_outline", func(ctx context.Context, o *OutlineResult) string {
        if o.NeedEdit {
            return "edit_outline"
        }
        return "generate_plan"
    }, map[string]string{
        "edit_outline":   "edit_outline",
        "generate_plan": "generate_plan",
    })
    
    graph.AddEdge("generate_plan", "generate_ppt")
    graph.AddEdge("generate_ppt", compose.END)
    
    return graph
}

// 编译并运行
func (w *WorkflowManager) Run(workflowID string, input *LessonPrepInput) error {
    graph := NewLessonPrepGraph(w.chatModel, w.retriever)
    runnable, _ := graph.Compile(ctx)
    
    // 流式执行
    stream, _ := runnable.Stream(ctx, input)
    for chunk := range stream {
        // 实时推送进度
        w.notifyProgress(workflowID, chunk)
    }
    
    return nil
}
```

**优化效果**：
- ✅ 声明式定义工作流
- ✅ 自动状态管理
- ✅ 易于扩展和修改

### 方案三：可观测性增强

**优化目标**：使用 Eino Callback 实现追踪和监控

**当前问题**：
- 无链路追踪
- 无性能指标
- 调试困难

**Eino 解决方案**：

```go
// 自定义回调实现追踪
type TraceCallback struct {
    traceID string
    spans   []Span
}

func (c *TraceCallback) OnStart(ctx context.Context, info *CallbackInfo) context.Context {
    span := Span{
        ID:        uuid.New().String(),
        TraceID:   c.traceID,
        NodeName:  info.NodeName,
        StartTime: time.Now(),
    }
    c.spans = append(c.spans, span)
    return context.WithValue(ctx, "span_id", span.ID)
}

func (c *TraceCallback) OnEnd(ctx context.Context, info *CallbackInfo) {
    // 记录执行时间
    duration := time.Since(info.StartTime)
    log.Printf("[Trace] %s: %v", info.NodeName, duration)
    
    // 上报指标
    metrics.RecordDuration("node_duration", duration, map[string]string{
        "node": info.NodeName,
    })
}

func (c *TraceCallback) OnStream(ctx context.Context, info *CallbackInfo, chunk interface{}) {
    // 记录流式数据
    log.Printf("[Stream] %s: %v", info.NodeName, chunk)
}

// 使用回调
func (c *DialogController) Chat(ctx context.Context, message string) {
    traceCallback := &TraceCallback{traceID: uuid.New().String()}
    
    result, _ := c.agent.Invoke(ctx, input,
        compose.WithCallbacks(traceCallback),
        compose.WithTrace(true),
    )
}
```

**优化效果**：
- ✅ 完整的链路追踪
- ✅ 性能指标收集
- ✅ 便于调试和优化

## 优化优先级

| 优先级 | 方案 | 工作量 | 收益 |
|--------|------|--------|------|
| **P0** | Agent 编排优化 | 中 | 解决核心问题（工具调用循环） |
| **P1** | 可观测性增强 | 低 | 提升可维护性 |
| **P2** | 工作流编排优化 | 高 | 架构更清晰 |

## 实施建议

### 第一阶段：Agent 编排优化

1. 引入 Eino 依赖
2. 实现标准化的 Tool 接口
3. 使用 ADK 重构 DialogController
4. 测试验证工具调用循环

### 第二阶段：可观测性增强

1. 实现 TraceCallback
2. 集成到 Agent 执行流程
3. 添加日志和指标上报

### 第三阶段：工作流编排优化（可选）

1. 使用 Graph 重构备课工作流
2. 迁移现有状态机逻辑
3. 测试验证工作流

## 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Eino 版本不稳定 | 中 | 使用稳定版本，关注更新 |
| 学习成本 | 低 | 文档完善，示例丰富 |
| 兼容性问题 | 中 | 逐步迁移，保持接口兼容 |

## 结论

Eino 框架可以有效解决项目中的以下问题：

1. **工具调用循环**：ADK 自动处理多轮工具调用
2. **组件标准化**：统一的接口定义，便于替换和测试
3. **可观测性**：Callback 机制实现追踪和监控
4. **代码质量**：声明式编排，代码更简洁

建议优先实施 **Agent 编排优化**，这是解决当前核心问题（工具调用循环未完成）最直接有效的方式。

---

## 实施记录

### P0 优化已完成 ✅

**实施时间**：2026-03-18

**实施方式**：借鉴 Eino 设计思想，自实现 Agent 服务

**新增文件**：

| 文件 | 说明 |
|------|------|
| `services/agent/agent.go` | Agent 服务，包含工具调用循环、回调机制、指标统计 |

**修改文件**：

| 文件 | 修改内容 |
|------|----------|
| `controllers/dialog.go` | 使用新 Agent 服务重构 Chat 方法 |
| `services/llm/client.go` | Message 结构体增加 ToolCalls、ToolCallID 字段 |
| `router/router.go` | 添加 `/api/dialog/agent/stats` 接口 |

**核心改进**：

1. **完整的工具调用循环**
   ```go
   // 工具调用循环（最多 5 轮）
   for round < a.config.MaxToolRounds {
       // 调用 LLM
       // 如果有工具调用 → 执行工具 → 添加工具结果 → 继续循环
       // 如果没有工具调用 → 返回结果
   }
   ```

2. **回调机制（借鉴 Eino Callback）**
   ```go
   type Callback interface {
       OnStart(ctx context.Context, info *CallbackInfo) context.Context
       OnEnd(ctx context.Context, info *CallbackInfo)
       OnError(ctx context.Context, info *CallbackInfo, err error)
       OnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent)
   }
   ```

3. **内置回调实现**
   - `LogCallback`：日志记录
   - `MetricsCallback`：指标统计
   - `TraceCallback`：链路追踪

**新增 API**：

```
GET /api/dialog/agent/stats  # 获取 Agent 统计指标
```

**响应示例**：
```json
{
  "success": true,
  "data": {
    "total_requests": 100,
    "total_duration": "5m30s",
    "avg_duration": "3.3s",
    "total_tool_calls": 45,
    "total_errors": 2
  }
}
```

**优化效果**：

| 指标 | 优化前 | 优化后 |
|------|--------|--------|
| 工具调用循环 | ❌ 未完成 | ✅ 完整实现 |
| 多轮工具调用 | ❌ 不支持 | ✅ 最多 5 轮 |
| 可观测性 | ❌ 无 | ✅ 日志+指标+追踪 |
| 代码结构 | 分散 | 集中在 Agent 服务 |

### 后续优化建议

| 优先级 | 方案 | 状态 |
|--------|------|------|
| P1 | 可观测性增强 | 部分完成（已实现回调机制） |
| P2 | 工作流编排优化 | 待实施 |
