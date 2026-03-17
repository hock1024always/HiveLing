package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/rag"
	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env 文件
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// 解析命令行参数
	batchSize := flag.Int("batch", 20, "batch size for embedding")
	dropExisting := flag.Bool("drop", false, "drop existing collection before import")
	dryRun := flag.Bool("dry-run", false, "dry run without inserting to Milvus")
	flag.Parse()

	// 初始化配置
	config.InitConfig()

	// 检查 Milvus 是否启用
	if !config.AppConfig.Milvus.Enabled {
		log.Fatal("Milvus is not enabled. Set MILVUS_ENABLED=true in .env")
	}

	// 初始化数据库
	if err := dao.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dao.CloseDB()

	log.Println("Starting vector import...")
	startTime := time.Now()

	// 创建 Milvus 客户端
	milvusClient, err := rag.NewMilvusClient()
	if err != nil {
		log.Fatalf("Failed to create Milvus client: %v", err)
	}
	defer milvusClient.Close()

	if !milvusClient.IsEnabled() {
		log.Fatal("Milvus client is not enabled")
	}

	// 如果指定了 drop，删除现有集合
	if *dropExisting {
		log.Println("Dropping existing collection...")
		if err := milvusClient.DropCollection(); err != nil {
			log.Printf("Warning: failed to drop collection: %v", err)
		}
	}

	// 创建集合
	log.Println("Creating collection...")
	if err := milvusClient.CreateCollection(); err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}

	// 获取所有知识块
	var chunks []models.KnowledgeChunk
	if err := dao.Db.Find(&chunks).Error; err != nil {
		log.Fatalf("Failed to fetch knowledge chunks: %v", err)
	}

	log.Printf("Found %d knowledge chunks to process", len(chunks))

	if len(chunks) == 0 {
		log.Println("No knowledge chunks found. Exiting.")
		return
	}

	// 创建 Embedding 服务
	embeddingSvc := rag.NewEmbeddingService()

	// 批量处理
	totalBatches := (len(chunks) + *batchSize - 1) / *batchSize
	successCount := 0
	errorCount := 0

	for i := 0; i < len(chunks); i += *batchSize {
		end := i + *batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		batchNum := i / *batchSize + 1

		log.Printf("Processing batch %d/%d (%d items)...", batchNum, totalBatches, len(batch))

		// 准备文本
		texts := make([]string, len(batch))
		for j, chunk := range batch {
			// 组合标题和内容作为嵌入文本
			texts[j] = fmt.Sprintf("%s\n%s", chunk.Title, chunk.Content)
		}

		// 获取向量
		embeddings, err := embeddingSvc.BatchGetEmbeddings(texts, *batchSize)
		if err != nil {
			log.Printf("Error getting embeddings for batch %d: %v", batchNum, err)
			errorCount += len(batch)
			continue
		}

		// 准备向量数据
		vectors := make([]rag.KnowledgeVector, len(batch))
		for j, chunk := range batch {
			if j < len(embeddings) {
				vectors[j] = rag.KnowledgeVector{
					ID:       int64(chunk.ID),
					Title:    chunk.Title,
					Content:  chunk.Content,
					Category: chunk.Category,
					Vector:   embeddings[j],
				}
			}
		}

		// 插入到 Milvus
		if !*dryRun {
			if err := milvusClient.InsertVectors(vectors); err != nil {
				log.Printf("Error inserting batch %d: %v", batchNum, err)
				errorCount += len(batch)
				continue
			}
		}

		successCount += len(batch)
		log.Printf("Batch %d completed. Progress: %d/%d", batchNum, end, len(chunks))

		// 避免速率限制
		if batchNum < totalBatches {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 获取统计信息
	stats, err := milvusClient.GetCollectionStats()
	if err != nil {
		log.Printf("Warning: failed to get stats: %v", err)
	}

	duration := time.Since(startTime)

	// 打印结果
	fmt.Println("\n========== Import Summary ==========")
	fmt.Printf("Total chunks:    %d\n", len(chunks))
	fmt.Printf("Success:         %d\n", successCount)
	fmt.Printf("Errors:          %d\n", errorCount)
	fmt.Printf("Duration:        %v\n", duration)
	fmt.Printf("Dry run:         %v\n", *dryRun)
	fmt.Println("====================================")

	if stats != nil {
		fmt.Println("\nCollection Stats:")
		for k, v := range stats {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}

	if errorCount > 0 {
		os.Exit(1)
	}
}
