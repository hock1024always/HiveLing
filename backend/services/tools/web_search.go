package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hock1024always/GoEdu/config"
)

// WebSearchTool 联网搜索工具
// 联网搜索历史资料，当本地知识库内容不足、需要最新研究资料、或问题超出春秋战国范围时使用
type WebSearchTool struct {
	apiKey  string
	baseURL string
}

// NewWebSearchTool 创建联网搜索工具
func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		apiKey:  config.AppConfig.Serper.APIKey,
		baseURL: config.AppConfig.Serper.BaseURL,
	}
}

// Info 返回工具元数据
func (t *WebSearchTool) Info() *ToolInfo {
	return &ToolInfo{
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
	}
}

// Run 执行工具调用
func (t *WebSearchTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Query  string `json:"query"`
		Source string `json:"source"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if args.Query == "" {
		return "", fmt.Errorf("%w: query is required", ErrInvalidInput)
	}

	if t.apiKey == "" || t.apiKey == "your_serper_api_key_here" {
		return "", fmt.Errorf("Serper API key not configured")
	}

	// 调用 Serper API
	return t.callSerperAPI(args.Query)
}

// SerperResponse Serper API 响应结构
type SerperResponse struct {
	Organic []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Snippet string `json:"snippet"`
	} `json:"organic"`
}

// callSerperAPI 调用 Serper 搜索 API
func (t *WebSearchTool) callSerperAPI(query string) (string, error) {
	// 构建请求
	reqBody := map[string]string{
		"q": query,
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", t.baseURL+"/search", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", t.apiKey)

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

	return t.formatResults(&serperResp), nil
}

// formatResults 格式化搜索结果
func (t *WebSearchTool) formatResults(resp *SerperResponse) string {
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
