package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/services/cache"
)

// EmbeddingService Embedding 服务
type EmbeddingService struct {
	apiKey     string
	baseURL    string
	model      string
	client     *http.Client
	cacheSvc   *cache.CacheService
	cacheEnabled bool
}

// NewEmbeddingService 创建 Embedding 服务
func NewEmbeddingService() *EmbeddingService {
	embeddingConfig := config.AppConfig.Embedding

	var apiKey, baseURL, model string

	switch embeddingConfig.Provider {
	case "deepseek":
		// DeepSeek 使用 OpenAI 兼容接口
		apiKey = config.AppConfig.DeepSeek.APIKey
		baseURL = config.AppConfig.DeepSeek.BaseURL
		model = "text-embedding-3-small" // DeepSeek 暂不支持 embedding，使用 OpenAI
	case "openai":
		apiKey = config.AppConfig.DeepSeek.APIKey // 复用 DeepSeek 的 key，实际应单独配置
		baseURL = "https://api.openai.com/v1"
		model = embeddingConfig.Model
	default:
		apiKey = config.AppConfig.DeepSeek.APIKey
		baseURL = "https://api.openai.com/v1"
		model = "text-embedding-3-small"
	}

	// 初始化缓存服务
	cacheSvc := cache.NewCacheService()

	return &EmbeddingService{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheSvc:     cacheSvc,
		cacheEnabled: cacheSvc.IsEnabled(),
	}
}

// EmbeddingRequest Embedding 请求
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse Embedding 响应
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// GetEmbedding 获取单个文本的向量（带缓存）
func (s *EmbeddingService) GetEmbedding(text string) ([]float32, error) {
	embeddings, err := s.GetEmbeddings([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// GetEmbeddings 批量获取文本向量（带缓存）
func (s *EmbeddingService) GetEmbeddings(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}

	ctx := context.Background()
	embeddings := make([][]float32, len(texts))
	needFetch := make([]int, 0) // 需要从 API 获取的索引

	// 尝试从缓存获取
	if s.cacheEnabled {
		for i, text := range texts {
			cacheKey := cache.EmbeddingCacheKey(text)
			var cached []float32
			if err := s.cacheSvc.Get(ctx, cacheKey, &cached); err == nil {
				embeddings[i] = cached
			} else {
				needFetch = append(needFetch, i)
			}
		}
	} else {
		// 缓存未启用，全部需要获取
		for i := range texts {
			needFetch = append(needFetch, i)
		}
	}

	// 如果所有结果都从缓存获取，直接返回
	if len(needFetch) == 0 {
		return embeddings, nil
	}

	// 批量获取未缓存的向量
	textsToFetch := make([]string, len(needFetch))
	for i, idx := range needFetch {
		textsToFetch[i] = texts[idx]
	}

	fetched, err := s.fetchEmbeddings(textsToFetch)
	if err != nil {
		return nil, err
	}

	// 填充结果并缓存
	for i, idx := range needFetch {
		if i < len(fetched) {
			embeddings[idx] = fetched[i]

			// 异步缓存
			if s.cacheEnabled {
				go func(text string, vec []float32) {
					bgCtx := context.Background()
					cacheKey := cache.EmbeddingCacheKey(text)
					s.cacheSvc.Set(bgCtx, cacheKey, vec, 24*time.Hour) // 缓存 24 小时
				}(texts[idx], fetched[i])
			}
		}
	}

	return embeddings, nil
}

// fetchEmbeddings 从 API 获取向量（内部方法）
func (s *EmbeddingService) fetchEmbeddings(texts []string) ([][]float32, error) {
	reqBody := EmbeddingRequest{
		Model: s.model,
		Input: texts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", s.baseURL+"/v1/embeddings", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", embeddingResp.Error.Message)
	}

	// 按索引排序结果
	embeddings := make([][]float32, len(texts))
	for _, data := range embeddingResp.Data {
		if data.Index < len(embeddings) {
			embeddings[data.Index] = data.Embedding
		}
	}

	return embeddings, nil
}

// GetEmbeddingWithRetry 带重试的获取向量
func (s *EmbeddingService) GetEmbeddingWithRetry(text string, maxRetries int) ([]float32, error) {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		embedding, err := s.GetEmbedding(text)
		if err == nil {
			return embedding, nil
		}
		lastErr = err

		// 如果是速率限制错误，等待后重试
		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

// BatchGetEmbeddings 批量获取向量（支持分批处理）
func (s *EmbeddingService) BatchGetEmbeddings(texts []string, batchSize int) ([][]float32, error) {
	if batchSize <= 0 {
		batchSize = 20 // 默认每批 20 个
	}

	allEmbeddings := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := s.GetEmbeddings(batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %v", i, end, err)
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

// CosineSimilarity 计算余弦相似度
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 float32 平方根
func sqrt32(x float32) float32 {
	if x < 0 {
		return 0
	}
	return float32(sqrt(float64(x)))
}

// sqrt float64 平方根（牛顿迭代法）
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
		if z*z-x < 1e-10 && z*z-x > -1e-10 {
			break
		}
	}
	return z
}
