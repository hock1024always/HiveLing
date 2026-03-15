package lessonprep

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
)

// OutlineEditService 大纲修改服务（对话式修改）
type OutlineEditService struct {
	llmClient *llm.Client
}

// NewOutlineEditService 创建大纲修改服务
func NewOutlineEditService() *OutlineEditService {
	return &OutlineEditService{
		llmClient: llm.NewClient(),
	}
}

// EditOutline 对话式修改大纲
// currentOutline: 当前大纲
// editRequest: 用户的修改意见（自然语言）
// history: 历史对话
func (s *OutlineEditService) EditOutline(currentOutline *models.CourseOutline, editRequest string, history []models.WorkflowMessage) (*models.CourseOutline, string, error) {
	// 构建对话上下文
	messages := s.buildEditMessages(currentOutline, editRequest, history)

	// 调用 LLM
	resp, err := s.llmClient.Chat(messages, nil)
	if err != nil {
		return nil, "", fmt.Errorf("LLM 调用失败: %v", err)
	}

	content := resp.Choices[0].Message.Content

	// 解析修改后的大纲
	outline, err := s.parseEditedOutline(content)
	if err != nil {
		return nil, content, fmt.Errorf("解析修改后的大纲失败: %v", err)
	}

	return outline, content, nil
}

// EditOutlineStream 流式对话修改大纲
func (s *OutlineEditService) EditOutlineStream(currentOutline *models.CourseOutline, editRequest string, history []models.WorkflowMessage, onChunk func(text string) error) (*models.CourseOutline, error) {
	messages := s.buildEditMessages(currentOutline, editRequest, history)

	var fullContent strings.Builder

	_, err := s.llmClient.StreamChat(messages, nil, func(chunk *llm.StreamChunk) error {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			text := chunk.Choices[0].Delta.Content
			fullContent.WriteString(text)
			if onChunk != nil {
				return onChunk(text)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 流式调用失败: %v", err)
	}

	outline, err := s.parseEditedOutline(fullContent.String())
	if err != nil {
		return nil, fmt.Errorf("解析修改后的大纲失败: %v", err)
	}

	return outline, nil
}

// buildEditMessages 构建修改对话的消息列表
func (s *OutlineEditService) buildEditMessages(currentOutline *models.CourseOutline, editRequest string, history []models.WorkflowMessage) []llm.Message {
	messages := []llm.Message{
		{Role: "system", Content: outlineEditSystemPrompt},
	}

	// 添加当前大纲作为上下文
	outlineJSON, _ := json.MarshalIndent(currentOutline, "", "  ")
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("以下是当前的课程大纲：\n```json\n%s\n```", string(outlineJSON)),
	})
	messages = append(messages, llm.Message{
		Role:    "assistant",
		Content: "好的，我已经了解当前大纲的内容。请告诉我您需要修改的地方。",
	})

	// 添加历史对话（最近的修改记录）
	for _, msg := range history {
		if msg.Stage == "outline" && msg.Role != "system" {
			messages = append(messages, llm.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// 添加本次修改请求
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("请根据以下修改意见修改大纲，输出修改后的完整 JSON：\n%s", editRequest),
	})

	return messages
}

// parseEditedOutline 解析修改后的大纲
func (s *OutlineEditService) parseEditedOutline(content string) (*models.CourseOutline, error) {
	var outline models.CourseOutline

	// 尝试直接解析
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &outline); err == nil {
		return &outline, nil
	}

	// 从 markdown 代码块提取
	jsonStr := extractJSONFromMarkdown(content)
	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &outline); err == nil {
			return &outline, nil
		}
	}

	// 提取 JSON 片段
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		jsonStr = content[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &outline); err == nil {
			return &outline, nil
		}
	}

	return nil, fmt.Errorf("无法从响应中解析出大纲 JSON")
}

const outlineEditSystemPrompt = `你是一位资深的春秋战国历史教学专家，用户正在修改课程大纲，你需要：

1. 理解用户的修改意见
2. 在当前大纲的基础上进行精确修改
3. 只修改用户提出的部分，保持其他部分不变
4. 确保修改后的大纲内容准确、结构完整

输出要求：
- 先简要说明你做了哪些修改（1-2句话）
- 然后在 ` + "```json```" + ` 代码块中输出修改后的完整 JSON 大纲
- JSON 格式与原大纲保持一致`
