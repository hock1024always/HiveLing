package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// DocumentType 文档类型
type DocumentType string

const (
	DocTypeHistorical DocumentType = "historical" // 史书类（史记、左传）
	DocTypeLecture    DocumentType = "lecture"    // 讲座类（百家讲坛）
	DocTypeArticle    DocumentType = "article"    // 论文/文章
	DocTypeGeneral    DocumentType = "general"    // 通用文本
)

// DocumentMetadata 文档元数据
type DocumentMetadata struct {
	Title       string            `json:"title"`       // 文档标题
	Author      string            `json:"author"`      // 作者
	Source      string            `json:"source"`      // 来源（史记/百家讲坛）
	Period      string            `json:"period"`      // 历史时期
	Category    string            `json:"category"`    // 分类
	PublishYear int               `json:"publish_year"`// 出版年份
	Tags        []string          `json:"tags"`        // 标签
	Extra       map[string]string `json:"extra"`       // 额外信息
}

// TextChunk 文本块
type TextChunk struct {
	Content   string           `json:"content"`    // 文本内容
	Metadata  DocumentMetadata `json:"metadata"`   // 元数据
	Index     int              `json:"index"`      // 块序号
	StartPos  int              `json:"start_pos"`  // 起始位置
	EndPos    int              `json:"end_pos"`    // 结束位置
	CharCount int              `json:"char_count"` // 字符数
}

// ChunkStrategy Chunk切分策略
type ChunkStrategy struct {
	MaxChunkSize    int  // 最大块大小（字符数）
	MinChunkSize    int  // 最小块大小
	OverlapSize     int  // 重叠大小（保持上下文连贯）
	RespectBoundary bool // 是否尊重段落/句子边界
}

// DefaultStrategies 默认策略配置
var DefaultStrategies = map[DocumentType]ChunkStrategy{
	DocTypeHistorical: {
		MaxChunkSize:    800,  // 史书类：中等粒度，保持事件完整性
		MinChunkSize:    200,
		OverlapSize:     100,
		RespectBoundary: true,
	},
	DocTypeLecture: {
		MaxChunkSize:    1200, // 讲座类：较大粒度，保持讲述连贯性
		MinChunkSize:    300,
		OverlapSize:     150,
		RespectBoundary: true,
	},
	DocTypeArticle: {
		MaxChunkSize:    600,  // 论文类：较小粒度，便于精准引用
		MinChunkSize:    150,
		OverlapSize:     80,
		RespectBoundary: true,
	},
	DocTypeGeneral: {
		MaxChunkSize:    500,
		MinChunkSize:    100,
		OverlapSize:     50,
		RespectBoundary: false,
	},
}

// DocumentParser 文档解析器
type DocumentParser struct {
	strategy ChunkStrategy
	docType  DocumentType
}

// NewDocumentParser 创建文档解析器
func NewDocumentParser(docType DocumentType) *DocumentParser {
	strategy, ok := DefaultStrategies[docType]
	if !ok {
		strategy = DefaultStrategies[DocTypeGeneral]
	}
	return &DocumentParser{
		strategy: strategy,
		docType:  docType,
	}
}

// SetStrategy 设置自定义策略
func (p *DocumentParser) SetStrategy(strategy ChunkStrategy) {
	p.strategy = strategy
}

// ParseFile 解析文件
func (p *DocumentParser) ParseFile(filePath string, metadata DocumentMetadata) ([]TextChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// 自动检测文档类型（如果未指定）
	if p.docType == "" {
		p.docType = DetectDocumentType(filePath, string(content))
		p.strategy = DefaultStrategies[p.docType]
	}

	// 根据文档类型预处理
	text := string(content)
	switch p.docType {
	case DocTypeHistorical:
		text = p.preprocessHistorical(text)
	case DocTypeLecture:
		text = p.preprocessLecture(text)
	}

	return p.ChunkText(text, metadata), nil
}

// DetectDocumentType 自动检测文档类型
func DetectDocumentType(filePath string, content string) DocumentType {
	ext := strings.ToLower(filepath.Ext(filePath))
	contentLower := strings.ToLower(content[:min(1000, len(content))])

	// 根据文件名和内容特征检测
	if strings.Contains(contentLower, "史记") || strings.Contains(contentLower, "左传") ||
		strings.Contains(contentLower, "春秋") || strings.Contains(contentLower, "战国策") {
		return DocTypeHistorical
	}

	if strings.Contains(contentLower, "百家讲坛") || strings.Contains(contentLower, "讲座") ||
		strings.Contains(contentLower, "主讲人") {
		return DocTypeLecture
	}

	if ext == ".md" || ext == ".markdown" {
		return DocTypeArticle
	}

	return DocTypeGeneral
}

// preprocessHistorical 史书类预处理
func (p *DocumentParser) preprocessHistorical(text string) string {
	// 1. 标准化标点
	replacements := map[string]string{
		"。": "。\n",
		"；": "；\n",
		"？": "？\n",
		"！": "！\n",
	}
	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}

	// 2. 识别并标记章节（如"卷一"、"本纪"等）
	chapterPattern := regexp.MustCompile(`(卷[一二三四五六七八九十百]+|本纪|世家|列传)`)
	text = chapterPattern.ReplaceAllString(text, "\n【章节】$1\n")

	// 3. 识别人名并标记（有助于后续实体链接）
	// 简单规则：两个字的词，在文本中多次出现
	// 实际项目中可以使用 jieba 分词 + NER

	return text
}

// preprocessLecture 讲座类预处理
func (p *DocumentParser) preprocessLecture(text string) string {
	// 1. 提取时间戳（如"00:23:15"或"第5分钟"）
	timePattern := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}|第\d+分钟)`)
	text = timePattern.ReplaceAllString(text, "\n【时间】$1\n")

	// 2. 提取主讲人信息
	speakerPattern := regexp.MustCompile(`(主讲人|讲师|主讲)：?\s*(\S+)`)
	text = speakerPattern.ReplaceAllString(text, "\n【主讲】$2\n")

	// 3. 提取主题/标题
	topicPattern := regexp.MustCompile(`(主题|标题|本期)：?\s*(.+)`)
	text = topicPattern.ReplaceAllString(text, "\n【主题】$2\n")

	return text
}

// ChunkText 切分文本
func (p *DocumentParser) ChunkText(text string, metadata DocumentMetadata) []TextChunk {
	var chunks []TextChunk
	strategy := p.strategy

	// 按段落分割
	paragraphs := p.splitIntoParagraphs(text)

	currentChunk := strings.Builder{}
	currentSize := 0
	chunkIndex := 0
	startPos := 0

	for i, para := range paragraphs {
		paraLen := utf8.RuneCountInString(para)

		// 如果当前段落本身超过最大限制，需要进一步切分
		if paraLen > strategy.MaxChunkSize {
			// 先保存当前积累的chunk
			if currentChunk.Len() > 0 {
				chunk := p.createChunk(currentChunk.String(), metadata, chunkIndex, startPos, startPos+currentSize)
				chunks = append(chunks, chunk)
				chunkIndex++
				currentChunk.Reset()
				currentSize = 0
			}

			// 切分长段落
			subChunks := p.splitLongParagraph(para, strategy)
			for _, subChunk := range subChunks {
				chunk := p.createChunk(subChunk, metadata, chunkIndex, startPos, startPos+utf8.RuneCountInString(subChunk))
				chunks = append(chunks, chunk)
				chunkIndex++
				startPos += utf8.RuneCountInString(subChunk)
			}
			continue
		}

		// 检查加入当前段落后是否超过限制
		if currentSize+paraLen > strategy.MaxChunkSize && currentChunk.Len() > 0 {
			// 保存当前chunk
			chunk := p.createChunk(currentChunk.String(), metadata, chunkIndex, startPos, startPos+currentSize)
			chunks = append(chunks, chunk)

			// 处理重叠（保留最后一部分内容到下一个chunk）
			overlapText := ""
			if strategy.OverlapSize > 0 && currentChunk.Len() > strategy.OverlapSize {
				overlapText = p.getOverlapText(currentChunk.String(), strategy.OverlapSize)
			}

			chunkIndex++
			startPos += currentSize

			// 开始新chunk，包含重叠内容
			currentChunk.Reset()
			if overlapText != "" {
				currentChunk.WriteString(overlapText)
				currentSize = utf8.RuneCountInString(overlapText)
			} else {
				currentSize = 0
			}
		}

		// 添加当前段落
		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n")
		}
		currentChunk.WriteString(para)
		currentSize += paraLen

		// 如果是最后一个段落，保存chunk
		if i == len(paragraphs)-1 && currentChunk.Len() > 0 {
			chunk := p.createChunk(currentChunk.String(), metadata, chunkIndex, startPos, startPos+currentSize)
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// splitIntoParagraphs 分割为段落
func (p *DocumentParser) splitIntoParagraphs(text string) []string {
	// 按换行分割，并过滤空行
	lines := strings.Split(text, "\n")
	var paragraphs []string
	var currentPara strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentPara.Len() > 0 {
				paragraphs = append(paragraphs, currentPara.String())
				currentPara.Reset()
			}
		} else {
			if currentPara.Len() > 0 {
				currentPara.WriteString(" ")
			}
			currentPara.WriteString(line)
		}
	}

	if currentPara.Len() > 0 {
		paragraphs = append(paragraphs, currentPara.String())
	}

	return paragraphs
}

// splitLongParagraph 切分长段落（按句子边界）
func (p *DocumentParser) splitLongParagraph(paragraph string, strategy ChunkStrategy) []string {
	var chunks []string

	// 按句子切分（简单实现：按句号、问号、感叹号）
	sentencePattern := regexp.MustCompile(`([。！？]+)`)
	sentences := sentencePattern.Split(paragraph, -1)

	currentChunk := strings.Builder{}
	currentSize := 0

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}

		sentenceLen := utf8.RuneCountInString(sentence)

		// 单句超过限制，强制切分
		if sentenceLen > strategy.MaxChunkSize {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
				currentChunk.Reset()
				currentSize = 0
			}
			// 按固定大小切分
			for i := 0; i < sentenceLen; i += strategy.MaxChunkSize {
				end := i + strategy.MaxChunkSize
				if end > sentenceLen {
					end = sentenceLen
				}
				chunk := string([]rune(sentence)[i:end])
				chunks = append(chunks, chunk)
			}
			continue
		}

		// 检查是否超过限制
		if currentSize+sentenceLen > strategy.MaxChunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentSize = 0
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("，")
		}
		currentChunk.WriteString(sentence)
		currentSize += sentenceLen
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// getOverlapText 获取重叠文本
func (p *DocumentParser) getOverlapText(text string, overlapSize int) string {
	runes := []rune(text)
	if len(runes) <= overlapSize {
		return text
	}
	return string(runes[len(runes)-overlapSize:])
}

// createChunk 创建文本块
func (p *DocumentParser) createChunk(content string, metadata DocumentMetadata, index, startPos, endPos int) TextChunk {
	return TextChunk{
		Content:   content,
		Metadata:  metadata,
		Index:     index,
		StartPos:  startPos,
		EndPos:    endPos,
		CharCount: utf8.RuneCountInString(content),
	}
}

// ExtractMetadataFromFilename 从文件名提取元数据
func ExtractMetadataFromFilename(filename string) DocumentMetadata {
	metadata := DocumentMetadata{
		Source: filename,
		Extra:  make(map[string]string),
	}

	// 移除扩展名
	base := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 尝试提取信息（格式：作者_标题_时期.txt）
	parts := strings.Split(base, "_")
	if len(parts) >= 1 {
		metadata.Author = parts[0]
	}
	if len(parts) >= 2 {
		metadata.Title = parts[1]
	}
	if len(parts) >= 3 {
		metadata.Period = parts[2]
	}

	return metadata
}

// BatchParseDirectory 批量解析目录
func BatchParseDirectory(dirPath string, docType DocumentType) ([]TextChunk, error) {
	var allChunks []TextChunk

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	parser := NewDocumentParser(docType)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理文本文件
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".txt" && ext != ".md" {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		metadata := ExtractMetadataFromFilename(entry.Name())

		chunks, err := parser.ParseFile(filePath, metadata)
		if err != nil {
			fmt.Printf("Warning: failed to parse %s: %v\n", entry.Name(), err)
			continue
		}

		allChunks = append(allChunks, chunks...)
		fmt.Printf("Parsed %s: %d chunks\n", entry.Name(), len(chunks))
	}

	return allChunks, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
