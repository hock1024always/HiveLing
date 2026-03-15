package dao

import (
	"fmt"
	"log"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	var err error

	// 配置 GORM 日志
	gormLogger := logger.Default.LogMode(logger.Info)

	// 连接数据库
	dsn := config.AppConfig.MySQL.GetDSN()
	Db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect database: %v", err)
	}

	// 获取底层的 sql.DB
	sqlDB, err := Db.DB()
	if err != nil {
		return fmt.Errorf("failed to get DB instance: %v", err)
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移表结构
	err = autoMigrate()
	if err != nil {
		return fmt.Errorf("failed to migrate tables: %v", err)
	}

	log.Println("Database connected and migrated successfully")
	return nil
}

// autoMigrate 自动迁移表结构
func autoMigrate() error {
	return Db.AutoMigrate(
		// 用户相关
		&models.User{},
		// 会话和消息
		&models.Session{},
		&models.Message{},
		// 知识库
		&models.KnowledgeChunk{},
		// 知识图谱
		&models.KGNode{},
		&models.KGEdge{},
		// 备课工作流
		&models.LessonWorkflow{},
		&models.WorkflowMessage{},
		// 数字人视频生成
		&models.DigitalHumanVideo{},
		&models.DigitalHumanAvatar{},
	)
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	sqlDB, err := Db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
