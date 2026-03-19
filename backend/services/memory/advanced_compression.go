package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
)

// ContextNode 上下文节点（树形结构）
type ContextNode struct {
	ID           string          `json:"id"`
	Type         NodeType        `json:"type"`          // Message, Summary, Topic
	Content      string          `json:"content"`
	Messages     []models.Message `json:"messages,omitempty"` // 如果是 Summary，包含被摘要的消息
	Children     []*ContextNode  `json:"children,omitempty"` // 子节点
	Parent       *ContextNode    `json:"-"`                // 父节点（不序列化）
	Level        int             `json:"level"`           // 层级（0=原始消息，1=一级摘要，2=二级摘要...）
	Importance   float64         `json:"importance"`      // 重要性评分 0-10
	TokenCount   int             `json:"token_count"`     // Token 数量（估算）
	CreatedAt    time.Time       `json:"created_at"`
	AccessedAt   time.Time       `json:"accessed_at"`     // 最后访问时间（用于 LRU）
	AccessCount  int             `json:"access_count"`    // 访问次数
}

// NodeType 节点类型
type NodeType string

const (
	NodeTypeMessage NodeType = "message" // 原始消息
	NodeTypeSummary NodeType = "summary" // 摘要节点
	NodeTypeTopic   NodeType = "topic"   // 主题节点
)

// HierarchicalMemory 分层记忆系统
type HierarchicalMemory struct {
	root         *ContextNode
	messageIndex map[string]*ContextNode // 消息ID索引
	llmClient    *llm.Client
	maxTokens    int                     // 最大 Token 限制
	currentTokens int                    // 当前 Token 数
}

// NewHierarchicalMemory 创建分层记忆
func NewHierarchicalMemory(maxTokens int) *HierarchicalMemory {
	return &HierarchicalMemory{
		root: &ContextNode{
			ID:       "root",
			Type:     NodeTypeTopic,
			Content:  "对话根节点",
			Children: make([]*ContextNode, 0),
			Level:    -1,
		},
		messageIndex:  make(map[string]*ContextNode),
		llmClient:     llm.NewClient(),
		maxTokens:     maxTokens,
		currentTokens: 0,
	}
}

// AddMessage 添加消息
func (hm *HierarchicalMemory) AddMessage(msg models.Message) *ContextNode {
	node := &ContextNode{
		ID:          fmt.Sprintf("msg_%d", msg.ID),
		Type:        NodeTypeMessage,
		Content:     msg.Content,
		Level:       0,
		Importance:  hm.calculateImportance(msg),
		TokenCount:  estimateTokens(msg.Content),
		CreatedAt:   msg.CreatedAt,
		AccessedAt:  time.Now(),
		AccessCount: 1,
	}

	// 添加到根节点
	hm.root.Children = append(hm.root.Children, node)
	node.Parent = hm.root
	hm.messageIndex[node.ID] = node
	hm.currentTokens += node.TokenCount

	// 检查是否需要压缩
	if hm.currentTokens > hm.maxTokens {
		hm.compress()
	}

	return node
}

// calculateImportance 计算消息重要性（多维度）
func (hm *HierarchicalMemory) calculateImportance(msg models.Message) float64 {
	score := 5.0 // 基础分

	// 1. 标签权重
	tagWeights := map[string]float64{
		models.TagQuestion:      8.0,
		models.TagAnswer:        7.0,
		models.TagToolCall:      6.0,
		models.TagToolResult:    6.0,
		models.TagFact:          5.0,
		models.TagFollowUp:      4.0,
		models.TagClarification: 3.0,
		models.TagOpinion:       3.0,
		models.TagGreeting:      1.0,
		models.TagThanks:        1.0,
	}
	if weight, ok := tagWeights[msg.Tag]; ok {
		score = weight
	}

	// 2. 内容长度（适中为佳，太短可能是问候，太长可能是啰嗦）
	contentLen := len([]rune(msg.Content))
	if contentLen < 10 {
		score *= 0.5
	} else if contentLen > 500 {
		score *= 0.9
	} else if contentLen > 100 && contentLen < 300 {
		score *= 1.1 // 理想长度加分
	}

	// 3. 包含关键实体（人名、地名、事件名）
	if msg.Entities != "" {
		var entities []string
		json.Unmarshal([]byte(msg.Entities), &entities)
		score += float64(len(entities)) * 0.5
	}

	// 4. 用户明确标记的重要消息（可以扩展 Message 模型）
	if msg.Importance > 0 {
		score = float64(msg.Importance)
	}

	// 限制在 0-10 范围
	if score > 10 {
		score = 10
	}
	if score < 1 {
		score = 1
	}

	return score
}

// compress 执行压缩
func (hm *HierarchicalMemory) compress() {
	// 策略：从最低层级开始，将相邻的低重要性消息合并为摘要

	// 1. 找到需要压缩的层级
	level := hm.findCompressibleLevel()
	if level < 0 {
		return
	}

	// 2. 收集该层级的节点
	nodes := hm.getNodesAtLevel(level)
	if len(nodes) < 3 {
		// 节点太少，尝试压缩上一层
		if level > 0 {
			hm.compressLevel(level - 1)
		}
		return
	}

	// 3. 按重要性排序（保留高重要性的）
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Importance > nodes[j].Importance
	})

	// 4. 保留前 30% 的高重要性消息，其余进行摘要
	keepCount := len(nodes) / 3
	if keepCount < 2 {
		keepCount = 2
	}

	toSummarize := nodes[keepCount:]
	if len(toSummarize) < 2 {
		return
	}

	// 5. 生成摘要节点
	summaryNode := hm.createSummary(toSummarize, level+1)

	// 6. 替换原始节点
	hm.replaceWithSummary(toSummarize, summaryNode)

	// 7. 更新 Token 计数
	hm.recalculateTokens()
}

// findCompressibleLevel 找到可压缩的层级
func (hm *HierarchicalMemory) findCompressibleLevel() int {
	// 从底层向上找，找到节点数最多的层级
	levelCounts := make(map[int]int)
	var maxCount int
	var maxLevel int

	var count func(node *ContextNode)
	count = func(node *ContextNode) {
		if node.Type == NodeTypeMessage {
			levelCounts[node.Level]++
			if levelCounts[node.Level] > maxCount {
				maxCount = levelCounts[node.Level]
				maxLevel = node.Level
			}
		}
		for _, child := range node.Children {
			count(child)
		}
	}
	count(hm.root)

	if maxCount >= 5 {
		return maxLevel
	}
	return -1
}

// getNodesAtLevel 获取指定层级的所有节点
func (hm *HierarchicalMemory) getNodesAtLevel(level int) []*ContextNode {
	var nodes []*ContextNode
	var collect func(node *ContextNode)
	collect = func(node *ContextNode) {
		if node.Level == level && node.Type == NodeTypeMessage {
			nodes = append(nodes, node)
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(hm.root)
	return nodes
}

// createSummary 创建摘要节点
func (hm *HierarchicalMemory) createSummary(nodes []*ContextNode, level int) *ContextNode {
	// 收集消息内容
	var contents []string
	var messages []models.Message
	totalImportance := 0.0

	for _, node := range nodes {
		contents = append(contents, node.Content)
		totalImportance += node.Importance
		if node.Type == NodeTypeMessage {
			messages = append(messages, models.Message{
				ID:      0, // 需要映射
				Content: node.Content,
				Role:    "assistant", // 简化处理
			})
		}
	}

	// 生成摘要（使用 LLM 或规则）
	summary := hm.generateSummary(contents)

	return &ContextNode{
		ID:         fmt.Sprintf("summary_%d_%d", level, time.Now().Unix()),
		Type:       NodeTypeSummary,
		Content:    summary,
		Messages:   messages, // 保留原始消息引用
		Level:      level,
		Importance: totalImportance / float64(len(nodes)), // 平均重要性
		TokenCount: estimateTokens(summary),
		CreatedAt:  time.Now(),
	}
}

// generateSummary 生成摘要
func (hm *HierarchicalMemory) generateSummary(contents []string) string {
	// 简单规则摘要（实际项目中可以使用 LLM）
	if len(contents) == 0 {
		return ""
	}

	// 提取关键信息
	keyPoints := make([]string, 0)
	for _, content := range contents {
		// 提取每段的前 50 个字符作为要点
		runes := []rune(content)
		if len(runes) > 50 {
			keyPoints = append(keyPoints, string(runes[:50])+"...")
		} else {
			keyPoints = append(keyPoints, content)
		}
	}

	// 合并为摘要
	summary := "【历史对话摘要】\n"
	for i, point := range keyPoints {
		summary += fmt.Sprintf("%d. %s\n", i+1, point)
	}

	return summary
}

// replaceWithSummary 用摘要节点替换原始节点
func (hm *HierarchicalMemory) replaceWithSummary(nodes []*ContextNode, summary *ContextNode) {
	if len(nodes) == 0 {
		return
	}

	// 找到这些节点的父节点（假设它们有相同的父节点）
	parent := nodes[0].Parent
	if parent == nil {
		return
	}

	// 从父节点移除原始节点
	newChildren := make([]*ContextNode, 0)
	nodeMap := make(map[string]bool)
	for _, node := range nodes {
		nodeMap[node.ID] = true
	}

	for _, child := range parent.Children {
		if !nodeMap[child.ID] {
			newChildren = append(newChildren, child)
		}
	}

	// 添加摘要节点
	summary.Parent = parent
	newChildren = append(newChildren, summary)
	parent.Children = newChildren
}

// compressLevel 压缩指定层级
func (hm *HierarchicalMemory) compressLevel(level int) {
	nodes := hm.getNodesAtLevel(level)
	if len(nodes) < 3 {
		return
	}

	summary := hm.createSummary(nodes, level+1)
	hm.replaceWithSummary(nodes, summary)
	hm.recalculateTokens()
}

// recalculateTokens 重新计算 Token 数
func (hm *HierarchicalMemory) recalculateTokens() {
	hm.currentTokens = 0
	var count func(node *ContextNode)
	count = func(node *ContextNode) {
		hm.currentTokens += node.TokenCount
		for _, child := range node.Children {
			count(child)
		}
	}
	count(hm.root)
}

// BuildContextForLLM 为 LLM 构建上下文
func (hm *HierarchicalMemory) BuildContextForLLM(maxTokens int) []models.Message {
	var messages []models.Message
	tokens := 0

	// 1. 优先包含最近的高重要性消息
	recentImportant := hm.getRecentImportantMessages(10)
	for _, node := range recentImportant {
		msg := models.Message{
			Role:    hm.inferRole(node),
			Content: node.Content,
		}
		msgTokens := estimateTokens(node.Content)
		if tokens+msgTokens > maxTokens {
			break
		}
		messages = append(messages, msg)
		tokens += msgTokens
		node.AccessedAt = time.Now()
		node.AccessCount++
	}

	// 2. 如果还有空间，包含相关摘要
	if float64(tokens) < float64(maxTokens)*0.8 {
		summaries := hm.getRelevantSummaries(maxTokens - tokens)
		messages = append(summaries, messages...) // 摘要放在前面
	}

	return messages
}

// getRecentImportantMessages 获取最近的重要消息
func (hm *HierarchicalMemory) getRecentImportantMessages(limit int) []*ContextNode {
	var nodes []*ContextNode
	var collect func(node *ContextNode)
	collect = func(node *ContextNode) {
		if node.Type == NodeTypeMessage {
			nodes = append(nodes, node)
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(hm.root)

	// 按时间和重要性排序
	sort.Slice(nodes, func(i, j int) bool {
		// 时间权重 0.4，重要性权重 0.6
		timeScoreI := float64(nodes[i].CreatedAt.Unix()) * 0.4
		timeScoreJ := float64(nodes[j].CreatedAt.Unix()) * 0.4
		importanceI := nodes[i].Importance * 0.6
		importanceJ := nodes[j].Importance * 0.6
		return timeScoreI+importanceI > timeScoreJ+importanceJ
	})

	if len(nodes) > limit {
		return nodes[:limit]
	}
	return nodes
}

// getRelevantSummaries 获取相关摘要
func (hm *HierarchicalMemory) getRelevantSummaries(maxTokens int) []models.Message {
	var summaries []models.Message
	tokens := 0

	var collect func(node *ContextNode)
	collect = func(node *ContextNode) {
		if node.Type == NodeTypeSummary {
			msgTokens := estimateTokens(node.Content)
			if tokens+msgTokens <= maxTokens {
				summaries = append(summaries, models.Message{
					Role:    "system",
					Content: node.Content,
				})
				tokens += msgTokens
			}
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(hm.root)

	return summaries
}

// inferRole 推断消息角色
func (hm *HierarchicalMemory) inferRole(node *ContextNode) string {
	// 简化处理，实际可以根据节点内容或关联信息推断
	if strings.Contains(node.Content, "？") || strings.Contains(node.Content, "吗") {
		return "user"
	}
	return "assistant"
}

// estimateTokens 估算 Token 数（简化版）
func estimateTokens(text string) int {
	// 中文字符：1 字符 ≈ 1.5 tokens
	// 英文单词：1 单词 ≈ 1.3 tokens
	// 简化计算：总字符数 / 2
	return len([]rune(text)) / 2
}

// GetStats 获取统计信息
func (hm *HierarchicalMemory) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_tokens":   hm.currentTokens,
		"max_tokens":     hm.maxTokens,
		"message_count":  0,
		"summary_count":  0,
		"compression_ratio": 0.0,
	}

	var countMessages, countSummaries int
	var count func(node *ContextNode)
	count = func(node *ContextNode) {
		switch node.Type {
		case NodeTypeMessage:
			countMessages++
		case NodeTypeSummary:
			countSummaries++
		}
		for _, child := range node.Children {
			count(child)
		}
	}
	count(hm.root)

	stats["message_count"] = countMessages
	stats["summary_count"] = countSummaries

	if countMessages > 0 {
		stats["compression_ratio"] = float64(countSummaries) / float64(countMessages)
	}

	return stats
}
