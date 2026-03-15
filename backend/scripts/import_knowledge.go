package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// KnowledgeData 知识数据结构
type KnowledgeData struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	Period    string `json:"period"`
	YearStart int    `json:"year_start"`
	YearEnd   int    `json:"year_end"`
	Keywords  string `json:"keywords"`
	Source    string `json:"source"`
}

func main() {
	// 加载配置
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}
	config.InitConfig()

	// 初始化数据库
	if err := dao.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dao.CloseDB()

	// 知识库文件路径
	knowledgeDir := "./knowledge"
	files := []string{
		"states.json",
		"people.json",
		"events.json",
		"schools.json",
	}

	totalImported := 0

	for _, file := range files {
		filePath := filepath.Join(knowledgeDir, file)
		count, err := importKnowledgeFile(filePath)
		if err != nil {
			log.Printf("Error importing %s: %v", file, err)
			continue
		}
		totalImported += count
		log.Printf("Imported %d records from %s", count, file)
	}

	log.Printf("Total imported: %d knowledge chunks", totalImported)
}

func importKnowledgeFile(filePath string) (int, error) {
	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	// 解析 JSON
	var knowledgeList []KnowledgeData
	if err := json.Unmarshal(data, &knowledgeList); err != nil {
		return 0, err
	}

	// 清空现有数据（可选）
	// dao.Db.Where("1 = 1").Delete(&models.KnowledgeChunk{})

	// 导入数据
	count := 0
	for _, kd := range knowledgeList {
		chunk := models.KnowledgeChunk{
			Title:     kd.Title,
			Content:   kd.Content,
			Category:  kd.Category,
			Period:    kd.Period,
			YearStart: kd.YearStart,
			YearEnd:   kd.YearEnd,
			Keywords:  kd.Keywords,
			Source:    kd.Source,
		}

		// 检查是否已存在
		var existing models.KnowledgeChunk
		result := dao.Db.Where("title = ? AND category = ?", kd.Title, kd.Category).First(&existing)
		if result.Error == nil {
			// 已存在，更新
			dao.Db.Model(&existing).Updates(chunk)
			continue
		}

		// 不存在，创建
		if err := dao.Db.Create(&chunk).Error; err != nil {
			log.Printf("Error creating chunk %s: %v", kd.Title, err)
			continue
		}
		count++
	}

	return count, nil
}
