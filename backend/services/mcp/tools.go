package mcp

import (
	"context"
	"encoding/json"

	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/tools"
)

// ToolExecutor 工具执行器
// 现在基于标准化的 tools.Tool 接口实现
type ToolExecutor struct {
	registry *tools.ToolRegistry
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor() *ToolExecutor {
	executor := &ToolExecutor{
		registry: tools.NewToolRegistry(),
	}

	// 注册所有标准工具
	executor.registerDefaultTools()

	return executor
}

// registerDefaultTools 注册默认工具集
func (e *ToolExecutor) registerDefaultTools() {
	// 注册知识库搜索工具
	e.registry.Register(tools.NewSearchKnowledgeTool())

	// 注册知识图谱查询工具
	e.registry.Register(tools.NewQueryGraphTool())

	// 注册时间线查询工具
	e.registry.Register(tools.NewGetTimelineTool())

	// 注册联网搜索工具
	e.registry.Register(tools.NewWebSearchTool())
}

// ExecuteTool 执行工具调用（兼容旧接口）
func (e *ToolExecutor) ExecuteTool(toolName string, arguments json.RawMessage) (string, error) {
	return e.registry.Execute(context.Background(), toolName, arguments)
}

// ExecuteWithContext 带上下文的工具执行
func (e *ToolExecutor) ExecuteWithContext(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	return e.registry.Execute(ctx, toolName, arguments)
}

// GetTools 获取所有工具定义（用于 LLM）
func (e *ToolExecutor) GetTools() []llm.Tool {
	return e.toLLMTools(e.registry.ToLLMTools())
}

// GetToolsByMode 根据模式获取工具列表
// mode: "local" - 仅本地工具, "online" - 包含联网搜索, "auto" - 自动判断
func (e *ToolExecutor) GetToolsByMode(mode string) []llm.Tool {
	allTools := e.registry.ToLLMTools()

	if mode == "local" {
		// 过滤掉 web_search 工具
		localTools := make([]tools.LLMTool, 0, len(allTools)-1)
		for _, tool := range allTools {
			if tool.Function.Name != "web_search" {
				localTools = append(localTools, tool)
			}
		}
		return e.toLLMTools(localTools)
	}

	// online 或 auto 模式返回所有工具
	return e.toLLMTools(allTools)
}

// toLLMTools 将 tools.LLMTool 转换为 llm.Tool
func (e *ToolExecutor) toLLMTools(toolList []tools.LLMTool) []llm.Tool {
	result := make([]llm.Tool, 0, len(toolList))
	for _, t := range toolList {
		result = append(result, llm.Tool{
			Type: t.Type,
			Function: llm.ToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}
	return result
}

// RegisterTool 注册自定义工具
func (e *ToolExecutor) RegisterTool(tool tools.Tool) {
	e.registry.Register(tool)
}

// ListTools 列出所有可用工具
func (e *ToolExecutor) ListTools() []string {
	toolList := e.registry.List()
	names := make([]string, 0, len(toolList))
	for _, tool := range toolList {
		names = append(names, tool.Info().Name)
	}
	return names
}
