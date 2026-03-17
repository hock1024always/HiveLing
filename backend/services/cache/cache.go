package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hock1024always/GoEdu/config"
)

// CacheService 缓存服务
type CacheService struct {
	client *redis.Client
	enabled bool
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Sets       int64 `json:"sets"`
	Deletes    int64 `json:"deletes"`
	Errors     int64 `json:"errors"`
	HitRate    float64 `json:"hit_rate"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled           bool
	DefaultExpiration time.Duration
}

var (
	// 全局缓存统计
	stats CacheStats
)

// NewCacheService 创建缓存服务
func NewCacheService() *CacheService {
	redisConfig := config.AppConfig.Redis

	if redisConfig.Address == "" {
		return &CacheService{enabled: false}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisConfig.Address,
		Password: redisConfig.Password,
		DB:       0,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("Warning: Redis connection failed: %v, caching disabled\n", err)
		return &CacheService{enabled: false}
	}

	return &CacheService{
		client:  client,
		enabled: true,
	}
}

// IsEnabled 检查缓存是否启用
func (c *CacheService) IsEnabled() bool {
	return c.enabled
}

// Set 设置缓存
func (c *CacheService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		stats.Errors++
		return fmt.Errorf("failed to marshal value: %v", err)
	}

	if expiration == 0 {
		expiration = 30 * time.Minute // 默认 30 分钟
	}

	if err := c.client.Set(ctx, key, data, expiration).Err(); err != nil {
		stats.Errors++
		return fmt.Errorf("failed to set cache: %v", err)
	}

	stats.Sets++
	return nil
}

// Get 获取缓存
func (c *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
	if !c.enabled {
		stats.Misses++
		return fmt.Errorf("cache disabled")
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			stats.Misses++
		} else {
			stats.Errors++
		}
		return fmt.Errorf("cache miss: %v", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		stats.Errors++
		return fmt.Errorf("failed to unmarshal value: %v", err)
	}

	stats.Hits++
	return nil
}

// Delete 删除缓存
func (c *CacheService) Delete(ctx context.Context, key string) error {
	if !c.enabled {
		return nil
	}

	if err := c.client.Del(ctx, key).Err(); err != nil {
		stats.Errors++
		return fmt.Errorf("failed to delete cache: %v", err)
	}

	stats.Deletes++
	return nil
}

// DeleteByPattern 按模式删除缓存
func (c *CacheService) DeleteByPattern(ctx context.Context, pattern string) error {
	if !c.enabled {
		return nil
	}

	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			stats.Errors++
		} else {
			stats.Deletes++
		}
	}

	return iter.Err()
}

// Exists 检查缓存是否存在
func (c *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	if !c.enabled {
		return false, nil
	}

	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetOrSet 获取缓存，不存在则执行 fn 并缓存结果
func (c *CacheService) GetOrSet(ctx context.Context, key string, dest interface{}, fn func() (interface{}, error), expiration time.Duration) error {
	// 先尝试获取
	err := c.Get(ctx, key, dest)
	if err == nil {
		// 缓存命中
		return nil
	}

	// 缓存未命中，执行函数获取数据
	data, err := fn()
	if err != nil {
		return err
	}

	// 将结果写入 dest
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(dataBytes, dest); err != nil {
		return err
	}

	// 缓存结果
	go func() {
		bgCtx := context.Background()
		c.Set(bgCtx, key, data, expiration)
	}()

	return nil
}

// GetStats 获取缓存统计
func (c *CacheService) GetStats() CacheStats {
	stats.HitRate = 0
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses) * 100
	}
	return stats
}

// ResetStats 重置统计
func (c *CacheService) ResetStats() {
	stats = CacheStats{}
}

// GetContext 获取上下文
func (c *CacheService) GetContext() context.Context {
	return context.Background()
}

// Close 关闭连接
func (c *CacheService) Close() error {
	if !c.enabled || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// 缓存键生成器

// EmbeddingCacheKey 生成 Embedding 缓存键
func EmbeddingCacheKey(text string) string {
	return fmt.Sprintf("embedding:%x", hashString(text))
}

// SearchCacheKey 生成搜索缓存键
func SearchCacheKey(query, category string, limit int, useVector bool) string {
	return fmt.Sprintf("search:%s:%s:%d:%v", hashString(query), category, limit, useVector)
}

// SessionCacheKey 生成会话缓存键
func SessionCacheKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// hashString 简单的字符串哈希
func hashString(s string) string {
	// 使用简单的哈希算法
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%x", h)
}
