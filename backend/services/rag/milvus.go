package rag

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusClient Milvus 客户端封装
type MilvusClient struct {
	client     client.Client
	collection string
	dimension  int
	enabled    bool
}

// KnowledgeVector 知识向量
type KnowledgeVector struct {
	ID        int64     // 知识块 ID
	Title     string    // 标题
	Content   string    // 内容
	Category  string    // 分类
	Vector    []float32 // 向量
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	ID       int64
	Title    string
	Content  string
	Category string
	Score    float32 // 相似度分数
}

// NewMilvusClient 创建 Milvus 客户端
func NewMilvusClient() (*MilvusClient, error) {
	milvusConfig := config.AppConfig.Milvus

	if !milvusConfig.Enabled {
		return &MilvusClient{
			enabled: false,
		}, nil
	}

	// 连接 Milvus
	addr := fmt.Sprintf("%s:%d", milvusConfig.Host, milvusConfig.Port)
	c, err := client.NewClient(context.Background(), client.Config{
		Address: addr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %v", err)
	}

	return &MilvusClient{
		client:     c,
		collection: milvusConfig.Collection,
		dimension:  milvusConfig.Dimension,
		enabled:    true,
	}, nil
}

// IsEnabled 检查是否启用
func (m *MilvusClient) IsEnabled() bool {
	return m.enabled
}

// CreateCollection 创建集合
func (m *MilvusClient) CreateCollection() error {
	if !m.enabled {
		return fmt.Errorf("milvus not enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查集合是否已存在
	has, err := m.client.HasCollection(ctx, m.collection)
	if err != nil {
		return fmt.Errorf("failed to check collection: %v", err)
	}
	if has {
		return nil // 集合已存在
	}

	// 定义字段
	schema := &entity.Schema{
		CollectionName: m.collection,
		Description:    "Knowledge base vectors for RAG",
		AutoID:         false,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeInt64,
				PrimaryKey: true,
				AutoID:     false,
			},
			{
				Name:     "title",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "500",
				},
			},
			{
				Name:     "content",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "8000",
				},
			},
			{
				Name:     "category",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "50",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", m.dimension),
				},
			},
		},
	}

	// 创建集合
	if err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("failed to create collection: %v", err)
	}

	// 创建向量索引
	idx, err := entity.NewIndexAUTOINDEX(entity.L2)
	if err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	if err := m.client.CreateIndex(ctx, m.collection, "vector", idx, false); err != nil {
		return fmt.Errorf("failed to create vector index: %v", err)
	}

	log.Printf("Milvus collection '%s' created successfully", m.collection)
	return nil
}

// DropCollection 删除集合
func (m *MilvusClient) DropCollection() error {
	if !m.enabled {
		return fmt.Errorf("milvus not enabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return m.client.DropCollection(ctx, m.collection)
}

// InsertVectors 插入向量
func (m *MilvusClient) InsertVectors(vectors []KnowledgeVector) error {
	if !m.enabled {
		return fmt.Errorf("milvus not enabled")
	}

	if len(vectors) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 准备数据
	ids := make([]int64, len(vectors))
	titles := make([]string, len(vectors))
	contents := make([]string, len(vectors))
	categories := make([]string, len(vectors))
	vecs := make([][]float32, len(vectors))

	for i, v := range vectors {
		ids[i] = v.ID
		titles[i] = truncateString(v.Title, 500)
		contents[i] = truncateString(v.Content, 8000)
		categories[i] = v.Category
		vecs[i] = v.Vector
	}

	// 插入数据
	columns := []entity.Column{
		entity.NewColumnInt64("id", ids),
		entity.NewColumnVarChar("title", titles),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnVarChar("category", categories),
		entity.NewColumnFloatVector("vector", m.dimension, vecs),
	}

	if _, err := m.client.Insert(ctx, m.collection, "", columns...); err != nil {
		return fmt.Errorf("failed to insert vectors: %v", err)
	}

	// 刷新以确保数据可见
	if err := m.client.Flush(ctx, m.collection, false); err != nil {
		log.Printf("Warning: failed to flush collection: %v", err)
	}

	return nil
}

// Search 向量搜索
func (m *MilvusClient) Search(queryVector []float32, topK int, category string) ([]VectorSearchResult, error) {
	if !m.enabled {
		return nil, fmt.Errorf("milvus not enabled")
	}

	if topK <= 0 {
		topK = 10
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 加载集合到内存
	if err := m.client.LoadCollection(ctx, m.collection, false); err != nil {
		return nil, fmt.Errorf("failed to load collection: %v", err)
	}

	// 构建搜索参数
	searchParams, err := entity.NewIndexAUTOINDEXSearchParam(32)
	if err != nil {
		return nil, fmt.Errorf("failed to create search params: %v", err)
	}

	// 执行搜索
	var results []client.SearchResult
	if category != "" && category != "all" {
		// 带过滤条件的搜索
		expr := fmt.Sprintf("category == \"%s\"", category)
		results, err = m.client.Search(
			ctx, m.collection, []string{}, expr,
			[]string{"id", "title", "content", "category"},
			[]entity.Vector{entity.FloatVector(queryVector)},
			"vector",
			entity.L2,
			topK,
			searchParams,
		)
	} else {
		results, err = m.client.Search(
			ctx, m.collection, []string{}, "",
			[]string{"id", "title", "content", "category"},
			[]entity.Vector{entity.FloatVector(queryVector)},
			"vector",
			entity.L2,
			topK,
			searchParams,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %v", err)
	}

	// 解析结果
	searchResults := make([]VectorSearchResult, 0)
	for _, result := range results {
		for i := 0; i < result.ResultCount; i++ {
			id, err := result.IDs.GetAsInt64(i)
			if err != nil {
				continue
			}

			// 获取分数（L2 距离，需要转换为相似度）
			score := float32(0)
			if result.Scores != nil && i < len(result.Scores) {
				// L2 距离转换为相似度：相似度 = 1 / (1 + 距离)
				score = 1.0 / (1.0 + result.Scores[i])
			}

			// 获取其他字段
			title := ""
			content := ""
			categoryVal := ""

			for _, field := range result.Fields {
				switch field.Name() {
				case "title":
					if col, ok := field.(*entity.ColumnVarChar); ok {
						if val, err := col.ValueByIdx(i); err == nil {
							title = val
						}
					}
				case "content":
					if col, ok := field.(*entity.ColumnVarChar); ok {
						if val, err := col.ValueByIdx(i); err == nil {
							content = val
						}
					}
				case "category":
					if col, ok := field.(*entity.ColumnVarChar); ok {
						if val, err := col.ValueByIdx(i); err == nil {
							categoryVal = val
						}
					}
				}
			}

			searchResults = append(searchResults, VectorSearchResult{
				ID:       id,
				Title:    title,
				Content:  content,
				Category: categoryVal,
				Score:    score,
			})
		}
	}

	return searchResults, nil
}

// DeleteByID 按 ID 删除向量
func (m *MilvusClient) DeleteByID(ids []int64) error {
	if !m.enabled {
		return fmt.Errorf("milvus not enabled")
	}

	if len(ids) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expr := fmt.Sprintf("id in %v", ids)
	return m.client.Delete(ctx, m.collection, "", expr)
}

// Close 关闭连接
func (m *MilvusClient) Close() error {
	if !m.enabled || m.client == nil {
		return nil
	}
	return m.client.Close()
}

// GetCollectionStats 获取集合统计信息
func (m *MilvusClient) GetCollectionStats() (map[string]interface{}, error) {
	if !m.enabled {
		return map[string]interface{}{"enabled": false}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats, err := m.client.GetCollectionStatistics(ctx, m.collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %v", err)
	}

	result := map[string]interface{}{
		"enabled":    true,
		"collection": m.collection,
		"dimension":  m.dimension,
	}

	for key, value := range stats {
		result[key] = value
	}

	return result, nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
