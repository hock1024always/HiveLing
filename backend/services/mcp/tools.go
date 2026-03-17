package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/services/graph"
	"github.com/hock1024always/GoEdu/services/rag"
	"github.com/hock1024always/GoEdu/services/timeline"
)

// ToolExecutor 工具执行器
type ToolExecutor struct {
	retriever     *rag.Retriever
	graphQuery    *graph.QueryService
	timelineSvc   *timeline.Service
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		retriever:     rag.NewRetriever(),
		graphQuery:    graph.NewQueryService(),
		timelineSvc:   timeline.NewService(),
	}
}

// ExecuteTool 执行工具调用
func (e *ToolExecutor) ExecuteTool(toolName string, arguments json.RawMessage) (string, error) {
	switch toolName {
	case "search_knowledge":
		return e.executeSearchKnowledge(arguments)
	case "query_graph":
		return e.executeQueryGraph(arguments)
	case "get_timeline":
		return e.executeGetTimeline(arguments)
	case "web_search":
		return e.executeWebSearch(arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// executeSearchKnowledge 执行知识库搜索
func (e *ToolExecutor) executeSearchKnowledge(arguments json.RawMessage) (string, error) {
	var args struct {
		Query    string `json:"query"`
		Category string `json:"category"`
		Limit    int    `json:"limit"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %v", err)
	}

	if args.Limit <= 0 {
		args.Limit = 5
	}

	results, err := e.retriever.Search(args.Query, args.Category, args.Limit)
	if err != nil {
		return "", err
	}

	return rag.FormatResults(results), nil
}

// executeQueryGraph 执行知识图谱查询
func (e *ToolExecutor) executeQueryGraph(arguments json.RawMessage) (string, error) {
	var args struct {
		Entity       string `json:"entity"`
		RelationType string `json:"relation_type"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %v", err)
	}

	// 查询实体及其关系
	result, err := e.graphQuery.QueryByName(args.Entity)
	if err != nil {
		// 如果找不到实体，尝试搜索
		nodes, searchErr := e.graphQuery.SearchEntities(args.Entity, 5)
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
			"found":      false,
			"suggestions": suggestions,
			"message":    "未找到精确匹配，您是否想查询以下实体？",
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

	return e.graphQuery.FormatResultJSON(result), nil
}

// executeGetTimeline 执行时间线查询
func (e *ToolExecutor) executeGetTimeline(arguments json.RawMessage) (string, error) {
	var args struct {
		StartYear int    `json:"start_year"`
		EndYear   int    `json:"end_year"`
		State     string `json:"state"`
		Category  string `json:"category"`
		Period    string `json:"period"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %v", err)
	}

	var result *timeline.TimelineResult
	var err error

	// 根据参数选择查询方式
	if args.Period != "" && args.StartYear == 0 && args.EndYear == 0 {
		// 按时期查询
		result, err = e.timelineSvc.GetEventsByPeriod(args.Period, 50)
	} else if args.StartYear != 0 || args.EndYear != 0 || args.State != "" {
		// 按年份范围和/或诸侯国查询
		result, err = e.timelineSvc.GetTimeline(args.StartYear, args.EndYear, args.State, args.Category)
	} else if args.Category != "" {
		// 按分类查询重要事件
		result, err = e.timelineSvc.GetImportantEvents(args.Category, 30)
	} else {
		// 默认返回春秋战国时期的重要事件
		result, err = e.timelineSvc.GetTimeline(-770, -221, "", "")
	}

	if err != nil {
		return "", fmt.Errorf("timeline query failed: %v", err)
	}

	return e.timelineSvc.FormatResultJSON(result), nil
}

// executeWebSearch 执行联网搜索
func (e *ToolExecutor) executeWebSearch(arguments json.RawMessage) (string, error) {
	var args struct {
		Query  string `json:"query"`
		Source string `json:"source"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %v", err)
	}

	apiKey := config.AppConfig.Serper.APIKey
	if apiKey == "" || apiKey == "your_serper_api_key_here" {
		return "", fmt.Errorf("Serper API key not configured")
	}

	// 调用 Serper API
	return e.callSerperAPI(args.Query, apiKey)
}

// callSerperAPI 调用 Serper 搜索 API
func (e *ToolExecutor) callSerperAPI(query, apiKey string) (string, error) {
	baseURL := config.AppConfig.Serper.BaseURL

	// 构建请求
	reqBody := map[string]string{
		"q": query,
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/search", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %d, response: %s", resp.StatusCode, string(body))
	}

	// 解析并格式化结果
	var serperResp SerperResponse
	if err := json.Unmarshal(body, &serperResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	return e.formatSerperResults(&serperResp), nil
}

// SerperResponse Serper API 响应结构
type SerperResponse struct {
	Organic []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic"`
}

// formatSerperResults 格式化 Serper 搜索结果
func (e *ToolExecutor) formatSerperResults(resp *SerperResponse) string {
	results := make([]map[string]string, 0, len(resp.Organic))
	for _, item := range resp.Organic {
		results = append(results, map[string]string{
			"title":   item.Title,
			"url":     item.Link,
			"snippet": item.Snippet,
		})
	}

	data := map[string]interface{}{
		"source":  "web_search",
		"results": results,
	}

	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}
