package lessonprep

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// WorkflowManager 工作流管理器
type WorkflowManager struct{}

// NewWorkflowManager 创建工作流管理器
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{}
}

// CreateWorkflow 创建新的备课工作流
func (wm *WorkflowManager) CreateWorkflow(userID uint, topic, subject, grade string, duration int) (*models.LessonWorkflow, error) {
	if subject == "" {
		subject = "历史"
	}
	if grade == "" {
		grade = "初中"
	}
	if duration <= 0 {
		duration = 45
	}

	wf := &models.LessonWorkflow{
		ID:        uuid.New().String(),
		UserID:    userID,
		Status:    models.WFStatusInit,
		Topic:     topic,
		Subject:   subject,
		Grade:     grade,
		Duration:  duration,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := dao.Db.Create(wf).Error; err != nil {
		return nil, fmt.Errorf("创建工作流失败: %v", err)
	}
	return wf, nil
}

// GetWorkflow 获取工作流
func (wm *WorkflowManager) GetWorkflow(workflowID string) (*models.LessonWorkflow, error) {
	var wf models.LessonWorkflow
	if err := dao.Db.Where("id = ?", workflowID).First(&wf).Error; err != nil {
		return nil, fmt.Errorf("获取工作流失败: %v", err)
	}
	return &wf, nil
}

// UpdateStatus 更新工作流状态
func (wm *WorkflowManager) UpdateStatus(workflowID, status string) error {
	return dao.Db.Model(&models.LessonWorkflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// SaveOutline 保存大纲
func (wm *WorkflowManager) SaveOutline(workflowID string, outline *models.CourseOutline) error {
	jsonBytes, err := json.Marshal(outline)
	if err != nil {
		return fmt.Errorf("序列化大纲失败: %v", err)
	}
	return dao.Db.Model(&models.LessonWorkflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"outline_json": string(jsonBytes),
			"status":       models.WFStatusOutlineReady,
			"updated_at":   time.Now(),
		}).Error
}

// GetOutline 获取大纲
func (wm *WorkflowManager) GetOutline(workflowID string) (*models.CourseOutline, error) {
	wf, err := wm.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}
	if wf.OutlineJSON == "" {
		return nil, fmt.Errorf("大纲尚未生成")
	}
	var outline models.CourseOutline
	if err := json.Unmarshal([]byte(wf.OutlineJSON), &outline); err != nil {
		return nil, fmt.Errorf("解析大纲失败: %v", err)
	}
	return &outline, nil
}

// SaveLessonPlan 保存教案
func (wm *WorkflowManager) SaveLessonPlan(workflowID, plan string) error {
	return dao.Db.Model(&models.LessonWorkflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"lesson_plan": plan,
			"status":      models.WFStatusPlanReady,
			"updated_at":  time.Now(),
		}).Error
}

// SavePPTPath 保存 PPT 路径
func (wm *WorkflowManager) SavePPTPath(workflowID, pptPath string) error {
	return dao.Db.Model(&models.LessonWorkflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"ppt_path":   pptPath,
			"status":     models.WFStatusDone,
			"updated_at": time.Now(),
		}).Error
}

// SetError 记录错误
func (wm *WorkflowManager) SetError(workflowID, errMsg string) error {
	return dao.Db.Model(&models.LessonWorkflow{}).
		Where("id = ?", workflowID).
		Updates(map[string]interface{}{
			"status":     models.WFStatusFailed,
			"error_msg":  errMsg,
			"updated_at": time.Now(),
		}).Error
}

// AddMessage 添加工作流对话消息
func (wm *WorkflowManager) AddMessage(workflowID, stage, role, content string) error {
	msg := &models.WorkflowMessage{
		WorkflowID: workflowID,
		Stage:      stage,
		Role:       role,
		Content:    content,
		CreatedAt:  time.Now(),
	}
	return dao.Db.Create(msg).Error
}

// GetMessages 获取工作流某阶段的对话消息
func (wm *WorkflowManager) GetMessages(workflowID, stage string) ([]models.WorkflowMessage, error) {
	var msgs []models.WorkflowMessage
	query := dao.Db.Where("workflow_id = ?", workflowID).Order("created_at ASC")
	if stage != "" {
		query = query.Where("stage = ?", stage)
	}
	if err := query.Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

// ListWorkflows 列出工作流
func (wm *WorkflowManager) ListWorkflows(userID uint, limit int) ([]models.LessonWorkflow, error) {
	if limit <= 0 {
		limit = 20
	}
	var workflows []models.LessonWorkflow
	err := dao.Db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&workflows).Error
	return workflows, err
}
