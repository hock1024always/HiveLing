package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hock1024always/GoEdu/services/graph"
)

// QueryGraphTool 知识图谱查询工具
// 查询知识图谱中的人物关系、事件关联、国家关系等结构化信息
type QueryGraphTool struct {
	graphService *graph.QueryService
}

// NewQueryGraphTool 创建知识图谱查询工具
func NewQueryGraphTool() *QueryGraphTool {
	return &QueryGraphTool{
		graphService: graph.NewQueryService(),
	}
}

// Info 返回工具元数据
func (t *QueryGraphTool) Info() *ToolInfo {
	return &ToolInfo{
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
	}
}

// Run 执行工具调用
func (t *QueryGraphTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Entity       string `json:"entity"`
		RelationType string `json:"relation_type"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if args.Entity == "" {
		return "", fmt.Errorf("%w: entity is required", ErrInvalidInput)
	}

	// 查询实体及其关系
	result, err := t.graphService.QueryByName(args.Entity)
	if err != nil {
		// 如果找不到实体，尝试搜索
		nodes, searchErr := t.graphService.SearchEntities(args.Entity, 5)
		if searchErr != nil || len(nodes) == 0 {
			return "", fmt.Errorf("entity not found: %s", args.Entity)
		}

		// 返回搜索建议
		suggestions := make([]map[string]string, 0, len(nodes))
		for _, node := range nodes {
			suggestions = append(suggestions, map[string]string{
				"name":        node.Name,
				"type":        node.Type,
				"description": node.Description,
			})
		}
		data := map[string]interface{}{
			"found":       false,
			"suggestions": suggestions,
			"message":     "未找到精确匹配，您是否想查询以下实体？",
		}
		jsonBytes, _ := json.Marshal(data)
		return string(jsonBytes), nil
	}

	// 如果指定了关系类型，过滤结果
	if args.RelationType != "" {
		filteredRelations := make([]graph.Relation, 0)
		for _, rel := range result.Relations {
			if rel.Type == args.RelationType {
				filteredRelations = append(filteredRelations, rel)
			}
		}
		result.Relations = filteredRelations
	}

	return t.graphService.FormatResultJSON(result), nil
}
