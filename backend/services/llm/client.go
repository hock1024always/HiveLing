package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hock1024always/GoEdu/config"
)

// Message 消息结构
type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCalls  string `json:"tool_calls,omitempty"`  // 工具调用 JSON
	ToolCallID string `json:"tool_call_id,omitempty"` // 工具调用 ID（用于工具结果消息）
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function FunctionCall    `json:"function"`
}

type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Tool 工具定义
// 为了保持兼容性，使用别名指向 tools 包的类型
type Tool = struct {
	Type     string      `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction = struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// ChatResponse 聊天响应（非流式）
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int        `json:"index"`
		Message      Message    `json:"message"`
		Delta        Delta      `json:"delta,omitempty"`
		FinishReason string     `json:"finish_reason"`
		ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Delta 流式响应的增量内容
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// StreamChunk 流式响应块
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Client DeepSeek 客户端
type Client struct {
	APIKey  string
	BaseURL string
	Model   string
	Client  *http.Client
}

// NewClient 创建新的 DeepSeek 客户端
func NewClient() *Client {
	return &Client{
		APIKey:  config.AppConfig.DeepSeek.APIKey,
		BaseURL: config.AppConfig.DeepSeek.BaseURL,
		Model:   config.AppConfig.DeepSeek.Model,
		Client:  &http.Client{},
	}
}

// Chat 非流式聊天
func (c *Client) Chat(messages []Message, tools []Tool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   false,
		Tools:    tools,
	}

	return c.doRequest(req)
}

// StreamChat 流式聊天
func (c *Client) StreamChat(messages []Message, tools []Tool, onChunk func(chunk *StreamChunk) error) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	}

	return c.doStreamRequest(req, onChunk)
}

// doRequest 执行非流式请求
func (c *Client) doRequest(chatReq ChatRequest) (*ChatResponse, error) {
	jsonData, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d, response: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &chatResp, nil
}

// doStreamRequest 执行流式请求
func (c *Client) doStreamRequest(chatReq ChatRequest, onChunk func(chunk *StreamChunk) error) (*ChatResponse, error) {
	jsonData, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d, response: %s", resp.StatusCode, string(body))
	}

	// 解析 SSE 流
	scanner := bufio.NewScanner(resp.Body)
	var finalResponse ChatResponse
	var contentBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式：data: {...}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// 检查是否结束
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// 调用回调函数
		if err := onChunk(&chunk); err != nil {
			return nil, err
		}

		// 累积内容
		if len(chunk.Choices) > 0 {
			contentBuilder.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	// 构建最终响应
	finalResponse.Model = c.Model
	if len(finalResponse.Choices) == 0 {
		finalResponse.Choices = make([]struct {
			Index        int        `json:"index"`
			Message      Message    `json:"message"`
			Delta        Delta      `json:"delta,omitempty"`
			FinishReason string     `json:"finish_reason"`
			ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
		}, 1)
	}
	finalResponse.Choices[0].Message.Content = contentBuilder.String()

	return &finalResponse, nil
}

// BuildTools 构建工具列表
func BuildTools(mode string) []Tool {
	tools := []Tool{
		// 本地知识库搜索
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "search_knowledge",
				Description: "搜索春秋战国历史知识库，包括人物传记、历史事件、战役详情、思想流派、诸侯国历史等内容。当用户询问春秋战国相关问题时优先使用此工具。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "搜索关键词或问题",
						},
						"category": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"person", "event", "battle", "school", "state", "culture", "all"},
							"description": "知识类别：person(人物)、event(事件)、battle(战役)、school(思想流派)、state(诸侯国)、culture(制度文化)、all(全部)",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "返回结果数量，默认5",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		// 知识图谱查询
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_graph",
				Description: "查询知识图谱中的人物关系、事件关联、国家关系等结构化信息。适用于查询'XX和XX是什么关系'、'XX参与了哪些事件'等问题。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"entity": map[string]interface{}{
							"type":        "string",
							"description": "要查询的实体名称（人物、国家、事件等）",
						},
						"relation_type": map[string]interface{}{
							"type":        "string",
							"description": "关系类型（可选）：BELONGS_TO(属于)、FOUNDED(创立)、PARTICIPATED(参与)、DEFEATED(击败)、ALLIED(结盟)等",
						},
					},
					"required": []string{"entity"},
				},
			},
		},
		// 时间线查询
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "get_timeline",
				Description: "获取春秋战国时期的重大事件时间线，可按时间范围或国家筛选。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"start_year": map[string]interface{}{
							"type":        "integer",
							"description": "起始年份（负数表示公元前，如-770表示公元前770年）",
						},
						"end_year": map[string]interface{}{
							"type":        "integer",
							"description": "结束年份（负数表示公元前）",
						},
						"state": map[string]interface{}{
							"type":        "string",
							"description": "按国家筛选（可选）",
						},
					},
				},
			},
		},
	}

	// 联网模式增加网络搜索工具
	if mode == "online" || mode == "auto" {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        "web_search",
				Description: "联网搜索历史资料。当本地知识库内容不足、需要最新研究资料、或问题超出春秋战国范围时使用。",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "搜索关键词",
						},
						"source": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"general", "academic"},
							"description": "搜索类型：general(通用搜索)、academic(学术搜索)",
						},
					},
					"required": []string{"query"},
				},
			},
		})
	}

	return tools
}
