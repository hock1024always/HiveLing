package rag

import (
	"fmt"
	"time"

	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// ImportService 向量库导入服务
type ImportService struct {
	parser      *DocumentParser
	embedding   *EmbeddingService
	milvus      *MilvusClient
	retriever   *Retriever
}

// NewImportService 创建导入服务
func NewImportService() *ImportService {
	return &ImportService{
		parser:    NewDocumentParser(DocTypeGeneral),
		embedding: NewEmbeddingService(),
		milvus:    nil, // 可选
		retriever: NewRetriever(),
	}
}

// ImportDocument 导入单个文档
func (s *ImportService) ImportDocument(filePath string, metadata DocumentMetadata, docType DocumentType) error {
	// 1. 解析文档
	parser := NewDocumentParser(docType)
	chunks, err := parser.ParseFile(filePath, metadata)
	if err != nil {
		return fmt.Errorf("parse failed: %v", err)
	}

	fmt.Printf("Document parsed into %d chunks\n", len(chunks))

	// 2. 导入到 MySQL（基础存储）
	if err := s.importToMySQL(chunks); err != nil {
		return fmt.Errorf("mysql import failed: %v", err)
	}

	// 3. 导入到 Milvus（向量检索）
	if s.milvus != nil && s.milvus.IsEnabled() {
		if err := s.importToMilvus(chunks); err != nil {
			fmt.Printf("Warning: milvus import failed: %v\n", err)
			// 不影响主流程
		}
	}

	return nil
}

// importToMySQL 导入到 MySQL
func (s *ImportService) importToMySQL(chunks []TextChunk) error {
	for _, chunk := range chunks {
		knowledge := &models.KnowledgeChunk{
			Title:     chunk.Metadata.Title,
			Content:   chunk.Content,
			Category:  chunk.Metadata.Category,
			Keywords:  fmt.Sprintf("%s,%s", chunk.Metadata.Author, chunk.Metadata.Source),
			Source:    chunk.Metadata.Source,
			Period:    chunk.Metadata.Period,
			YearStart: 0, // 可以从元数据解析
			YearEnd:   0,
		}

		if err := dao.Db.Create(knowledge).Error; err != nil {
			return err
		}
	}
	return nil
}

// importToMilvus 导入到 Milvus
func (s *ImportService) importToMilvus(chunks []TextChunk) error {
	// 批量获取向量
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	embeddings, err := s.embedding.BatchGetEmbeddings(texts, 20)
	if err != nil {
		return err
	}

	// 构建 Milvus 实体
	var vectors []KnowledgeVector
	for i, chunk := range chunks {
		if i >= len(embeddings) {
			break
		}

		vector := KnowledgeVector{
			ID:        int64(chunk.Index), // 这里应该使用 MySQL 返回的 ID
			Vector:    embeddings[i],
			Title:     chunk.Metadata.Title,
			Content:   chunk.Content,
			Category:  chunk.Metadata.Category,
		}
		vectors = append(vectors, vector)
	}

	// 批量插入 Milvus (如果启用)
	if s.milvus != nil && s.milvus.enabled {
		return s.milvus.InsertVectors(vectors)
	}
	return nil
}

// ImportProgress 导入进度
type ImportProgress struct {
	TotalFiles    int     `json:"total_files"`
	ProcessedFiles int    `json:"processed_files"`
	TotalChunks   int     `json:"total_chunks"`
	CurrentFile   string  `json:"current_file"`
	Progress      float64 `json:"progress"`
}

// BatchImport 批量导入（带进度回调）
func (s *ImportService) BatchImport(dirPath string, docType DocumentType, progressChan chan<- ImportProgress) error {
	// 获取文件列表
	chunks, err := BatchParseDirectory(dirPath, docType)
	if err != nil {
		return err
	}

	// 分批导入
	batchSize := 50
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		if err := s.importToMySQL(batch); err != nil {
			return err
		}

		// 发送进度
		if progressChan != nil {
			progressChan <- ImportProgress{
				TotalChunks:    len(chunks),
				ProcessedFiles: end,
				Progress:       float64(end) / float64(len(chunks)) * 100,
			}
		}

		// 避免请求过快
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// CheckDuplicate 检查重复内容
func (s *ImportService) CheckDuplicate(content string, threshold float64) (bool, *models.KnowledgeChunk, error) {
	// 使用向量相似度检查
	if s.milvus != nil && s.milvus.IsEnabled() {
		embedding, err := s.embedding.GetEmbedding(content)
		if err != nil {
			return false, nil, err
		}

		results, err := s.milvus.Search(embedding, 1, "")
		if err != nil {
			return false, nil, err
		}

		if len(results) > 0 && float64(results[0].Score) > threshold {
			// 查询对应的 MySQL 记录
			var chunk models.KnowledgeChunk
			if err := dao.Db.First(&chunk, results[0].ID).Error; err == nil {
				return true, &chunk, nil
			}
		}
	}

	return false, nil, nil
}

// UpdateDocument 更新文档（删除旧内容，插入新内容）
func (s *ImportService) UpdateDocument(source string, newChunks []TextChunk) error {
	// 1. 删除旧内容
	if err := dao.Db.Where("source = ?", source).Delete(&models.KnowledgeChunk{}).Error; err != nil {
		return err
	}

	// 2. 插入新内容
	return s.importToMySQL(newChunks)
}

// MilvusEntity Milvus 实体
type MilvusEntity struct {
	ID       uint64
	Vector   []float32
	Title    string
	Content  string
	Category string
}
