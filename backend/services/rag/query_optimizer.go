package rag

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/nlp"
)

// QueryOptimizer 查询优化器
type QueryOptimizer struct {
	llmClient    *llm.Client
	jieba        *nlp.JiebaService
	corefResolver *nlp.LLMCoreferenceResolver
}

// NewQueryOptimizer 创建查询优化器
func NewQueryOptimizer() *QueryOptimizer {
	return &QueryOptimizer{
		llmClient:    llm.NewClient(),
		jieba:        nlp.GetJiebaService(),
		corefResolver: nlp.NewLLMCoreferenceResolver(),
	}
}

// QueryAnalysis 查询分析结果
type QueryAnalysis struct {
	OriginalQuery   string    `json:"original_query"`           // 原始查询
	ResolvedQuery   string    `json:"resolved_query"`           // 指代消解后的查询
	ExpandedQueries []string  `json:"expanded_queries"`         // 扩展查询（同义词、相关词）
	Intent          string    `json:"intent"`                   // 查询意图
	Entities        []string  `json:"entities"`                 // 提取的实体
	Keywords        []string  `json:"keywords"`                 // 关键词
	TimeRange       *TimeRange `json:"time_range,omitempty"`    // 时间范围
	CategoryHint    string    `json:"category_hint"`            // 分类提示
	NeedsFallback   bool      `json:"needs_fallback"`           // 是否需要回退到通用知识
	CorefConfidence float64   `json:"coref_confidence"`         // 指代消解置信度
}

// TimeRange 时间范围
type TimeRange struct {
	StartYear int `json:"start_year"`
	EndYear   int `json:"end_year"`
}

// OptimizeQuery 优化查询（带对话历史的版本）
func (o *QueryOptimizer) OptimizeQuery(query string) (*QueryAnalysis, error) {
	return o.OptimizeQueryWithContext(query, nil, 0)
}

// OptimizeQueryWithContext 带上下文的查询优化（多轮对话场景）
func (o *QueryOptimizer) OptimizeQueryWithContext(query string, dialogHistory []string, turn int) (*QueryAnalysis, error) {
	analysis := &QueryAnalysis{
		OriginalQuery:   query,
		ResolvedQuery:   query,
		CorefConfidence: 1.0,
	}

	// 1. 指代消解（多轮对话时消解代词和省略）
	if turn > 0 || len(dialogHistory) > 0 {
		corefResult, err := o.corefResolver.Resolve(query, dialogHistory, turn)
		if err == nil && corefResult.ResolvedQuery != query {
			analysis.ResolvedQuery = corefResult.ResolvedQuery
			analysis.CorefConfidence = corefResult.Confidence
		}
	}

	// 后续分析都基于消解后的查询
	workQuery := analysis.ResolvedQuery

	// 2. 提取实体和关键词
	analysis.Entities = o.extractEntities(workQuery)
	analysis.Keywords = o.extractKeywords(workQuery)

	// 3. 识别意图
	analysis.Intent = o.identifyIntent(workQuery)

	// 4. 解析时间范围
	analysis.TimeRange = o.parseTimeRange(workQuery)

	// 5. 生成分类提示
	analysis.CategoryHint = o.inferCategory(workQuery, analysis.Entities)

	// 6. 查询扩展（同义词、相关词）
	analysis.ExpandedQueries = o.expandQuery(workQuery, analysis.Entities)

	// 7. 判断是否需要回退
	analysis.NeedsFallback = o.needsFallback(workQuery, analysis)

	// 8. 更新消解器的实体上下文（供下一轮使用）
	jiebaEntities := o.jieba.ExtractEntities(workQuery)
	o.corefResolver.AddContext(jiebaEntities, turn)

	return analysis, nil
}

// ResetContext 重置指代消解上下文（新对话时调用）
func (o *QueryOptimizer) ResetContext() {
	o.corefResolver.Clear()
}

// extractEntities 提取实体（历史人物、事件、地点等）
func (o *QueryOptimizer) extractEntities(query string) []string {
	entities := make([]string, 0)

	// 使用jieba提取实体
	jiebaEntities := o.jieba.ExtractEntities(query)
	for _, e := range jiebaEntities {
		entities = append(entities, e.Text)
	}

	return uniqueStrings(entities)
}

// extractKeywords 提取关键词
func (o *QueryOptimizer) extractKeywords(query string) []string {
	// 使用jieba TF-IDF提取关键词
	keywords := make([]string, 0)
	kwList := o.jieba.ExtractKeywords(query, 10)
	for _, kw := range kwList {
		keywords = append(keywords, kw.Word)
	}
	return keywords
}

// identifyIntent 识别查询意图
func (o *QueryOptimizer) identifyIntent(query string) string {
	query = strings.ToLower(query)

	// 定义意图模式
	intentPatterns := map[string][]string{
		"definition":     {"是什么", "什么叫", "什么是", "定义", "意思"},
		"comparison":     {"区别", "对比", "比较", "vs", "和.*有什么不同", "vs\\."},
		"timeline":       {"时间", "什么时候", "年代", "时期", "年份", "先后顺序"},
		"cause_effect":   {"为什么", "原因", "导致", "结果", "影响"},
		"relationship":   {"关系", "联系", "关联", "有什么联系"},
		"process":        {"过程", "经过", "怎么发生", "如何发展"},
		"list":           {"有哪些", "都有谁", "列举", "名单"},
		"location":       {"在哪里", "地点", "位置", "疆域"},
		"evaluation":     {"评价", "看法", "意义", "重要性"},
	}

	for intent, patterns := range intentPatterns {
		for _, pattern := range patterns {
			if matched, _ := regexp.MatchString(pattern, query); matched {
				return intent
			}
		}
	}

	return "general"
}

// parseTimeRange 解析时间范围
func (o *QueryOptimizer) parseTimeRange(query string) *TimeRange {
	// 匹配时期
	if strings.Contains(query, "春秋") {
		return &TimeRange{StartYear: -770, EndYear: -476}
	}
	if strings.Contains(query, "战国") {
		return &TimeRange{StartYear: -475, EndYear: -221}
	}

	// 匹配具体年份
	yearPattern := regexp.MustCompile(`(?:公元前|前)?(\d{2,4})年`)
	matches := yearPattern.FindAllStringSubmatch(query, -1)
	if len(matches) >= 2 {
		// 有两个年份，认为是范围
		start := parseYear(matches[0][0])
		end := parseYear(matches[1][0])
		if start != 0 && end != 0 {
			return &TimeRange{StartYear: start, EndYear: end}
		}
	}

	return nil
}

// parseYear 解析年份字符串
func parseYear(yearStr string) int {
	yearStr = strings.ReplaceAll(yearStr, "公元前", "-")
	yearStr = strings.ReplaceAll(yearStr, "前", "-")
	yearStr = strings.ReplaceAll(yearStr, "年", "")

	var year int
	fmt.Sscanf(yearStr, "%d", &year)
	return year
}

// inferCategory 推断分类
func (o *QueryOptimizer) inferCategory(query string, entities []string) string {
	// 根据实体和关键词推断分类
	categoryKeywords := map[string][]string{
		"person": {"人物", "谁", "生平", "事迹", "传记"},
		"event":  {"事件", "战役", "战争", "发生", "过程"},
		"battle": {"之战", "战役", "战争", "战斗", "胜负"},
		"school": {"学派", "思想", "主张", "理念", "儒家", "道家", "墨家"},
		"state":  {"国家", "诸侯", "疆域", "都城", "国力"},
		"culture": {"制度", "礼仪", "文化", "习俗", "礼乐"},
	}

	for category, keywords := range categoryKeywords {
		for _, kw := range keywords {
			if strings.Contains(query, kw) {
				return category
			}
		}
	}

	// 根据实体推断
	for _, entity := range entities {
		if isPersonName(entity) {
			return "person"
		}
		if isStateName(entity) {
			return "state"
		}
		if isBattleName(entity) {
			return "battle"
		}
	}

	return "all"
}

// expandQuery 查询扩展
func (o *QueryOptimizer) expandQuery(query string, entities []string) []string {
	expanded := []string{query}

	// 同义词扩展
	synonyms := map[string][]string{
		"齐桓公": {"桓公", "小白", "齐小白"},
		"晋文公": {"文公", "重耳"},
		"孔子":   {"孔丘", "仲尼", "孔夫子"},
		"春秋":   {"春秋时期", "东周前期"},
		"战国":   {"战国时期", "东周后期"},
	}

	for entity, syns := range synonyms {
		if strings.Contains(query, entity) {
			for _, syn := range syns {
				newQuery := strings.ReplaceAll(query, entity, syn)
				expanded = append(expanded, newQuery)
			}
		}
	}

	// 添加相关概念
	relatedConcepts := map[string][]string{
		"齐桓公": {"管仲", "春秋五霸", "葵丘之盟"},
		"孔子":   {"儒家", "论语", "春秋", "周游列国"},
		"商鞅":   {"商鞅变法", "秦国", "法家"},
		"长平之战": {"白起", "赵括", "纸上谈兵", "秦国", "赵国"},
	}

	for entity, concepts := range relatedConcepts {
		if strings.Contains(query, entity) {
			for _, concept := range concepts {
				newQuery := query + " " + concept
				expanded = append(expanded, newQuery)
			}
		}
	}

	return uniqueStrings(expanded)
}

// needsFallback 判断是否需要回退到通用知识
func (o *QueryOptimizer) needsFallback(query string, analysis *QueryAnalysis) bool {
	// 如果查询包含以下特征，可能需要回退
	fallbackIndicators := []string{
		"现代", "今天", "现在", "当代", "今天",
		"为什么", "怎么看", "评价", "意义",
	}

	for _, indicator := range fallbackIndicators {
		if strings.Contains(query, indicator) {
			return true
		}
	}

	// 如果没有提取到任何实体，可能需要回退
	if len(analysis.Entities) == 0 && len(analysis.Keywords) < 2 {
		return true
	}

	return false
}

// isPersonName 判断是否是人物名（简单规则）
func isPersonName(s string) bool {
	// 常见的春秋战国人物名特征
	personTitles := []string{"公", "王", "侯", "子", "君", "父", "氏"}
	for _, title := range personTitles {
		if strings.Contains(s, title) {
			return true
		}
	}
	return false
}

// isStateName 判断是否是国家名
func isStateName(s string) bool {
	states := []string{"齐", "晋", "楚", "秦", "宋", "鲁", "卫", "郑", "吴", "越", "燕", "赵", "魏", "韩"}
	for _, state := range states {
		if strings.Contains(s, state) && strings.Contains(s, "国") {
			return true
		}
	}
	return false
}

// isBattleName 判断是否是战役名
func isBattleName(s string) bool {
	return strings.Contains(s, "之战") || strings.Contains(s, "战役") || strings.Contains(s, "战争")
}

// uniqueStrings 去重字符串切片
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// EnhancedRetriever 增强检索器
type EnhancedRetriever struct {
	baseRetriever *Retriever
	optimizer     *QueryOptimizer
}

// NewEnhancedRetriever 创建增强检索器
func NewEnhancedRetriever() *EnhancedRetriever {
	return &EnhancedRetriever{
		baseRetriever: NewRetriever(),
		optimizer:     NewQueryOptimizer(),
	}
}

// SmartSearch 智能搜索（带查询优化）
func (r *EnhancedRetriever) SmartSearch(query string, limit int) ([]SearchResult, error) {
	// 1. 分析查询
	analysis, err := r.optimizer.OptimizeQuery(query)
	if err != nil {
		// 分析失败，使用原始查询
		return r.baseRetriever.Search(query, "", limit)
	}

	// 2. 多查询并行检索
	allResults := make(map[uint]*SearchResult)

	// 原始查询
	results, _ := r.baseRetriever.Search(query, analysis.CategoryHint, limit)
	for i := range results {
		allResults[results[i].Chunk.ID] = &results[i]
	}

	// 扩展查询
	for _, expandedQuery := range analysis.ExpandedQueries {
		if expandedQuery == query {
			continue
		}
		results, _ := r.baseRetriever.Search(expandedQuery, analysis.CategoryHint, limit/2)
		for i := range results {
			if existing, ok := allResults[results[i].Chunk.ID]; ok {
				// 合并分数
				existing.Score = max(existing.Score, results[i].Score)
			} else {
				allResults[results[i].Chunk.ID] = &results[i]
			}
		}
	}

	// 3. 转换为切片并排序
	finalResults := make([]SearchResult, 0, len(allResults))
	for _, result := range allResults {
		finalResults = append(finalResults, *result)
	}

	// 4. 按分数排序
	sortResults(finalResults)

	// 5. 限制数量
	if len(finalResults) > limit {
		finalResults = finalResults[:limit]
	}

	return finalResults, nil
}

// FallbackSearch 回退搜索（当知识库无结果时）
func (r *EnhancedRetriever) FallbackSearch(query string) (string, error) {
	// 使用 LLM 直接回答
	prompt := fmt.Sprintf(`用户询问关于春秋战国时期的问题："%s"

知识库中没有找到完全匹配的内容。请基于你的知识，简要回答这个问题。
如果问题涉及现代概念或超出春秋战国时期范围，请说明并建议用户换个问法。

请用中文回答，控制在200字以内。`, query)

	// 使用 LLM 客户端直接回答
	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := r.optimizer.llmClient.Chat(messages, nil)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no response from LLM")
}

// sortResults 排序结果
func sortResults(results []SearchResult) {
	// 使用简单的冒泡排序（实际项目中可以使用 sort.Slice）
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
