package main

import (
	"log"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/router"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// 初始化配置
	config.InitConfig()

	// 初始化数据库
	if err := dao.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dao.CloseDB()

	// 启动路由
	r := router.Router()
	port := config.AppConfig.Server.Port
	log.Printf("Server starting on port %d...", port)
	r.Run(":9090")
}
