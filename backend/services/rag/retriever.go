package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/cache"
	"gorm.io/gorm"
)

// SearchResult 搜索结果
type SearchResult struct {
	Chunk    *models.KnowledgeChunk `json:"chunk"`
	Score    float64                `json:"score"`
	Keywords []string               `json:"keywords"`
	Source   string                 `json:"source"` // "vector", "keyword", "hybrid", "cache"
}

// Retriever 检索器
type Retriever struct {
	milvusClient    *MilvusClient
	embeddingSvc    *EmbeddingService
	cacheSvc        *cache.CacheService
	useVectorSearch bool
	cacheEnabled    bool
}

// NewRetriever 创建检索器
func NewRetriever() *Retriever {
	milvusClient, err := NewMilvusClient()
	if err != nil {
		milvusClient = &MilvusClient{enabled: false}
	}

	cacheSvc := cache.NewCacheService()

	return &Retriever{
		milvusClient:    milvusClient,
		embeddingSvc:    NewEmbeddingService(),
		cacheSvc:        cacheSvc,
		useVectorSearch: config.AppConfig.Milvus.Enabled,
		cacheEnabled:    cacheSvc.IsEnabled(),
	}
}

// Search 搜索知识库（智能选择检索方式，带缓存）
func (r *Retriever) Search(query string, category string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	// 尝试从缓存获取
	if r.cacheEnabled {
		ctx := context.Background()
		cacheKey := cache.SearchCacheKey(query, category, limit, r.useVectorSearch)
		var cachedResults []SearchResult
		if err := r.cacheSvc.Get(ctx, cacheKey, &cachedResults); err == nil {
			// 缓存命中，标记来源
			for i := range cachedResults {
				cachedResults[i].Source = "cache"
			}
			return cachedResults, nil
		}
	}

	// 缓存未命中，执行检索
	var results []SearchResult
	var err error

	// 如果启用了向量检索，使用混合检索
	if r.useVectorSearch && r.milvusClient.IsEnabled() {
		results, err = r.HybridSearch(query, category, limit)
	} else {
		// 否则使用关键词检索
		results, err = r.KeywordSearch(query, category, limit)
	}

	if err != nil {
		return nil, err
	}

	// 异步缓存结果
	if r.cacheEnabled && len(results) > 0 {
		go func() {
			bgCtx := context.Background()
			cacheKey := cache.SearchCacheKey(query, category, limit, r.useVectorSearch)
			r.cacheSvc.Set(bgCtx, cacheKey, results, 10*time.Minute) // 缓存 10 分钟
		}()
	}

	return results, nil
}

// VectorSearch 向量检索
func (r *Retriever) VectorSearch(query string, category string, limit int) ([]SearchResult, error) {
	if !r.milvusClient.IsEnabled() {
		return nil, fmt.Errorf("vector search not enabled")
	}

	if limit <= 0 {
		limit = 5
	}

	// 获取查询向量
	queryVector, err := r.embeddingSvc.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %v", err)
	}

	// 执行向量搜索
	results, err := r.milvusClient.Search(queryVector, limit*2, category)
	if err != nil {
		return nil, fmt.Errorf("milvus search failed: %v", err)
	}

	// 转换为 SearchResult
	searchResults := make([]SearchResult, 0, len(results))
	for _, result := range results {
		chunk := &models.KnowledgeChunk{
			ID:       uint(result.ID),
			Title:    result.Title,
			Content:  result.Content,
			Category: result.Category,
		}
		searchResults = append(searchResults, SearchResult{
			Chunk:    chunk,
			Score:    float64(result.Score),
			Source:   "vector",
			Keywords: extractKeywords(result.Content),
		})
	}

	// 按分数排序并限制数量
	sort.Slice(searchResults, func(i, j int) bool {
		return searchResults[i].Score > searchResults[j].Score
	})

	if len(searchResults) > limit {
		searchResults = searchResults[:limit]
	}

	return searchResults, nil
}

// KeywordSearch 关键词检索
func (r *Retriever) KeywordSearch(query string, category string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	var chunks []models.KnowledgeChunk
	query = strings.TrimSpace(query)

	// 构建查询
	db := dao.Db.Model(&models.KnowledgeChunk{})

	// 分类筛选
	if category != "" && category != "all" {
		db = db.Where("category = ?", category)
	}

	// 使用 MySQL FULLTEXT 搜索（需要确保表有 FULLTEXT 索引）
	// 先尝试全文搜索
	err := db.Where("MATCH(title, content, keywords) AGAINST(? IN NATURAL LANGUAGE MODE)", query).
		Order(gorm.Expr("MATCH(title, content, keywords) AGAINST(? IN NATURAL LANGUAGE MODE) DESC", query)).
		Limit(limit).
		Find(&chunks).Error

	if err != nil || len(chunks) == 0 {
		// 如果全文搜索失败或无结果，使用 LIKE 模糊搜索
		keywords := extractKeywords(query)
		db = dao.Db.Model(&models.KnowledgeChunk{})
		if category != "" && category != "all" {
			db = db.Where("category = ?", category)
		}

		// 构建模糊查询条件
		conditions := make([]string, 0)
		args := make([]interface{}, 0)
		for _, kw := range keywords {
			conditions = append(conditions, "(title LIKE ? OR content LIKE ? OR keywords LIKE ?)")
			pattern := "%" + kw + "%"
			args = append(args, pattern, pattern, pattern)
		}

		if len(conditions) > 0 {
			db = db.Where(strings.Join(conditions, " OR "), args...)
		}

		err = db.Limit(limit * 2).Find(&chunks).Error
		if err != nil {
			return nil, fmt.Errorf("search failed: %v", err)
		}
	}

	// 计算相关性分数并排序
	results := make([]SearchResult, 0, len(chunks))
	for i := range chunks {
		score := calculateScore(query, &chunks[i])
		results = append(results, SearchResult{
			Chunk:    &chunks[i],
			Score:    score,
			Keywords: extractKeywords(chunks[i].Content),
			Source:   "keyword",
		})
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制返回数量
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// HybridSearch 混合检索（向量 + 关键词）
func (r *Retriever) HybridSearch(query string, category string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	// 并行执行两种检索
	vectorResults := make(chan []SearchResult, 1)
	keywordResults := make(chan []SearchResult, 1)
	vectorErr := make(chan error, 1)
	keywordErr := make(chan error, 1)

	// 向量检索
	go func() {
		results, err := r.VectorSearch(query, category, limit)
		vectorResults <- results
		vectorErr <- err
	}()

	// 关键词检索
	go func() {
		results, err := r.KeywordSearch(query, category, limit)
		keywordResults <- results
		keywordErr <- err
	}()

	// 收集结果
	vResults := <-vectorResults
	vErr := <-vectorErr
	kResults := <-keywordResults
	kErr := <-keywordErr

	// 如果两种检索都失败，返回错误
	if vErr != nil && kErr != nil {
		return nil, fmt.Errorf("both searches failed: vector=%v, keyword=%v", vErr, kErr)
	}

	// 融合结果
	return r.mergeResults(vResults, kResults, limit), nil
}

// mergeResults 融合向量检索和关键词检索结果
func (r *Retriever) mergeResults(vectorResults, keywordResults []SearchResult, limit int) []SearchResult {
	// 使用加权融合
	// 向量检索权重 0.6，关键词检索权重 0.4
	vectorWeight := 0.6
	keywordWeight := 0.4

	// 使用 map 去重并合并分数
	mergedMap := make(map[uint]*SearchResult)

	// 处理向量检索结果
	for _, result := range vectorResults {
		id := result.Chunk.ID
		if existing, ok := mergedMap[id]; ok {
			// 已存在，合并分数
			existing.Score = existing.Score*keywordWeight + result.Score*vectorWeight
			existing.Source = "hybrid"
		} else {
			result.Score = result.Score * vectorWeight
			result.Source = "vector"
			mergedMap[id] = &result
		}
	}

	// 处理关键词检索结果
	for _, result := range keywordResults {
		id := result.Chunk.ID
		if existing, ok := mergedMap[id]; ok {
			// 已存在，合并分数
			if existing.Source == "vector" {
				existing.Score = existing.Score + result.Score*keywordWeight
				existing.Source = "hybrid"
			}
		} else {
			result.Score = result.Score * keywordWeight
			result.Source = "keyword"
			mergedMap[id] = &result
		}
	}

	// 转换为切片并排序
	results := make([]SearchResult, 0, len(mergedMap))
	for _, result := range mergedMap {
		results = append(results, *result)
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制返回数量
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// extractKeywords 提取关键词（简单实现）
func extractKeywords(text string) []string {
	// 简单的关键词提取：分词后过滤停用词
	// TODO: 后续可以接入 jieba 分词
	words := strings.Fields(text)
	keywords := make([]string, 0)
	stopWords := map[string]bool{
		"的": true, "是": true, "在": true, "了": true, "和": true,
		"与": true, "或": true, "等": true, "也": true, "都": true,
		"而": true, "但": true, "却": true, "又": true, "就": true,
		"着": true, "过": true, "被": true, "把": true, "给": true,
		"这": true, "那": true, "有": true, "为": true, "以": true,
	}

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) >= 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// calculateScore 计算相关性分数（BM25 简化版）
func calculateScore(query string, chunk *models.KnowledgeChunk) float64 {
	queryTerms := strings.Fields(strings.ToLower(query))
	title := strings.ToLower(chunk.Title)
	content := strings.ToLower(chunk.Content)
	keywords := strings.ToLower(chunk.Keywords)

	score := 0.0
	for _, term := range queryTerms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}

		// 标题匹配权重最高
		if strings.Contains(title, term) {
			score += 3.0
		}

		// 关键词匹配
		if strings.Contains(keywords, term) {
			score += 2.0
		}

		// 内容匹配
		count := strings.Count(content, term)
		score += float64(count) * 0.5
	}

	return score
}

// BuildContext 构建上下文 Prompt
func BuildContext(results []SearchResult, maxLen int) string {
	if len(results) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n【参考资料】\n")

	totalLen := 0
	for i, result := range results {
		chunkText := fmt.Sprintf("\n%d. [%s] %s\n%s\n",
			i+1,
			result.Chunk.Category,
			result.Chunk.Title,
			result.Chunk.Content,
		)

		if totalLen+len(chunkText) > maxLen {
			break
		}

		builder.WriteString(chunkText)
		totalLen += len(chunkText)
	}

	return builder.String()
}

// FormatResults 格式化搜索结果为 JSON
func FormatResults(results []SearchResult) string {
	data := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		data = append(data, map[string]interface{}{
			"id":       r.Chunk.ID,
			"title":    r.Chunk.Title,
			"category": r.Chunk.Category,
			"content":  r.Chunk.Content,
			"score":    r.Score,
			"source":   r.Source,
		})
	}
	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}

// Close 关闭资源
func (r *Retriever) Close() error {
	if r.milvusClient != nil {
		r.milvusClient.Close()
	}
	if r.cacheSvc != nil {
		r.cacheSvc.Close()
	}
	return nil
}

// GetCacheStats 获取缓存统计
func (r *Retriever) GetCacheStats() cache.CacheStats {
	if r.cacheSvc != nil {
		return r.cacheSvc.GetStats()
	}
	return cache.CacheStats{}
}

// ClearCache 清除缓存
func (r *Retriever) ClearCache(pattern string) error {
	if r.cacheSvc != nil && r.cacheEnabled {
		ctx := context.Background()
		if pattern == "" {
			pattern = "search:*" // 默认清除搜索缓存
		}
		return r.cacheSvc.DeleteByPattern(ctx, pattern)
	}
	return nil
}

// UseVectorSearch 返回是否启用向量检索
func (r *Retriever) UseVectorSearch() bool {
	return r.useVectorSearch && r.milvusClient != nil && r.milvusClient.IsEnabled()
}

// IsCacheEnabled 返回是否启用缓存
func (r *Retriever) IsCacheEnabled() bool {
	return r.cacheEnabled
}
