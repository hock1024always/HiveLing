package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/agent"
	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/memory"
	"github.com/hock1024always/GoEdu/services/rag"
)

// DialogController 对话控制器
type DialogController struct {
	llmClient  *llm.Client
	sessionMgr *memory.SessionManager
	retriever  *rag.Retriever
	dialogAgent *agent.DialogAgent
	metrics    *agent.MetricsCallback
}

// NewDialogController 创建对话控制器
func NewDialogController() *DialogController {
	metrics := &agent.MetricsCallback{}
	dialogAgent := agent.NewDialogAgent(nil).
		WithCallbacks(&agent.LogCallback{}, metrics)

	return &DialogController{
		llmClient:   llm.NewClient(),
		sessionMgr:  memory.NewSessionManager(),
		retriever:   rag.NewRetriever(),
		dialogAgent: dialogAgent,
		metrics:     metrics,
	}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message   string `json:"message" binding:"required"`
	SessionID string `json:"session_id"`
	Mode      string `json:"mode"` // local, online, auto
}

// SSEEvent SSE 事件
type SSEEvent struct {
	Type      string      `json:"type"` // status, tool_call, tool_result, text, error, done
	Content   string      `json:"content,omitempty"`
	Tool      string      `json:"tool,omitempty"`
	Arguments interface{} `json:"arguments,omitempty"`
	Result    string      `json:"result,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
}

// Chat SSE 流式对话接口
func (d *DialogController) Chat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 获取或创建会话
	session, err := d.getOrCreateSession(req.SessionID, req.Mode)
	if err != nil {
		d.sendSSEError(c, err.Error())
		return
	}

	// 发送状态
	d.sendSSE(c, SSEEvent{Type: "status", Content: "正在分析问题..."})

	// 添加用户消息
	d.sessionMgr.AddMessage(session.ID, "user", req.Message, "", "")

	// 如果是首条消息，生成标题
	if req.SessionID == "" {
		d.sessionMgr.GenerateTitle(session.ID, req.Message)
	}

	// 构建上下文
	context, err := d.sessionMgr.BuildContext(session.ID, 20)
	if err != nil {
		d.sendSSEError(c, err.Error())
		return
	}

	// 添加系统提示
	systemPrompt := d.buildSystemPrompt(session.Mode)
	messages := append([]llm.Message{
		{Role: "system", Content: systemPrompt},
	}, context...)

	// 添加当前用户消息
	messages = append(messages, llm.Message{Role: "user", Content: req.Message})

	// 获取工具列表
	tools := llm.BuildTools(session.Mode)

	// 发送状态
	d.sendSSE(c, SSEEvent{Type: "status", Content: "正在思考..."})

	// 使用 Agent 执行（自动处理工具调用循环）
	result, err := d.dialogAgent.Run(c.Request.Context(), messages, tools, func(event *agent.AgentEvent) {
		switch event.Type {
		case agent.EventTypeText:
			d.sendSSE(c, SSEEvent{
				Type:    "text",
				Content: event.Content,
			})
		case agent.EventTypeToolCall:
			d.sendSSE(c, SSEEvent{
				Type:      "tool_call",
				Tool:      event.ToolName,
				Arguments: event.Arguments,
			})
			d.sendSSE(c, SSEEvent{
				Type:    "status",
				Content: fmt.Sprintf("正在调用 %s...", event.ToolName),
			})
		case agent.EventTypeToolResult:
			d.sendSSE(c, SSEEvent{
				Type:   "tool_result",
				Tool:   event.ToolName,
				Result: event.Result,
			})
		case agent.EventTypeStatus:
			d.sendSSE(c, SSEEvent{
				Type:    "status",
				Content: event.Content,
			})
		}
	})

	if err != nil {
		d.sendSSEError(c, err.Error())
		return
	}

	// 保存助手回复
	d.sessionMgr.AddMessage(session.ID, "assistant", result.Content, "", "")

	// 发送完成事件
	d.sendSSE(c, SSEEvent{
		Type:      "done",
		SessionID: session.ID,
	})
}

// getOrCreateSession 获取或创建会话
func (d *DialogController) getOrCreateSession(sessionID string, mode string) (*models.Session, error) {
	if mode == "" {
		mode = "auto"
	}

	if sessionID != "" {
		session, err := d.sessionMgr.GetSession(sessionID)
		if err != nil {
			return nil, err
		}
		if session != nil {
			return session, nil
		}
	}

	// 创建新会话
	session, err := d.sessionMgr.CreateSession(0, mode) // TODO: 从 token 获取用户 ID
	if err != nil {
		return nil, err
	}

	return session, nil
}

// buildSystemPrompt 构建系统提示
func (d *DialogController) buildSystemPrompt(mode string) string {
	basePrompt := `你是春秋战国历史教学助手，专注于公元前770年至公元前221年的中国历史。

你的职责：
1. 回答关于春秋战国时期的历史问题，包括人物、事件、战役、思想流派、诸侯国等
2. 提供准确、有深度的历史知识讲解
3. 引导用户深入理解历史事件的背景、原因和影响

回答原则：
- 准确性优先，基于史料和学术研究
- 条理清晰，适当引用史料原文
- 深入浅出，适合教学场景
- 如果问题超出春秋战国范围，诚实告知

当前模式：`

	switch mode {
	case "local":
		basePrompt += "本地知识库模式（仅使用本地知识，不联网）"
	case "online":
		basePrompt += "联网模式（可使用网络搜索补充资料）"
	default:
		basePrompt += "智能模式（自动判断是否需要联网）"
	}

	return basePrompt
}

// sendSSE 发送 SSE 事件
func (d *DialogController) sendSSE(c *gin.Context, event SSEEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
	c.Writer.(http.Flusher).Flush()
}

// sendSSEError 发送错误事件
func (d *DialogController) sendSSEError(c *gin.Context, errMsg string) {
	d.sendSSE(c, SSEEvent{
		Type:    "error",
		Content: errMsg,
	})
}

// GetHistory 获取会话历史
func (d *DialogController) GetHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	messages, err := d.sessionMgr.GetMessages(sessionID, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   messages,
	})
}

// ListSessions 列出会话列表
func (d *DialogController) ListSessions(c *gin.Context) {
	// TODO: 从 token 获取用户 ID
	sessions, err := d.sessionMgr.ListSessions(0, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
	})
}

// SearchKnowledge 知识库搜索接口（供前端直接调用）
func (d *DialogController) SearchKnowledge(c *gin.Context) {
	var req struct {
		Query    string `json:"query" binding:"required"`
		Category string `json:"category"`
		Limit    int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}

	results, err := d.retriever.Search(req.Query, req.Category, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
	})
}

// GetAgentStats 获取 Agent 统计指标
// GET /api/dialog/agent/stats
func (d *DialogController) GetAgentStats(c *gin.Context) {
	stats := d.metrics.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}
