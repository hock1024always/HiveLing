package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/lessonprep"
)

// LessonPrepController 备课工作流控制器
type LessonPrepController struct {
	workflowMgr     *lessonprep.WorkflowManager
	outlineService  *lessonprep.OutlineService
	editService     *lessonprep.OutlineEditService
	planService     *lessonprep.LessonPlanService
	pptService      *lessonprep.PPTService
}

// NewLessonPrepController 创建备课控制器
func NewLessonPrepController() *LessonPrepController {
	return &LessonPrepController{
		workflowMgr:     lessonprep.NewWorkflowManager(),
		outlineService:  lessonprep.NewOutlineService(),
		editService:     lessonprep.NewOutlineEditService(),
		planService:     lessonprep.NewLessonPlanService(),
		pptService:      lessonprep.NewPPTService("runtime/ppt"),
	}
}

// --- SSE 工具函数 ---

// WorkflowSSEEvent 工作流 SSE 事件
type WorkflowSSEEvent struct {
	Type       string      `json:"type"`                  // status, text, outline, plan, ppt, error, done
	Content    string      `json:"content,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	WorkflowID string     `json:"workflow_id,omitempty"`
}

func sendWorkflowSSE(c *gin.Context, event WorkflowSSEEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
	c.Writer.(http.Flusher).Flush()
}

func setSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
}

// --- API 接口 ---

// StartRequest 启动工作流请求
type StartRequest struct {
	Topic    string `json:"topic" binding:"required"`
	Subject  string `json:"subject"`
	Grade    string `json:"grade"`
	Duration int    `json:"duration"`
}

// Start 启动备课工作流：输入需求 → 生成大纲
func (ctrl *LessonPrepController) Start(c *gin.Context) {
	var req StartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供课程主题(topic)"})
		return
	}

	setSSEHeaders(c)

	// 1. 创建工作流
	wf, err := ctrl.workflowMgr.CreateWorkflow(0, req.Topic, req.Subject, req.Grade, req.Duration)
	if err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: err.Error()})
		return
	}

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:       "status",
		Content:    "工作流已创建，正在生成大纲...",
		WorkflowID: wf.ID,
	})

	// 2. 更新状态
	ctrl.workflowMgr.UpdateStatus(wf.ID, models.WFStatusOutlineGenerating)

	// 3. 记录用户请求
	ctrl.workflowMgr.AddMessage(wf.ID, "outline", "user", req.Topic)

	// 4. 流式生成大纲
	outline, err := ctrl.outlineService.GenerateOutlineStream(
		req.Topic, wf.Subject, wf.Grade, wf.Duration,
		func(text string) error {
			sendWorkflowSSE(c, WorkflowSSEEvent{Type: "text", Content: text})
			return nil
		},
	)

	if err != nil {
		ctrl.workflowMgr.SetError(wf.ID, err.Error())
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "大纲生成失败: " + err.Error()})
		return
	}

	// 5. 保存大纲
	if err := ctrl.workflowMgr.SaveOutline(wf.ID, outline); err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "保存大纲失败: " + err.Error()})
		return
	}

	outlineJSON, _ := json.Marshal(outline)
	ctrl.workflowMgr.AddMessage(wf.ID, "outline", "assistant", string(outlineJSON))

	// 6. 发送大纲结构
	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type: "outline",
		Data: outline,
	})

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:       "done",
		Content:    "大纲生成完成，您可以对大纲提出修改意见，或确认后生成教案。",
		WorkflowID: wf.ID,
	})
}

// EditOutlineRequest 修改大纲请求
type EditOutlineRequest struct {
	WorkflowID string `json:"workflow_id" binding:"required"`
	Message    string `json:"message" binding:"required"`
}

// EditOutline 对话式修改大纲
func (ctrl *LessonPrepController) EditOutline(c *gin.Context) {
	var req EditOutlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id 和修改意见(message)"})
		return
	}

	setSSEHeaders(c)

	// 1. 获取当前大纲
	outline, err := ctrl.workflowMgr.GetOutline(req.WorkflowID)
	if err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: err.Error()})
		return
	}

	// 2. 获取历史对话
	history, _ := ctrl.workflowMgr.GetMessages(req.WorkflowID, "outline")

	// 3. 更新状态
	ctrl.workflowMgr.UpdateStatus(req.WorkflowID, models.WFStatusOutlineEditing)

	// 4. 记录用户修改请求
	ctrl.workflowMgr.AddMessage(req.WorkflowID, "outline", "user", req.Message)

	sendWorkflowSSE(c, WorkflowSSEEvent{Type: "status", Content: "正在根据您的意见修改大纲..."})

	// 5. 流式修改
	newOutline, err := ctrl.editService.EditOutlineStream(
		outline, req.Message, history,
		func(text string) error {
			sendWorkflowSSE(c, WorkflowSSEEvent{Type: "text", Content: text})
			return nil
		},
	)

	if err != nil {
		ctrl.workflowMgr.UpdateStatus(req.WorkflowID, models.WFStatusOutlineReady)
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "修改大纲失败: " + err.Error()})
		return
	}

	// 6. 保存修改后的大纲
	if err := ctrl.workflowMgr.SaveOutline(req.WorkflowID, newOutline); err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "保存大纲失败: " + err.Error()})
		return
	}

	outlineJSON, _ := json.Marshal(newOutline)
	ctrl.workflowMgr.AddMessage(req.WorkflowID, "outline", "assistant", string(outlineJSON))

	// 7. 发送更新后的大纲
	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type: "outline",
		Data: newOutline,
	})

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:       "done",
		Content:    "大纲已修改完成，您可以继续修改或确认后生成教案。",
		WorkflowID: req.WorkflowID,
	})
}

// ConfirmOutlineRequest 确认大纲请求
type ConfirmOutlineRequest struct {
	WorkflowID string `json:"workflow_id" binding:"required"`
}

// ConfirmOutline 确认大纲
func (ctrl *LessonPrepController) ConfirmOutline(c *gin.Context) {
	var req ConfirmOutlineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	wf, err := ctrl.workflowMgr.GetWorkflow(req.WorkflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if wf.OutlineJSON == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "大纲尚未生成"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "大纲已确认，可以生成教案",
		"workflow_id": req.WorkflowID,
		"status":      models.WFStatusOutlineReady,
	})
}

// GeneratePlanRequest 生成教案请求
type GeneratePlanRequest struct {
	WorkflowID string `json:"workflow_id" binding:"required"`
}

// GeneratePlan 基于确认的大纲生成教案
func (ctrl *LessonPrepController) GeneratePlan(c *gin.Context) {
	var req GeneratePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	setSSEHeaders(c)

	// 1. 获取大纲
	outline, err := ctrl.workflowMgr.GetOutline(req.WorkflowID)
	if err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: err.Error()})
		return
	}

	// 2. 更新状态
	ctrl.workflowMgr.UpdateStatus(req.WorkflowID, models.WFStatusPlanGenerating)

	sendWorkflowSSE(c, WorkflowSSEEvent{Type: "status", Content: "正在基于大纲生成详细教案..."})

	// 3. 流式生成教案
	plan, err := ctrl.planService.GenerateLessonPlanStream(
		outline,
		func(text string) error {
			sendWorkflowSSE(c, WorkflowSSEEvent{Type: "text", Content: text})
			return nil
		},
	)

	if err != nil {
		ctrl.workflowMgr.SetError(req.WorkflowID, err.Error())
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "教案生成失败: " + err.Error()})
		return
	}

	// 4. 保存教案
	if err := ctrl.workflowMgr.SaveLessonPlan(req.WorkflowID, plan); err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "保存教案失败: " + err.Error()})
		return
	}

	ctrl.workflowMgr.AddMessage(req.WorkflowID, "plan", "assistant", plan)

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:    "plan",
		Content: plan,
	})

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:       "done",
		Content:    "教案生成完成，可以继续生成 PPT。",
		WorkflowID: req.WorkflowID,
	})
}

// GeneratePPTRequest 生成 PPT 请求
type GeneratePPTRequest struct {
	WorkflowID string `json:"workflow_id" binding:"required"`
}

// GeneratePPT 生成 PPT
func (ctrl *LessonPrepController) GeneratePPT(c *gin.Context) {
	var req GeneratePPTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	setSSEHeaders(c)

	// 1. 获取工作流
	wf, err := ctrl.workflowMgr.GetWorkflow(req.WorkflowID)
	if err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: err.Error()})
		return
	}

	// 2. 获取大纲
	outline, err := ctrl.workflowMgr.GetOutline(req.WorkflowID)
	if err != nil {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: err.Error()})
		return
	}

	if wf.LessonPlan == "" {
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "请先生成教案"})
		return
	}

	// 3. 更新状态
	ctrl.workflowMgr.UpdateStatus(req.WorkflowID, models.WFStatusPPTGenerating)

	sendWorkflowSSE(c, WorkflowSSEEvent{Type: "status", Content: "正在生成 PPT 结构..."})

	// 4. 生成 PPT
	pptPath, err := ctrl.pptService.GeneratePPT(req.WorkflowID, outline, wf.LessonPlan)
	if err != nil {
		ctrl.workflowMgr.SetError(req.WorkflowID, err.Error())
		sendWorkflowSSE(c, WorkflowSSEEvent{Type: "error", Content: "PPT 生成失败: " + err.Error()})
		return
	}

	// 5. 保存 PPT 路径
	if err := ctrl.workflowMgr.SavePPTPath(req.WorkflowID, pptPath); err != nil {
		log.Printf("保存 PPT 路径失败: %v", err)
	}

	sendWorkflowSSE(c, WorkflowSSEEvent{Type: "status", Content: "PPT 生成完成！"})

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type: "ppt",
		Data: gin.H{
			"path":         pptPath,
			"download_url": fmt.Sprintf("/api/lessonprep/ppt/download?workflow_id=%s", req.WorkflowID),
		},
	})

	sendWorkflowSSE(c, WorkflowSSEEvent{
		Type:       "done",
		Content:    "备课全流程完成！大纲、教案、PPT 均已生成。",
		WorkflowID: req.WorkflowID,
	})
}

// DownloadPPT 下载 PPT 文件
func (ctrl *LessonPrepController) DownloadPPT(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	wf, err := ctrl.workflowMgr.GetWorkflow(workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if wf.PPTPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "PPT 尚未生成"})
		return
	}

	if _, err := os.Stat(wf.PPTPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "PPT 文件不存在"})
		return
	}

	fileName := filepath.Base(wf.PPTPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
	c.File(wf.PPTPath)
}

// GetStatus 获取工作流状态
func (ctrl *LessonPrepController) GetStatus(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	wf, err := ctrl.workflowMgr.GetWorkflow(workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	result := gin.H{
		"workflow_id": wf.ID,
		"status":      wf.Status,
		"topic":       wf.Topic,
		"subject":     wf.Subject,
		"grade":       wf.Grade,
		"duration":    wf.Duration,
		"created_at":  wf.CreatedAt,
		"updated_at":  wf.UpdatedAt,
	}

	if wf.OutlineJSON != "" {
		var outline models.CourseOutline
		if json.Unmarshal([]byte(wf.OutlineJSON), &outline) == nil {
			result["outline"] = outline
		}
	}
	if wf.LessonPlan != "" {
		result["has_plan"] = true
	}
	if wf.PPTPath != "" {
		result["has_ppt"] = true
		result["ppt_download_url"] = fmt.Sprintf("/api/lessonprep/ppt/download?workflow_id=%s", wf.ID)
	}
	if wf.ErrorMsg != "" {
		result["error_msg"] = wf.ErrorMsg
	}

	c.JSON(http.StatusOK, result)
}

// ListWorkflows 列出工作流
func (ctrl *LessonPrepController) ListWorkflows(c *gin.Context) {
	// TODO: 从 token 获取用户 ID
	workflows, err := ctrl.workflowMgr.ListWorkflows(0, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
	})
}

// GetOutline 获取大纲详情
func (ctrl *LessonPrepController) GetOutline(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	outline, err := ctrl.workflowMgr.GetOutline(workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id": workflowID,
		"outline":     outline,
	})
}

// GetPlan 获取教案详情
func (ctrl *LessonPrepController) GetPlan(c *gin.Context) {
	workflowID := c.Query("workflow_id")
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 workflow_id"})
		return
	}

	wf, err := ctrl.workflowMgr.GetWorkflow(workflowID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if wf.LessonPlan == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "教案尚未生成"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow_id":  workflowID,
		"lesson_plan":  wf.LessonPlan,
	})
}
