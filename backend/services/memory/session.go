package memory

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
	"gorm.io/gorm"
)

// SessionManager 会话管理器
type SessionManager struct {
	llmClient *llm.Client
}

// NewSessionManager 创建会话管理器
func NewSessionManager() *SessionManager {
	return &SessionManager{
		llmClient: llm.NewClient(),
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

// AddMessage 添加消息到会话
func (m *SessionManager) AddMessage(sessionID string, role string, content string, toolCalls string, toolResult string) error {
	message := &models.Message{
		SessionID:  sessionID,
		Role:       role,
		Content:    content,
		ToolCalls:  toolCalls,
		ToolResult: toolResult,
		CreatedAt:  time.Now(),
	}

	if err := dao.Db.Create(message).Error; err != nil {
		return fmt.Errorf("failed to add message: %v", err)
	}

	// 更新会话时间
	dao.Db.Model(&models.Session{}).Where("id = ?", sessionID).Update("updated_at", time.Now())

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

// BuildContext 构建对话上下文
func (m *SessionManager) BuildContext(sessionID string, maxMessages int) ([]llm.Message, error) {
	messages, err := m.GetMessages(sessionID, maxMessages)
	if err != nil {
		return nil, err
	}

	context := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		context = append(context, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return context, nil
}

// SummarizeHistory 压缩历史对话（当对话过长时）
func (m *SessionManager) SummarizeHistory(sessionID string) (string, error) {
	// 获取所有历史消息
	messages, err := m.GetMessages(sessionID, 0)
	if err != nil {
		return "", err
	}

	if len(messages) <= 10 {
		return "", nil // 不需要压缩
	}

	// 构建摘要请求
	var historyText string
	for _, msg := range messages[:len(messages)-5] { // 保留最近5条
		historyText += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
	}

	prompt := fmt.Sprintf("请将以下历史对话内容压缩成简洁的摘要，保留关键信息：\n\n%s", historyText)

	resp, err := m.llmClient.Chat([]llm.Message{
		{Role: "user", Content: prompt},
	}, nil)

	if err != nil {
		return "", err
	}

	summary := resp.Choices[0].Message.Content

	// 保存摘要作为系统消息
	m.AddMessage(sessionID, "system", "[对话摘要] "+summary, "", "")

	return summary, nil
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

	return nil
}

// UpdateTitle 更新会话标题
func (m *SessionManager) UpdateTitle(sessionID string, title string) error {
	return dao.Db.Model(&models.Session{}).Where("id = ?", sessionID).Update("title", title).Error
}

// GenerateTitle 根据首条消息生成会话标题
func (m *SessionManager) GenerateTitle(sessionID string, firstMessage string) error {
	// 简单截取前20个字符作为标题
	title := firstMessage
	if len(title) > 20 {
		title = title[:20] + "..."
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
