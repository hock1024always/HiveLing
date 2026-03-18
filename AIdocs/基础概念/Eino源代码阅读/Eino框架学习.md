# Eino 框架学习

## 概述

Eino ['aino]（近似音: i know）是字节跳动开源的 **Go 语言大模型应用开发框架**，旨在帮助开发者更快、更优雅地构建可靠的 AI 应用。

### 为什么需要 Eino

在开发 LLM 应用时，开发者面临以下挑战：

| 挑战 | 传统方式的问题 | Eino 的解决方案 |
|------|---------------|----------------|
| 组件碎片化 | 各组件接口不统一 | 标准化 Components 抽象 |
| 流式处理复杂 | 手动处理 SSE、拼接 | 自动流式处理 |
| 工作流编排 | 硬编码、难以维护 | Chain/Graph 声明式编排 |
| 可观测性缺失 | 难以追踪调试 | 统一 Callback 机制 |
| Agent 开发难 | 需要自己实现循环 | ADK 开发套件 |

### 核心特性

```
┌─────────────────────────────────────────────────────────────────┐
│                    Eino 核心特性                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. 组件化设计 (Components)                                     │
│     - ChatModel、Tool、Retriever 等标准化接口                   │
│     - 可替换实现，解耦业务逻辑                                  │
│                                                                 │
│  2. 编排能力 (Composition)                                      │
│     - Chain：线性流水线                                         │
│     - Graph：复杂有向图                                         │
│     - 声明式定义，自动执行                                      │
│                                                                 │
│  3. 流式优先 (Stream First)                                     │
│     - 自动处理流式数据                                          │
│     - 拼接、合并、转换                                          │
│                                                                 │
│  4. Agent 开发套件 (ADK)                                        │
│     - ChatModelAgent：简单 Agent                                │
│     - DeepAgent：复杂多步 Agent                                 │
│     - 工具调用、中断恢复                                        │
│                                                                 │
│  5. 可观测性 (Observability)                                    │
│     - Callback：生命周期钩子                                    │
│     - Trace：链路追踪                                           │
│     - Metrics：指标收集                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Eino 架构图                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    应用层 (Application)                   │   │
│  │         Agent / Chain / Graph 编排                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    编排层 (Composition)                   │   │
│  │      Chain / Graph / Workflow / Runnable                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    组件层 (Components)                    │   │
│  │  ChatModel │ Tool │ Retriever │ Embedding │ Indexer     │   │
│  └─────────────────────────────────────────────────────────┘   │
│                            │                                    │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    基础层 (Infrastructure)                │   │
│  │      Schema │ Callback │ Stream │ Context               │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 目录结构

```
eino/
├── adk/                    # Agent Development Kit
│   ├── agent.go           # Agent 接口定义
│   ├── chat_model_agent.go # 基础 Agent 实现
│   └── deep_agent.go      # 复杂 Agent 实现
├── callbacks/              # 回调机制
│   ├── callback.go        # 回调接口
│   └── trace.go           # 追踪实现
├── components/             # 组件抽象
│   ├── chat_model/        # 聊天模型
│   ├── tool/              # 工具
│   ├── retriever/         # 检索器
│   ├── embedding/         # 向量化
│   └── indexer/           # 索引器
├── compose/                # 编排框架
│   ├── chain.go           # Chain 编排
│   ├── graph.go           # Graph 编排
│   └── runnable.go        # 可执行接口
├── flow/                   # 流程控制
├── schema/                 # 数据结构
│   ├── message.go         # 消息结构
│   ├── document.go        # 文档结构
│   └── tool_info.go       # 工具信息
└── utils/                  # 工具函数
```

## 核心组件 (Components)

### 1. ChatModel

聊天模型组件，负责与 LLM 交互。

```go
// 接口定义
type ChatModel interface {
    // 生成回复
    Generate(ctx context.Context, input []*Message, opts ...Option) (*Message, error)
    
    // 流式生成
    Stream(ctx context.Context, input []*Message, opts ...Option) (*StreamReader, error)
    
    // 绑定工具
    BindTools(tools []*ToolInfo) ChatModel
}

// 使用示例
chatModel, _ := openai.NewChatModel(ctx, &openai.ChatModelConfig{
    Model:  "gpt-4o",
    APIKey: os.Getenv("OPENAI_API_KEY"),
})

// 流式调用
stream, _ := chatModel.Stream(ctx, []*schema.Message{
    {Role: schema.User, Content: "你好"},
})

for chunk := range stream {
    fmt.Println(chunk.Content)
}
```

### 2. Tool

工具组件，定义可被 Agent 调用的能力。

```go
// 接口定义
type Tool interface {
    // 工具信息
    Info() *ToolInfo
    
    // 执行工具
    Run(ctx context.Context, input string) (string, error)
}

// 工具信息结构
type ToolInfo struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  *Parameters `json:"parameters"`
}

// 自定义工具示例
type SearchTool struct{}

func (t *SearchTool) Info() *ToolInfo {
    return &ToolInfo{
        Name:        "search",
        Description: "搜索知识库",
        Parameters: &Parameters{
            Type: "object",
            Properties: map[string]Property{
                "query": {Type: "string", Description: "搜索关键词"},
            },
            Required: []string{"query"},
        },
    }
}

func (t *SearchTool) Run(ctx context.Context, input string) (string, error) {
    // 执行搜索逻辑
    return "搜索结果...", nil
}
```

### 3. Retriever

检索器组件，用于 RAG 场景。

```go
// 接口定义
type Retriever interface {
    // 检索相关文档
    Retrieve(ctx context.Context, query string, opts ...Option) ([]*Document, error)
}

// 文档结构
type Document struct {
    ID       string                 `json:"id"`
    Content  string                 `json:"content"`
    Metadata map[string]interface{} `json:"metadata"`
}
```

### 4. Embedding

向量化组件。

```go
// 接口定义
type Embedding interface {
    // 批量向量化
    EmbedStrings(ctx context.Context, texts []string, opts ...Option) ([][]float64, error)
}
```

## 编排系统 (Composition)

### Chain 编排

Chain 是线性的流水线编排，数据依次流过各个节点。

```go
// 创建 Chain
chain := compose.NewChain[*Input, *Output]()

// 添加节点
chain.
    AppendChatModel(chatModel).
    AppendLambda(processFunc).
    AppendChatModel(chatModel2)

// 编译并运行
runnable, _ := chain.Compile(ctx)
result, _ := runnable.Invoke(ctx, input)

// 流式运行
stream, _ := runnable.Stream(ctx, input)
for chunk := range stream {
    fmt.Println(chunk)
}
```

### Graph 编排

Graph 是有向图编排，支持分支、并行等复杂逻辑。

```go
// 创建 Graph
graph := compose.NewGraph[*Input, *Output]()

// 添加节点
graph.AddChatModelNode("llm", chatModel)
graph.AddLambdaNode("process", processFunc)
graph.AddToolsNode("tools", toolsNode)

// 添加边
graph.AddEdge(compose.START, "llm")
graph.AddEdge("llm", "process")
graph.AddEdge("process", "tools")
graph.AddEdge("tools", compose.END)

// 条件边
graph.AddConditionalEdge("process", func(ctx context.Context, input *Output) string {
    if input.NeedTools {
        return "tools"
    }
    return compose.END
}, map[string]string{"tools": "tools", compose.END: compose.END})

// 编译并运行
runnable, _ := graph.Compile(ctx)
result, _ := runnable.Invoke(ctx, input)
```

### Runnable 接口

所有可执行组件都实现 Runnable 接口：

```go
type Runnable[I, O any] interface {
    // 同步执行
    Invoke(ctx context.Context, input I, opts ...Option) (O, error)
    
    // 流式执行
    Stream(ctx context.Context, input I, opts ...Option) (*StreamReader[O], error)
    
    // 批量执行
    Batch(ctx context.Context, inputs []I, opts ...Option) ([]O, error)
}
```

## Agent 开发套件 (ADK)

### ChatModelAgent

最基础的 Agent，封装了 LLM 调用和工具使用。

```go
// 创建 Agent
agent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model: chatModel,
    ToolsConfig: &adk.ToolsConfig{
        Tools: []tool.Tool{
            &SearchTool{},
            &CalculatorTool{},
        },
    },
})

// 创建 Runner
runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent: agent,
})

// 运行
iter, _ := runner.Run(ctx, &adk.RunInput{
    Messages: []*schema.Message{
        {Role: schema.User, Content: "搜索齐桓公的信息"},
    },
})

// 迭代获取结果
for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    
    switch event.Type {
    case adk.EventMessage:
        fmt.Println(event.Message.Content)
    case adk.EventToolCall:
        fmt.Printf("调用工具: %s\n", event.ToolCall.Name)
    case adk.EventToolResult:
        fmt.Printf("工具结果: %s\n", event.ToolResult)
    }
}
```

### DeepAgent

复杂 Agent，支持子 Agent 协作和深度推理。

```go
// 创建子 Agent
researchAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model: chatModel,
    ToolsConfig: &adk.ToolsConfig{
        Tools: []tool.Tool{&SearchTool{}},
    },
})

codeAgent, _ := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model: chatModel,
    ToolsConfig: &adk.ToolsConfig{
        Tools: []tool.Tool{&CodeExecuteTool{}},
    },
})

// 创建 DeepAgent
deepAgent, _ := deep.New(ctx, &deep.Config{
    ChatModel: chatModel,
    SubAgents: []adk.Agent{researchAgent, codeAgent},
    ToolsConfig: &adk.ToolsConfig{
        Tools: []tool.Tool{&PlanningTool{}},
    },
})
```

### 中断与恢复

Agent 支持中断和恢复，适合需要人工确认的场景。

```go
// 运行时检查点
iter, _ := runner.Run(ctx, input)

for {
    event, ok := iter.Next()
    if !ok {
        break
    }
    
    if event.Type == adk.EventInterrupt {
        // 需要人工确认
        userConfirm := askUser(event.InterruptMessage)
        
        // 恢复执行
        iter.Resume(ctx, userConfirm)
    }
}
```

## 流式处理

### 自动流式处理

Eino 自动处理流式数据的拼接和转换：

```go
// 流式输入 → 流式输出
stream, _ := runnable.Stream(ctx, input)

// 自动拼接
for chunk := range stream {
    fmt.Print(chunk.Content)  // 逐字输出
}
```

### 流式数据结构

```go
// 流式读取器
type StreamReader[T any] struct {
    ch   chan T
    done chan struct{}
}

// 读取下一个
func (r *StreamReader[T]) Next() (T, bool)

// 关闭
func (r *StreamReader[T]) Close()
```

## 回调系统 (Callbacks)

### 回调接口

```go
type Callback interface {
    // 执行前
    OnStart(ctx context.Context, info *CallbackInfo) context.Context
    
    // 执行后
    OnEnd(ctx context.Context, info *CallbackInfo)
    
    // 错误
    OnError(ctx context.Context, info *CallbackInfo, err error)
    
    // 流式数据
    OnStream(ctx context.Context, info *CallbackInfo, chunk interface{})
}
```

### 使用回调

```go
// 自定义回调
type LogCallback struct{}

func (c *LogCallback) OnStart(ctx context.Context, info *CallbackInfo) context.Context {
    log.Printf("开始执行: %s", info.NodeName)
    return ctx
}

func (c *LogCallback) OnEnd(ctx context.Context, info *CallbackInfo) {
    log.Printf("执行完成: %s, 耗时: %v", info.NodeName, info.Duration)
}

// 注册回调
runnable.Invoke(ctx, input, compose.WithCallbacks(&LogCallback{}))
```

### 追踪 (Trace)

```go
// 开启追踪
runnable.Invoke(ctx, input, compose.WithTrace(true))

// 获取追踪信息
trace := compose.GetTrace(ctx)
fmt.Println(trace.String())
```

## 与项目对比

### 我们项目 vs Eino

| 维度 | 我们项目 | Eino |
|------|---------|------|
| **ChatModel** | `services/llm/client.go` | 标准化接口 |
| **Tool** | `services/mcp/tools.go` | 标准化接口 + 自动编排 |
| **Retriever** | `services/rag/retriever.go` | 标准化接口 |
| **编排** | 手动调用 | Chain/Graph 声明式 |
| **流式** | 手动 SSE | 自动处理 |
| **Agent** | 分散在 controller | ADK 统一封装 |
| **Callback** | 无 | 完整生命周期钩子 |

### 可优化方向

1. **Agent 编排**：用 Eino ADK 重构对话助手的工具调用循环
2. **工作流编排**：用 Graph 重构备课工作流状态机
3. **流式处理**：简化 SSE 处理逻辑
4. **可观测性**：添加 Callback 实现追踪和监控

## 最佳实践

### 1. 组件解耦

```go
// 不好的做法：硬编码模型
func chat(prompt string) string {
    resp, _ := openai.Chat(prompt)
    return resp
}

// 好的做法：依赖注入
func chat(ctx context.Context, model ChatModel, prompt string) string {
    resp, _ := model.Generate(ctx, []*Message{
        {Role: User, Content: prompt},
    })
    return resp.Content
}
```

### 2. 使用 Graph 管理复杂流程

```go
// 不好的做法：嵌套 if-else
if result.NeedSearch {
    searchResult := search(query)
    if searchResult.HasData {
        return process(searchResult)
    } else {
        return fallback()
    }
}

// 好的做法：Graph 条件边
graph.AddConditionalEdge("decision", func(ctx context.Context, r *Result) string {
    if r.NeedSearch && r.HasData {
        return "process"
    }
    return "fallback"
}, map[string]string{"process": "process", "fallback": "fallback"})
```

### 3. 利用回调实现可观测性

```go
// 统一添加日志、追踪、指标
runnable.Invoke(ctx, input,
    compose.WithCallbacks(
        &LogCallback{},
        &TraceCallback{},
        &MetricsCallback{},
    ),
)
```

## 参考资料

- [Eino GitHub](https://github.com/cloudwego/eino)
- [Eino 官方文档](https://www.cloudwego.io/zh/docs/eino/)
- [CloudWeGo Eino 实践](https://www.cloudwego.io/zh/docs/eino/overview/bytedance_eino_practice/)
