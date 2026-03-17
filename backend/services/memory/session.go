package memory

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/cache"
	"github.com/hock1024always/GoEdu/services/llm"
	"gorm.io/gorm"
)

// SessionManager 会话管理器
type SessionManager struct {
	llmClient *llm.Client
	cacheSvc  *cache.CacheService
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		llmClient: llm.NewClient(),
		cacheSvc:  cache.NewCacheService(),
	}
}

// CreateSession 创建新会话
func (m *SessionManager) CreateSession(userID uint, mode string) (*models.Session, error) {
	session := &models.Session{
		ID:        uuid.New().String(),
		UserID:    userID,
		Mode:      mode,
		Title:     "新对话",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := dao.Db.Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}

	return session, nil
}

// GetSession 获取会话
func (m *SessionManager) GetSession(sessionID string) (*models.Session, error) {
	var session models.Session
	err := dao.Db.Preload("Messages").Where("id = ?", sessionID).First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %v", err)
	}
	return &session, nil
}

// AddMessage 添加消息到会话（带智能标签）
func (m *SessionManager) AddMessage(sessionID string, role string, content string, toolCalls string, toolResult string) error {
	// 自动识别消息标签
	tag := m.classifyMessage(role, content, toolCalls)
	
	// 提取重要性
	importance := m.estimateImportance(content, tag)
	
	// 提取实体
	entities := m.extractEntities(content)
	entitiesJSON, _ := json.Marshal(entities)

	message := &models.Message{
		SessionID:  sessionID,
		Role:       role,
		Content:    content,
		Tag:        tag,
		Importance: importance,
		Entities:   string(entitiesJSON),
		ToolCalls:  toolCalls,
		ToolResult: toolResult,
		CreatedAt:  time.Now(),
	}

	if err := dao.Db.Create(message).Error; err != nil {
		return fmt.Errorf("failed to add message: %v", err)
	}

	// 更新会话统计
	dao.Db.Model(&models.Session{}).Where("id = ?", sessionID).Updates(map[string]interface{}{
		"updated_at":    time.Now(),
		"message_count": gorm.Expr("message_count + 1"),
	})

	// 清除会话缓存
	if m.cacheSvc.IsEnabled() {
		ctx := m.cacheSvc.GetContext()
		m.cacheSvc.Delete(ctx, cache.SessionCacheKey(sessionID))
	}

	return nil
}

// GetMessages 获取会话消息历史
func (m *SessionManager) GetMessages(sessionID string, limit int) ([]models.Message, error) {
	var messages []models.Message
	query := dao.Db.Where("session_id = ?", sessionID).Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %v", err)
	}

	return messages, nil
}

// BuildContext 构建对话上下文（智能选择）
func (m *SessionManager) BuildContext(sessionID string, maxMessages int) ([]llm.Message, error) {
	// 获取所有消息
	allMessages, err := m.GetMessages(sessionID, 0)
	if err != nil {
		return nil, err
	}

	// 如果消息数量在限制内，直接返回
	if len(allMessages) <= maxMessages {
		return m.messagesToLLMContext(allMessages), nil
	}

	// 智能选择消息
	selectedMessages := m.selectImportantMessages(allMessages, maxMessages)

	return m.messagesToLLMContext(selectedMessages), nil
}

// selectImportantMessages 智能选择重要消息
func (m *SessionManager) selectImportantMessages(messages []models.Message, maxCount int) []models.Message {
	if len(messages) <= maxCount {
		return messages
	}

	// 策略：
	// 1. 始终保留最近 5 条消息
	// 2. 保留高重要性消息
	// 3. 保留用户问题（用于理解上下文）
	// 4. 保留摘要消息

	recentCount := 5
	result := make([]models.Message, 0, maxCount)
	
	// 标记已选中的消息索引
	selected := make(map[int]bool)

	// 1. 选择最近的消息
	start := len(messages) - recentCount
	if start < 0 {
		start = 0
	}
	for i := start; i < len(messages); i++ {
		result = append(result, messages[i])
		selected[i] = true
	}

	// 2. 选择高重要性和关键消息
	for i := 0; i < len(messages)-recentCount && len(result) < maxCount; i++ {
		if selected[i] {
			continue
		}

		msg := messages[i]
		// 高重要性 或 用户问题 或 摘要
		if msg.Importance >= 8 || msg.Tag == models.TagQuestion || msg.Tag == models.TagSummary {
			result = append(result, msg)
			selected[i] = true
		}
	}

	// 3. 填充剩余空间
	for i := 0; i < len(messages)-recentCount && len(result) < maxCount; i++ {
		if !selected[i] {
			result = append(result, messages[i])
			selected[i] = true
		}
	}

	// 按时间排序
	m.sortMessagesByTime(result)

	return result
}

// messagesToLLMContext 转换为 LLM 上下文格式
func (m *SessionManager) messagesToLLMContext(messages []models.Message) []llm.Message {
	context := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		context = append(context, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return context
}

// sortMessagesByTime 按时间排序消息
func (m *SessionManager) sortMessagesByTime(messages []models.Message) {
	for i := 0; i < len(messages)-1; i++ {
		for j := i + 1; j < len(messages); j++ {
			if messages[i].CreatedAt.After(messages[j].CreatedAt) {
				messages[i], messages[j] = messages[j], messages[i]
			}
		}
	}
}

// classifyMessage 分类消息标签
func (m *SessionManager) classifyMessage(role string, content string, toolCalls string) string {
	// 如果有工具调用
	if toolCalls != "" {
		return models.TagToolCall
	}

	content = strings.TrimSpace(content)
	lowerContent := strings.ToLower(content)

	// 规则匹配
	switch {
	case m.isGreeting(lowerContent):
		return models.TagGreeting
	case m.isThanks(lowerContent):
		return models.TagThanks
	case m.isQuestion(content):
		return models.TagQuestion
	case m.isFollowUp(content):
		return models.TagFollowUp
	case role == "assistant":
		return models.TagAnswer
	default:
		return models.TagOther
	}
}

// isQuestion 判断是否是问题
func (m *SessionManager) isQuestion(content string) bool {
	// 中文问号
	if strings.Contains(content, "？") || strings.Contains(content, "?") {
		return true
	}

	// 疑问词
	questionWords := []string{"什么", "为什么", "怎么", "如何", "谁", "哪", "几", "多少", "吗", "呢"}
	for _, word := range questionWords {
		if strings.Contains(content, word) {
			return true
		}
	}

	return false
}

// isFollowUp 判断是否是追问
func (m *SessionManager) isFollowUp(content string) bool {
	followUpPatterns := []string{"那", "那么", "接着", "继续", "还有", "另外", "补充"}
	for _, pattern := range followUpPatterns {
		if strings.HasPrefix(content, pattern) {
			return true
		}
	}
	return false
}

// isGreeting 判断是否是问候
func (m *SessionManager) isGreeting(content string) bool {
	greetings := []string{"你好", "您好", "hi", "hello", "嗨", "早上好", "晚上好"}
	for _, g := range greetings {
		if strings.HasPrefix(content, g) {
			return true
		}
	}
	return false
}

// isThanks 判断是否是感谢
func (m *SessionManager) isThanks(content string) bool {
	thanks := []string{"谢谢", "感谢", "thanks", "thank you"}
	for _, t := range thanks {
		if strings.Contains(content, t) {
			return true
		}
	}
	return false
}

// estimateImportance 估计消息重要性 (1-10)
func (m *SessionManager) estimateImportance(content string, tag string) int {
	importance := 5 // 默认中等重要性

	// 根据标签调整
	switch tag {
	case models.TagQuestion:
		importance = 8 // 问题重要
	case models.TagAnswer:
		importance = 7 // 回答较重要
	case models.TagSummary:
		importance = 9 // 摘要非常重要
	case models.TagGreeting, models.TagThanks:
		importance = 2 // 问候和感谢不太重要
	case models.TagFollowUp:
		importance = 7 // 追问重要
	}

	// 根据内容长度调整
	if len(content) > 500 {
		importance += 1
	}

	// 包含关键实体
	keyEntities := []string{"齐桓公", "管仲", "春秋", "战国", "孔子", "老子"}
	for _, entity := range keyEntities {
		if strings.Contains(content, entity) {
			importance += 1
			break
		}
	}

	// 限制范围
	if importance > 10 {
		importance = 10
	}
	if importance < 1 {
		importance = 1
	}

	return importance
}

// extractEntities 提取实体
func (m *SessionManager) extractEntities(content string) []models.Entity {
	entities := make([]models.Entity, 0)

	// 春秋战国人物
	persons := []string{"齐桓公", "管仲", "晋文公", "楚庄王", "秦穆公", "宋襄公", 
		"孔子", "老子", "墨子", "庄子", "孟子", "韩非子", "孙子", "吴起"}
	
	// 诸侯国
	states := []string{"齐国", "晋国", "楚国", "秦国", "燕国", "赵国", "魏国", "韩国"}

	// 战役
	battles := []string{"城濮之战", "邲之战", "鄢陵之战", "马陵之战", "长平之战"}

	// 提取人物
	for _, person := range persons {
		if strings.Contains(content, person) {
			entities = append(entities, models.Entity{
				Name: person,
				Type: "person",
			})
		}
	}

	// 提取国家
	for _, state := range states {
		if strings.Contains(content, state) {
			entities = append(entities, models.Entity{
				Name: state,
				Type: "state",
			})
		}
	}

	// 提取战役
	for _, battle := range battles {
		if strings.Contains(content, battle) {
			entities = append(entities, models.Entity{
				Name: battle,
				Type: "battle",
			})
		}
	}

	// 提取年份（公元前）
	yearPattern := regexp.MustCompile(`公元前(\d+)年`)
	years := yearPattern.FindAllString(content, -1)
	for _, year := range years {
		entities = append(entities, models.Entity{
			Name: year,
			Type: "time",
		})
	}

	return entities
}

// SummarizeHistory 压缩历史对话（智能压缩）
func (m *SessionManager) SummarizeHistory(sessionID string) (string, error) {
	// 获取所有历史消息
	messages, err := m.GetMessages(sessionID, 0)
	if err != nil {
		return "", err
	}

	if len(messages) <= 10 {
		return "", nil // 不需要压缩
	}

	// 分离需要压缩的消息和保留的消息
	keepRecent := 5
	toSummarize := messages[:len(messages)-keepRecent]
	// toKeep := messages[len(messages)-keepRecent:] // 最近的消息保留，不参与摘要

	// 提取关键信息
	keyEntities := m.aggregateEntities(toSummarize)
	keyTopics := m.extractTopics(toSummarize)

	// 构建摘要请求
	var historyText strings.Builder
	historyText.WriteString("以下是对话历史，请生成结构化摘要：\n\n")
	
	for _, msg := range toSummarize {
		if msg.Importance >= 6 || msg.Tag == models.TagQuestion {
			historyText.WriteString(fmt.Sprintf("[%s][%s] %s\n", 
				msg.Tag, msg.Role, msg.Content))
		}
	}

	prompt := historyText.String() + `

请按以下格式生成摘要：
1. 主要话题：列出讨论的主要话题
2. 关键问题：用户提出的关键问题
3. 重要结论：得出的重要结论或信息
4. 涉及实体：提及的历史人物、事件等`

	resp, err := m.llmClient.Chat([]llm.Message{
		{Role: "system", Content: "你是一个对话摘要助手，擅长提取对话中的关键信息。"},
		{Role: "user", Content: prompt},
	}, nil)

	if err != nil {
		return "", err
	}

	summary := resp.Choices[0].Message.Content

	// 标记已摘要的消息
	ids := make([]uint, len(toSummarize))
	for i, msg := range toSummarize {
		ids[i] = msg.ID
	}
	dao.Db.Model(&models.Message{}).Where("id IN ?", ids).Update("is_summarized", true)

	// 保存摘要消息
	entitiesJSON, _ := json.Marshal(keyEntities)
	topicsJSON, _ := json.Marshal(keyTopics)
	
	summaryMsg := &models.Message{
		SessionID:  sessionID,
		Role:       "system",
		Content:    "[对话摘要]\n" + summary,
		Tag:        models.TagSummary,
		Importance: 9,
		Entities:   string(entitiesJSON),
		Topics:     string(topicsJSON),
		CreatedAt:  time.Now(),
	}
	dao.Db.Create(summaryMsg)

	// 更新会话摘要
	dao.Db.Model(&models.Session{}).Where("id = ?", sessionID).Updates(map[string]interface{}{
		"summary":     summary,
		"key_entities": string(entitiesJSON),
		"topics":      string(topicsJSON),
	})

	return summary, nil
}

// aggregateEntities 聚合实体
func (m *SessionManager) aggregateEntities(messages []models.Message) []models.Entity {
	entityMap := make(map[string]models.Entity)

	for _, msg := range messages {
		var entities []models.Entity
		if err := json.Unmarshal([]byte(msg.Entities), &entities); err == nil {
			for _, e := range entities {
				entityMap[e.Name+e.Type] = e
			}
		}
	}

	result := make([]models.Entity, 0, len(entityMap))
	for _, e := range entityMap {
		result = append(result, e)
	}

	return result
}

// extractTopics 提取话题
func (m *SessionManager) extractTopics(messages []models.Message) []models.Topic {
	topicMap := make(map[string]*models.Topic)

	// 关键词到话题的映射
	topicKeywords := map[string][]string{
		"政治改革": {"改革", "变法", "制度", "政策"},
		"军事战争": {"战争", "战役", "攻打", "征战"},
		"思想文化": {"思想", "学说", "学派", "哲学"},
		"人物传记": {"生平", "事迹", "成就"},
		"外交关系": {"联盟", "会盟", "外交"},
	}

	for _, msg := range messages {
		content := msg.Content
		for topicName, keywords := range topicKeywords {
			for _, kw := range keywords {
				if strings.Contains(content, kw) {
					if _, exists := topicMap[topicName]; !exists {
						topicMap[topicName] = &models.Topic{
							Name:     topicName,
							Keywords: keywords,
							Count:    0,
						}
					}
					topicMap[topicName].Count++
					break
				}
			}
		}
	}

	result := make([]models.Topic, 0, len(topicMap))
	for _, t := range topicMap {
		if t.Count > 0 {
			result = append(result, *t)
		}
	}

	return result
}

// ListSessions 列出用户的会话
func (m *SessionManager) ListSessions(userID uint, limit int) ([]models.Session, error) {
	var sessions []models.Session
	query := dao.Db.Where("user_id = ?", userID).Order("updated_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %v", err)
	}

	return sessions, nil
}

// DeleteSession 删除会话
func (m *SessionManager) DeleteSession(sessionID string) error {
	// 删除消息
	if err := dao.Db.Where("session_id = ?", sessionID).Delete(&models.Message{}).Error; err != nil {
		return fmt.Errorf("failed to delete messages: %v", err)
	}

	// 删除会话
	if err := dao.Db.Where("id = ?", sessionID).Delete(&models.Session{}).Error; err != nil {
		return fmt.Errorf("failed to delete session: %v", err)
	}

	// 清除缓存
	if m.cacheSvc.IsEnabled() {
		ctx := m.cacheSvc.GetContext()
		m.cacheSvc.Delete(ctx, cache.SessionCacheKey(sessionID))
	}

	return nil
}

// UpdateTitle 更新会话标题
func (m *SessionManager) UpdateTitle(sessionID string, title string) error {
	return dao.Db.Model(&models.Session{}).Where("id = ?", sessionID).Update("title", title).Error
}

// GenerateTitle 根据首条消息生成会话标题
func (m *SessionManager) GenerateTitle(sessionID string, firstMessage string) error {
	// 提取关键实体作为标题
	entities := m.extractEntities(firstMessage)
	
	var title string
	if len(entities) > 0 {
		// 使用第一个实体作为标题
		title = fmt.Sprintf("关于%s的讨论", entities[0].Name)
	} else {
		// 简单截取
		title = firstMessage
		if len(title) > 20 {
			title = title[:20] + "..."
		}
	}
	
	return m.UpdateTitle(sessionID, title)
}

// GetToolCallsJSON 将工具调用转为 JSON
func GetToolCallsJSON(toolCalls []llm.ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}
	jsonBytes, _ := json.Marshal(toolCalls)
	return string(jsonBytes)
}

// GetSessionStats 获取会话统计
func (m *SessionManager) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	var session models.Session
	if err := dao.Db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, err
	}

	var messages []models.Message
	if err := dao.Db.Where("session_id = ?", sessionID).Find(&messages).Error; err != nil {
		return nil, err
	}

	// 统计各标签数量
	tagCounts := make(map[string]int)
	importanceSum := 0
	entityCount := 0

	for _, msg := range messages {
		tagCounts[msg.Tag]++
		importanceSum += msg.Importance
		
		var entities []models.Entity
		if err := json.Unmarshal([]byte(msg.Entities), &entities); err == nil {
			entityCount += len(entities)
		}
	}

	avgImportance := 0.0
	if len(messages) > 0 {
		avgImportance = float64(importanceSum) / float64(len(messages))
	}

	return map[string]interface{}{
		"session_id":       sessionID,
		"title":            session.Title,
		"message_count":    len(messages),
		"tag_distribution": tagCounts,
		"avg_importance":   avgImportance,
		"entity_count":     entityCount,
		"has_summary":      session.Summary != "",
	}, nil
}
