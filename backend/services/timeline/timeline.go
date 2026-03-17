package timeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// TimelineEvent 时间线事件
type TimelineEvent struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	YearStart   int    `json:"year_start"`
	YearEnd     int    `json:"year_end"`
	Period      string `json:"period"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

// TimelineResult 时间线查询结果
type TimelineResult struct {
	StartYear int            `json:"start_year"`
	EndYear   int            `json:"end_year"`
	State     string         `json:"state,omitempty"`
	Period    string         `json:"period,omitempty"`
	Events    []TimelineEvent `json:"events"`
	Total     int            `json:"total"`
}

// Service 时间线服务
type Service struct{}

// NewService 创建时间线服务
func NewService() *Service {
	return &Service{}
}

// GetTimeline 获取时间线事件
// startYear: 起始年份（负数表示公元前，如 -770 表示公元前770年）
// endYear: 结束年份
// state: 诸侯国名称（可选）
// category: 事件类型（可选）：person, event, battle, school, state, culture
func (s *Service) GetTimeline(startYear, endYear int, state, category string) (*TimelineResult, error) {
	var chunks []models.KnowledgeChunk

	query := dao.Db.Model(&models.KnowledgeChunk{})

	// 年份范围过滤
	// 事件年份范围与查询范围有交集即可
	if startYear != 0 || endYear != 0 {
		if startYear != 0 && endYear != 0 {
			// 查询范围：[startYear, endYear]
			// 事件范围：[YearStart, YearEnd]
			// 交集条件：事件的结束年份 >= 查询起始年份 AND 事件的起始年份 <= 查询结束年份
			query = query.Where(
				"(year_end >= ? OR year_end = 0) AND (year_start <= ? OR year_start = 0)",
				startYear, endYear,
			)
		} else if startYear != 0 {
			query = query.Where("year_end >= ? OR year_end = 0", startYear)
		} else {
			query = query.Where("year_start <= ? OR year_start = 0", endYear)
		}
	}

	// 诸侯国过滤
	if state != "" {
		// 搜索标题或内容中包含该诸侯国名称的记录
		query = query.Where(
			"title LIKE ? OR content LIKE ? OR keywords LIKE ?",
			"%"+state+"%", "%"+state+"%", "%"+state+"%",
		)
	}

	// 分类过滤
	if category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}

	// 按起始年份排序
	query = query.Order("year_start ASC, year_end ASC")

	if err := query.Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}

	// 转换为时间线事件
	events := make([]TimelineEvent, 0, len(chunks))
	for _, chunk := range chunks {
		events = append(events, TimelineEvent{
			ID:          chunk.ID,
			Title:       chunk.Title,
			Category:    chunk.Category,
			YearStart:   chunk.YearStart,
			YearEnd:     chunk.YearEnd,
			Period:      chunk.Period,
			Description: s.truncateContent(chunk.Content, 200),
			Source:      chunk.Source,
		})
	}

	// 确定实际的时间范围
	actualStart := startYear
	actualEnd := endYear
	if len(events) > 0 {
		if actualStart == 0 || events[0].YearStart < actualStart {
			if events[0].YearStart != 0 {
				actualStart = events[0].YearStart
			}
		}
		if actualEnd == 0 || events[len(events)-1].YearEnd > actualEnd {
			if events[len(events)-1].YearEnd != 0 {
				actualEnd = events[len(events)-1].YearEnd
			}
		}
	}

	return &TimelineResult{
		StartYear: actualStart,
		EndYear:   actualEnd,
		State:     state,
		Events:    events,
		Total:     len(events),
	}, nil
}

// GetEventsByPeriod 按时期获取事件
func (s *Service) GetEventsByPeriod(period string, limit int) (*TimelineResult, error) {
	if limit <= 0 {
		limit = 50
	}

	var chunks []models.KnowledgeChunk
	query := dao.Db.Model(&models.KnowledgeChunk{})

	if period != "" {
		query = query.Where("period = ?", period)
	}

	query = query.Order("year_start ASC").Limit(limit)

	if err := query.Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}

	events := make([]TimelineEvent, 0, len(chunks))
	for _, chunk := range chunks {
		events = append(events, TimelineEvent{
			ID:          chunk.ID,
			Title:       chunk.Title,
			Category:    chunk.Category,
			YearStart:   chunk.YearStart,
			YearEnd:     chunk.YearEnd,
			Period:      chunk.Period,
			Description: s.truncateContent(chunk.Content, 200),
		})
	}

	return &TimelineResult{
		Period: period,
		Events: events,
		Total:  len(events),
	}, nil
}

// GetImportantEvents 获取重要事件（按类型）
func (s *Service) GetImportantEvents(category string, limit int) (*TimelineResult, error) {
	if limit <= 0 {
		limit = 20
	}

	var chunks []models.KnowledgeChunk
	query := dao.Db.Model(&models.KnowledgeChunk{})

	if category != "" && category != "all" {
		query = query.Where("category = ?", category)
	}

	// 优先选择有明确年份的事件
	query = query.Where("year_start != 0").
		Order("year_start ASC").
		Limit(limit)

	if err := query.Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}

	events := make([]TimelineEvent, 0, len(chunks))
	for _, chunk := range chunks {
		events = append(events, TimelineEvent{
			ID:          chunk.ID,
			Title:       chunk.Title,
			Category:    chunk.Category,
			YearStart:   chunk.YearStart,
			YearEnd:     chunk.YearEnd,
			Period:      chunk.Period,
			Description: s.truncateContent(chunk.Content, 200),
		})
	}

	return &TimelineResult{
		Events: events,
		Total:  len(events),
	}, nil
}

// SearchByYearRange 按年份范围搜索
func (s *Service) SearchByYearRange(year int, radius int) (*TimelineResult, error) {
	if radius <= 0 {
		radius = 10 // 默认前后10年
	}

	startYear := year - radius
	endYear := year + radius

	return s.GetTimeline(startYear, endYear, "", "")
}

// FormatYear 格式化年份显示
func (s *Service) FormatYear(year int) string {
	if year == 0 {
		return "年份不详"
	}
	if year < 0 {
		return fmt.Sprintf("公元前%d年", -year)
	}
	return fmt.Sprintf("公元%d年", year)
}

// FormatResult 格式化结果为可读文本
func (s *Service) FormatResult(result *TimelineResult) string {
	var builder strings.Builder

	if result.State != "" {
		builder.WriteString(fmt.Sprintf("【%s 相关历史事件】\n\n", result.State))
	} else if result.Period != "" {
		builder.WriteString(fmt.Sprintf("【%s时期历史事件】\n\n", result.Period))
	} else {
		builder.WriteString("【历史时间线】\n\n")
	}

	if result.StartYear != 0 || result.EndYear != 0 {
		builder.WriteString(fmt.Sprintf("时间范围：%s ~ %s\n\n",
			s.FormatYear(result.StartYear),
			s.FormatYear(result.EndYear)))
	}

	if len(result.Events) == 0 {
		builder.WriteString("未找到相关历史事件。\n")
		return builder.String()
	}

	// 按年份分组显示
	currentYear := 0
	for _, event := range result.Events {
		// 年份标题
		if event.YearStart != currentYear {
			currentYear = event.YearStart
			builder.WriteString(fmt.Sprintf("\n【%s】\n", s.FormatYear(event.YearStart)))
		}

		// 事件详情
		categoryName := s.getCategoryName(event.Category)
		builder.WriteString(fmt.Sprintf("  • [%s] %s\n", categoryName, event.Title))
		if event.Description != "" {
			builder.WriteString(fmt.Sprintf("    %s\n", event.Description))
		}
	}

	builder.WriteString(fmt.Sprintf("\n共 %d 条记录\n", result.Total))

	return builder.String()
}

// FormatResultJSON 格式化为 JSON
func (s *Service) FormatResultJSON(result *TimelineResult) string {
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

// truncateContent 截断内容
func (s *Service) truncateContent(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// getCategoryName 获取分类中文名
func (s *Service) getCategoryName(category string) string {
	names := map[string]string{
		"person":  "人物",
		"event":   "事件",
		"battle":  "战役",
		"school":  "学派",
		"state":   "诸侯国",
		"culture": "文化",
	}
	if name, ok := names[category]; ok {
		return name
	}
	return category
}

// SortEventsByYear 按年份排序事件
func SortEventsByYear(events []TimelineEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].YearStart == events[j].YearStart {
			return events[i].YearEnd < events[j].YearEnd
		}
		return events[i].YearStart < events[j].YearStart
	})
}
