package cors

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	cfg := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return true //允许所有前端访问
		},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true, // 允许发送Cookie
	}
	return cors.New(cfg)
}
