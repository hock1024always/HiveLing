package lessonprep

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/rag"
)

// OutlineService 大纲生成服务
type OutlineService struct {
	llmClient *llm.Client
	retriever *rag.Retriever
}

// NewOutlineService 创建大纲服务
func NewOutlineService() *OutlineService {
	return &OutlineService{
		llmClient: llm.NewClient(),
		retriever: rag.NewRetriever(),
	}
}

// GenerateOutline 生成课程大纲（非流式，返回结构化 JSON）
func (s *OutlineService) GenerateOutline(topic, subject, grade string, duration int) (*models.CourseOutline, string, error) {
	// 1. RAG 检索相关知识
	ragContext := s.retrieveContext(topic)

	// 2. 构建 prompt
	prompt := s.buildOutlinePrompt(topic, subject, grade, duration, ragContext)

	// 3. 调用 LLM
	resp, err := s.llmClient.Chat([]llm.Message{
		{Role: "system", Content: outlineSystemPrompt},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return nil, "", fmt.Errorf("LLM 调用失败: %v", err)
	}

	content := resp.Choices[0].Message.Content

	// 4. 解析 JSON
	outline, err := s.parseOutlineJSON(content)
	if err != nil {
		return nil, content, fmt.Errorf("大纲解析失败: %v", err)
	}

	// 补全基本信息
	if outline.Subject == "" {
		outline.Subject = subject
	}
	if outline.Grade == "" {
		outline.Grade = grade
	}
	if outline.Duration == 0 {
		outline.Duration = duration
	}

	return outline, content, nil
}

// GenerateOutlineStream 流式生成大纲
func (s *OutlineService) GenerateOutlineStream(topic, subject, grade string, duration int, onChunk func(text string) error) (*models.CourseOutline, error) {
	ragContext := s.retrieveContext(topic)
	prompt := s.buildOutlinePrompt(topic, subject, grade, duration, ragContext)

	var fullContent strings.Builder

	_, err := s.llmClient.StreamChat([]llm.Message{
		{Role: "system", Content: outlineSystemPrompt},
		{Role: "user", Content: prompt},
	}, nil, func(chunk *llm.StreamChunk) error {
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

	outline, err := s.parseOutlineJSON(fullContent.String())
	if err != nil {
		return nil, fmt.Errorf("大纲解析失败: %v (原始内容: %s)", err, fullContent.String()[:min(200, fullContent.Len())])
	}

	if outline.Subject == "" {
		outline.Subject = subject
	}
	if outline.Grade == "" {
		outline.Grade = grade
	}
	if outline.Duration == 0 {
		outline.Duration = duration
	}

	return outline, nil
}

// retrieveContext RAG 检索相关资料
func (s *OutlineService) retrieveContext(topic string) string {
	results, err := s.retriever.Search(topic, "all", 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	return rag.BuildContext(results, 3000)
}

// buildOutlinePrompt 构建大纲生成 Prompt
func (s *OutlineService) buildOutlinePrompt(topic, subject, grade string, duration int, ragContext string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`请为 %s 学生设计一节 %s 课，主题是「%s」，课时 %d 分钟。`, grade, subject, topic, duration))
	sb.WriteString("\n\n")

	if ragContext != "" {
		sb.WriteString("以下是来自知识库的参考资料，请结合使用：\n")
		sb.WriteString(ragContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`请严格按照以下 JSON 格式输出课程大纲，不要包含任何其他内容，只输出 JSON：
{
    "title": "课程标题",
    "subject": "学科",
    "grade": "年级",
    "duration": 课时分钟数,
    "objectives": {
        "knowledge": "知识与技能目标",
        "ability": "过程与方法目标",
        "emotion": "情感态度价值观目标"
    },
    "key_points": ["重点1", "重点2"],
    "difficult_points": ["难点1", "难点2"],
    "teaching_methods": ["讲授法", "讨论法", "史料分析法"],
    "process": [
        {"stage": "导入", "duration": 5, "activity": "教学活动描述", "method": "使用的方法"},
        {"stage": "新授", "duration": 25, "activity": "教学活动描述", "method": "使用的方法"},
        {"stage": "练习", "duration": 8, "activity": "教学活动描述", "method": "使用的方法"},
        {"stage": "小结", "duration": 5, "activity": "教学活动描述", "method": "使用的方法"},
        {"stage": "作业", "duration": 2, "activity": "布置的作业内容", "method": ""}
    ],
    "board_design": "板书设计内容"
}`)

	return sb.String()
}

// parseOutlineJSON 从 LLM 返回内容中提取并解析大纲 JSON
func (s *OutlineService) parseOutlineJSON(content string) (*models.CourseOutline, error) {
	// 尝试直接解析
	var outline models.CourseOutline
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &outline); err == nil {
		return &outline, nil
	}

	// 尝试从 markdown code block 中提取
	jsonStr := extractJSONFromMarkdown(content)
	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &outline); err == nil {
			return &outline, nil
		}
	}

	// 尝试找到第一个 { 和最后一个 }
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		jsonStr = content[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &outline); err == nil {
			return &outline, nil
		}
	}

	return nil, fmt.Errorf("无法从 LLM 响应中解析 JSON")
}

// extractJSONFromMarkdown 从 markdown 代码块中提取 JSON
func extractJSONFromMarkdown(content string) string {
	// 查找 ```json ... ``` 或 ``` ... ```
	markers := []string{"```json", "```"}
	for _, marker := range markers {
		start := strings.Index(content, marker)
		if start < 0 {
			continue
		}
		start += len(marker)
		end := strings.Index(content[start:], "```")
		if end < 0 {
			continue
		}
		return strings.TrimSpace(content[start : start+end])
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// outlineSystemPrompt 大纲生成的系统提示
const outlineSystemPrompt = `你是一位资深的春秋战国历史教学专家，擅长设计课程大纲。你需要：

1. 基于用户指定的主题、学科、年级和课时，设计结构完整的课程大纲
2. 内容要贴合春秋战国时期（公元前770年-公元前221年）的史实
3. 教学目标要明确、可衡量
4. 教学过程环节完整：导入→新授→练习→小结→作业
5. 每个环节的时间分配合理，总时间不超过课时

输出要求：只输出 JSON 格式，不要包含任何解释性文字。`
