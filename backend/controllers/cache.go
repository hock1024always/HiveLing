package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/services/cache"
	"github.com/hock1024always/GoEdu/services/rag"
)

// CacheController 缓存控制器
type CacheController struct {
	cacheSvc *cache.CacheService
	retriever *rag.Retriever
}

// NewCacheController 创建缓存控制器
func NewCacheController() *CacheController {
	return &CacheController{
		cacheSvc:  cache.NewCacheService(),
		retriever: rag.NewRetriever(),
	}
}

// GetStats 获取缓存统计
// GET /api/cache/stats
func (c *CacheController) GetStats(ctx *gin.Context) {
	stats := c.retriever.GetCacheStats()

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hits":       stats.Hits,
			"misses":     stats.Misses,
			"sets":       stats.Sets,
			"deletes":    stats.Deletes,
			"errors":     stats.Errors,
			"hit_rate":   stats.HitRate,
			"enabled":    c.cacheSvc.IsEnabled(),
		},
	})
}

// ClearCache 清除缓存
// DELETE /api/cache
func (c *CacheController) ClearCache(ctx *gin.Context) {
	pattern := ctx.Query("pattern")
	if pattern == "" {
		pattern = "search:*" // 默认清除搜索缓存
	}

	if err := c.retriever.ClearCache(pattern); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache cleared successfully",
		"pattern": pattern,
	})
}

// ResetStats 重置缓存统计
// POST /api/cache/stats/reset
func (c *CacheController) ResetStats(ctx *gin.Context) {
	c.cacheSvc.ResetStats()

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Cache stats reset successfully",
	})
}

// CacheStatus 缓存状态详情
// GET /api/cache/status
func (c *CacheController) CacheStatus(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"redis_enabled":     c.cacheSvc.IsEnabled(),
			"vector_enabled":    c.retriever.UseVectorSearch(),
			"cache_enabled":     c.retriever.IsCacheEnabled(),
			"cache_description": gin.H{
				"embedding_cache": "缓存文本向量，有效期 24 小时",
				"search_cache":    "缓存检索结果，有效期 10 分钟",
			},
		},
	})
}
