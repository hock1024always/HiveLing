package tools

import (
	"context"
	"encoding/json"
	"errors"
)

// 错误定义
var (
	ErrToolNotFound = errors.New("tool not found")
	ErrInvalidInput = errors.New("invalid tool input")
)

// Tool 工具接口（借鉴 Eino ToolsNode 设计）
// 实现此接口的工具可以自描述并被 Agent 自动调用
type Tool interface {
	// Info 返回工具的元数据信息
	Info() *ToolInfo
	// Run 执行工具调用
	Run(ctx context.Context, input string) (string, error)
}

// ToolInfo 工具元数据
type ToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolRegistry 工具注册表
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry 创建新的工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
func (r *ToolRegistry) Register(tool Tool) {
	info := tool.Info()
	r.tools[info.Name] = tool
}

// Get 获取工具
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// Execute 执行工具
func (r *ToolRegistry) Execute(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	tool, ok := r.Get(name)
	if !ok {
		return "", ErrToolNotFound
	}
	return tool.Run(ctx, string(arguments))
}

// List 列出所有工具
func (r *ToolRegistry) List() []Tool {
	list := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		list = append(list, tool)
	}
	return list
}

// ToLLMTools 转换为 LLM 工具格式
func (r *ToolRegistry) ToLLMTools() []LLMTool {
	list := r.List()
	tools := make([]LLMTool, 0, len(list))
	for _, tool := range list {
		info := tool.Info()
		tools = append(tools, info.ToLLMTool())
	}
	return tools
}

// LLMTool LLM 工具格式
// 与 llm.Tool 保持相同的 JSON 结构
type LLMTool struct {
	Type     string      `json:"type"`
	Function LLMFunction `json:"function"`
}

// LLMFunction LLM 函数定义
type LLMFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToLLMTool 将 ToolInfo 转换为 LLMTool
func (info *ToolInfo) ToLLMTool() LLMTool {
	return LLMTool{
		Type: "function",
		Function: LLMFunction{
			Name:        info.Name,
			Description: info.Description,
			Parameters:  info.Parameters,
		},
	}
}
