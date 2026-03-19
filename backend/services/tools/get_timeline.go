package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hock1024always/GoEdu/services/timeline"
)

// GetTimelineTool 时间线查询工具
// 获取春秋战国时期的重大事件时间线，可按时间范围或国家筛选
type GetTimelineTool struct {
	timelineSvc *timeline.Service
}

// NewGetTimelineTool 创建时间线查询工具
func NewGetTimelineTool() *GetTimelineTool {
	return &GetTimelineTool{
		timelineSvc: timeline.NewService(),
	}
}

// Info 返回工具元数据
func (t *GetTimelineTool) Info() *ToolInfo {
	return &ToolInfo{
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
				"category": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"person", "event", "battle", "school", "state", "culture", "all"},
					"description": "事件类别（可选）",
				},
				"period": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"春秋", "战国"},
					"description": "历史时期（可选）：春秋、战国",
				},
			},
		},
	}
}

// Run 执行工具调用
func (t *GetTimelineTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		StartYear int    `json:"start_year"`
		EndYear   int    `json:"end_year"`
		State     string `json:"state"`
		Category  string `json:"category"`
		Period    string `json:"period"`
	}

	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	var result *timeline.TimelineResult
	var err error

	// 根据参数选择查询方式
	if args.Period != "" && args.StartYear == 0 && args.EndYear == 0 {
		// 按时期查询
		result, err = t.timelineSvc.GetEventsByPeriod(args.Period, 50)
	} else if args.StartYear != 0 || args.EndYear != 0 || args.State != "" {
		// 按年份范围和/或诸侯国查询
		result, err = t.timelineSvc.GetTimeline(args.StartYear, args.EndYear, args.State, args.Category)
	} else if args.Category != "" {
		// 按分类查询重要事件
		result, err = t.timelineSvc.GetImportantEvents(args.Category, 30)
	} else {
		// 默认返回春秋战国时期的重要事件
		result, err = t.timelineSvc.GetTimeline(-770, -221, "", "")
	}

	if err != nil {
		return "", fmt.Errorf("timeline query failed: %v", err)
	}

	return t.timelineSvc.FormatResultJSON(result), nil
}
