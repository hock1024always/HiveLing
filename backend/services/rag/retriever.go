package rag

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"gorm.io/gorm"
)

// SearchResult 搜索结果
type SearchResult struct {
	Chunk    *models.KnowledgeChunk `json:"chunk"`
	Score    float64                `json:"score"`
	Keywords []string               `json:"keywords"`
}

// Retriever 检索器
type Retriever struct{}

// NewRetriever 创建检索器
func NewRetriever() *Retriever {
	return &Retriever{}
}

// Search 搜索知识库
func (r *Retriever) Search(query string, category string, limit int) ([]SearchResult, error) {
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
		})
	}
	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}
