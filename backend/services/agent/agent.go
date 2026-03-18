package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/mcp"
)

// AgentEvent Agent 事件类型
type AgentEventType string

const (
	EventTypeText       AgentEventType = "text"        // 文本内容
	EventTypeToolCall   AgentEventType = "tool_call"   // 工具调用
	EventTypeToolResult AgentEventType = "tool_result" // 工具结果
	EventTypeStatus     AgentEventType = "status"      // 状态更新
	EventTypeDone       AgentEventType = "done"        // 完成
	EventTypeError      AgentEventType = "error"       // 错误
)

// AgentEvent Agent 事件
type AgentEvent struct {
	Type      AgentEventType `json:"type"`
	Content   string         `json:"content,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	Arguments interface{}    `json:"arguments,omitempty"`
	Result    string         `json:"result,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// Callback Agent 回调接口（借鉴 Eino Callback 设计）
type Callback interface {
	OnStart(ctx context.Context, info *CallbackInfo) context.Context
	OnEnd(ctx context.Context, info *CallbackInfo)
	OnError(ctx context.Context, info *CallbackInfo, err error)
	OnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent)
}

// CallbackInfo 回调信息
type CallbackInfo struct {
	SessionID   string
	Query       string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	ToolCalls   int
	TokenUsage  int
	NodeName    string
}

// AgentConfig Agent 配置
type AgentConfig struct {
	MaxToolRounds   int           // 最大工具调用轮数
	Timeout         time.Duration // 超时时间
	EnableStreaming bool          // 是否启用流式
}

// DefaultAgentConfig 默认配置
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		MaxToolRounds:   5,
		Timeout:         60 * time.Second,
		EnableStreaming: true,
	}
}

// DialogAgent 对话 Agent（借鉴 Eino ADK 设计）
type DialogAgent struct {
	llmClient    *llm.Client
	toolExecutor *mcp.ToolExecutor
	config       *AgentConfig
	callbacks    []Callback
}

// NewDialogAgent 创建对话 Agent
func NewDialogAgent(config *AgentConfig) *DialogAgent {
	if config == nil {
		config = DefaultAgentConfig()
	}
	return &DialogAgent{
		llmClient:    llm.NewClient(),
		toolExecutor: mcp.NewToolExecutor(),
		config:       config,
		callbacks:    make([]Callback, 0),
	}
}

// WithCallbacks 添加回调
func (a *DialogAgent) WithCallbacks(callbacks ...Callback) *DialogAgent {
	a.callbacks = append(a.callbacks, callbacks...)
	return a
}

// Run 运行 Agent（支持流式回调）
func (a *DialogAgent) Run(ctx context.Context, messages []llm.Message, tools []llm.Tool, onEvent func(event *AgentEvent)) (*AgentResult, error) {
	info := &CallbackInfo{
		Query:     a.extractQuery(messages),
		StartTime: time.Now(),
	}
	
	// 触发开始回调
	ctx = a.triggerOnStart(ctx, info)
	
	result := &AgentResult{
		Messages: messages,
	}
	
	defer func() {
		info.EndTime = time.Now()
		info.Duration = info.EndTime.Sub(info.StartTime)
		a.triggerOnEnd(ctx, info)
	}()

	// 工具调用循环
	round := 0
	for round < a.config.MaxToolRounds {
		round++
		
		// 发送状态
		a.emitEvent(ctx, info, onEvent, &AgentEvent{
			Type:    EventTypeStatus,
			Content: fmt.Sprintf("第 %d 轮思考...", round),
		})

		// 调用 LLM
		var fullContent strings.Builder
		var toolCalls []llm.ToolCall
		var finishReason string

		if a.config.EnableStreaming {
			// 流式调用
			_, err := a.llmClient.StreamChat(messages, tools, func(chunk *llm.StreamChunk) error {
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta

					// 处理文本内容
					if delta.Content != "" {
						fullContent.WriteString(delta.Content)
						a.emitEvent(ctx, info, onEvent, &AgentEvent{
							Type:    EventTypeText,
							Content: delta.Content,
						})
					}

					// 处理工具调用
					if len(delta.ToolCalls) > 0 {
						toolCalls = append(toolCalls, delta.ToolCalls...)
					}

					// 记录结束原因
					if chunk.Choices[0].FinishReason != "" {
						finishReason = chunk.Choices[0].FinishReason
					}
				}
				return nil
			})

			if err != nil {
				a.triggerOnError(ctx, info, err)
				return nil, fmt.Errorf("LLM call failed: %v", err)
			}
		} else {
			// 非流式调用
			resp, err := a.llmClient.Chat(messages, tools)
			if err != nil {
				a.triggerOnError(ctx, info, err)
				return nil, fmt.Errorf("LLM call failed: %v", err)
			}

			if len(resp.Choices) > 0 {
				fullContent.WriteString(resp.Choices[0].Message.Content)
				toolCalls = resp.Choices[0].ToolCalls
				finishReason = resp.Choices[0].FinishReason
			}
		}

		// 如果没有工具调用或结束原因是 stop，返回结果
		if len(toolCalls) == 0 || finishReason == "stop" {
			result.Content = fullContent.String()
			result.FinishReason = finishReason
			break
		}

		// 处理工具调用
		info.ToolCalls += len(toolCalls)
		
		// 添加助手消息（包含工具调用）
		toolCallsJSON, _ := json.Marshal(toolCalls)
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   fullContent.String(),
			ToolCalls: string(toolCallsJSON),
		})

		// 执行每个工具调用
		for _, tc := range toolCalls {
			a.emitEvent(ctx, info, onEvent, &AgentEvent{
				Type:      EventTypeToolCall,
				ToolName:  tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})

			// 执行工具
			result, err := a.toolExecutor.ExecuteTool(tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%s"}`, err.Error())
			}

			a.emitEvent(ctx, info, onEvent, &AgentEvent{
				Type:     EventTypeToolResult,
				ToolName: tc.Function.Name,
				Result:   result,
			})

			// 添加工具结果消息
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// 发送完成事件
	a.emitEvent(ctx, info, onEvent, &AgentEvent{
		Type: EventTypeDone,
	})

	result.Rounds = round
	return result, nil
}

// AgentResult Agent 执行结果
type AgentResult struct {
	Content      string
	Messages     []llm.Message
	FinishReason string
	Rounds       int
}

// emitEvent 发送事件
func (a *DialogAgent) emitEvent(ctx context.Context, info *CallbackInfo, onEvent func(event *AgentEvent), event *AgentEvent) {
	if onEvent != nil {
		onEvent(event)
	}
	a.triggerOnEvent(ctx, info, event)
}

// 回调触发方法
func (a *DialogAgent) triggerOnStart(ctx context.Context, info *CallbackInfo) context.Context {
	for _, cb := range a.callbacks {
		ctx = cb.OnStart(ctx, info)
	}
	return ctx
}

func (a *DialogAgent) triggerOnEnd(ctx context.Context, info *CallbackInfo) {
	for _, cb := range a.callbacks {
		cb.OnEnd(ctx, info)
	}
}

func (a *DialogAgent) triggerOnError(ctx context.Context, info *CallbackInfo, err error) {
	for _, cb := range a.callbacks {
		cb.OnError(ctx, info, err)
	}
}

func (a *DialogAgent) triggerOnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent) {
	for _, cb := range a.callbacks {
		cb.OnEvent(ctx, info, event)
	}
}

// extractQuery 从消息中提取查询
func (a *DialogAgent) extractQuery(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

// ============ 内置回调实现 ============

// LogCallback 日志回调
type LogCallback struct{}

func (c *LogCallback) OnStart(ctx context.Context, info *CallbackInfo) context.Context {
	log.Printf("[Agent] Start: query=%s", info.Query)
	return ctx
}

func (c *LogCallback) OnEnd(ctx context.Context, info *CallbackInfo) {
	log.Printf("[Agent] End: duration=%v, tool_calls=%d", info.Duration, info.ToolCalls)
}

func (c *LogCallback) OnError(ctx context.Context, info *CallbackInfo, err error) {
	log.Printf("[Agent] Error: %v", err)
}

func (c *LogCallback) OnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent) {
	log.Printf("[Agent] Event: type=%s, content=%s", event.Type, event.Content)
}

// MetricsCallback 指标回调
type MetricsCallback struct {
	TotalRequests  int64
	TotalDuration  time.Duration
	TotalToolCalls int64
	TotalErrors    int64
}

func (c *MetricsCallback) OnStart(ctx context.Context, info *CallbackInfo) context.Context {
	c.TotalRequests++
	return ctx
}

func (c *MetricsCallback) OnEnd(ctx context.Context, info *CallbackInfo) {
	c.TotalDuration += info.Duration
	c.TotalToolCalls += int64(info.ToolCalls)
}

func (c *MetricsCallback) OnError(ctx context.Context, info *CallbackInfo, err error) {
	c.TotalErrors++
}

func (c *MetricsCallback) OnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent) {
	// 暂不处理事件
}

func (c *MetricsCallback) GetStats() map[string]interface{} {
	avgDuration := time.Duration(0)
	if c.TotalRequests > 0 {
		avgDuration = time.Duration(int64(c.TotalDuration) / c.TotalRequests)
	}
	return map[string]interface{}{
		"total_requests":   c.TotalRequests,
		"total_duration":   c.TotalDuration.String(),
		"avg_duration":     avgDuration.String(),
		"total_tool_calls": c.TotalToolCalls,
		"total_errors":     c.TotalErrors,
	}
}

// TraceCallback 追踪回调
type TraceCallback struct {
	TraceID string
	Spans   []SpanInfo
}

type SpanInfo struct {
	ID        string
	StartTime time.Time
	EndTime   time.Time
	NodeName  string
	Event     string
}

func (c *TraceCallback) OnStart(ctx context.Context, info *CallbackInfo) context.Context {
	c.Spans = append(c.Spans, SpanInfo{
		ID:        fmt.Sprintf("span-%d", len(c.Spans)+1),
		StartTime: time.Now(),
		NodeName:  "agent_run",
		Event:     "start",
	})
	return ctx
}

func (c *TraceCallback) OnEnd(ctx context.Context, info *CallbackInfo) {
	if len(c.Spans) > 0 {
		c.Spans[len(c.Spans)-1].EndTime = time.Now()
	}
}

func (c *TraceCallback) OnError(ctx context.Context, info *CallbackInfo, err error) {
	c.Spans = append(c.Spans, SpanInfo{
		ID:        fmt.Sprintf("span-%d", len(c.Spans)+1),
		StartTime: time.Now(),
		EndTime:   time.Now(),
		NodeName:  "error",
		Event:     err.Error(),
	})
}

func (c *TraceCallback) OnEvent(ctx context.Context, info *CallbackInfo, event *AgentEvent) {
	c.Spans = append(c.Spans, SpanInfo{
		ID:        fmt.Sprintf("span-%d", len(c.Spans)+1),
		StartTime: time.Now(),
		NodeName:  string(event.Type),
		Event:     event.Content,
	})
}
