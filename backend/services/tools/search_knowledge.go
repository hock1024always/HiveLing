package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hock1024always/GoEdu/services/rag"
)

// SearchKnowledgeTool 知识库搜索工具
// 搜索春秋战国历史知识库，包括人物传记、历史事件、战役详情、思想流派、诸侯国历史等内容
type SearchKnowledgeTool struct {
	retriever *rag.Retriever
}

// NewSearchKnowledgeTool 创建知识库搜索工具
func NewSearchKnowledgeTool() *SearchKnowledgeTool {
	return &SearchKnowledgeTool{
		retriever: rag.NewRetriever(),
	}
}

// Info 返回工具元数据
func (t *SearchKnowledgeTool) Info() *ToolInfo {
	return &ToolInfo{
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
					"type": "string",
					"enum": []string{"person", "event", "battle", "school", "state", "culture", "all"},
					"description": "知识类别：person(人物)、event(事件)、battle(战役)、school(思想流派)、state(诸侯国)、culture(制度文化)、all(全部)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果数量，默认5",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Run 执行工具调用
func (t *SearchKnowledgeTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Query    string `json:"query"`
		Category string `json:"category"`
		Limit    int    `json:"limit"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if args.Query == "" {
		return "", fmt.Errorf("%w: query is required", ErrInvalidInput)
	}

	if args.Limit <= 0 {
		args.Limit = 5
	}

	results, err := t.retriever.Search(args.Query, args.Category, args.Limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %v", err)
	}

	return rag.FormatResults(results), nil
}
