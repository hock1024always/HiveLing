package lessonprep

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hock1024always/GoEdu/models"
	"github.com/hock1024always/GoEdu/services/llm"
	"github.com/hock1024always/GoEdu/services/rag"
)

// LessonPlanService 教案生成服务
type LessonPlanService struct {
	llmClient *llm.Client
	retriever *rag.Retriever
}

// NewLessonPlanService 创建教案生成服务
func NewLessonPlanService() *LessonPlanService {
	return &LessonPlanService{
		llmClient: llm.NewClient(),
		retriever: rag.NewRetriever(),
	}
}

// GenerateLessonPlan 基于大纲生成教案（非流式）
func (s *LessonPlanService) GenerateLessonPlan(outline *models.CourseOutline) (string, error) {
	ragContext := s.retrieveContext(outline.Title + " " + outline.Subject)
	prompt := s.buildLessonPlanPrompt(outline, ragContext)

	resp, err := s.llmClient.Chat([]llm.Message{
		{Role: "system", Content: lessonPlanSystemPrompt},
		{Role: "user", Content: prompt},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("LLM 调用失败: %v", err)
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateLessonPlanStream 流式生成教案
func (s *LessonPlanService) GenerateLessonPlanStream(outline *models.CourseOutline, onChunk func(text string) error) (string, error) {
	ragContext := s.retrieveContext(outline.Title + " " + outline.Subject)
	prompt := s.buildLessonPlanPrompt(outline, ragContext)

	var fullContent strings.Builder

	_, err := s.llmClient.StreamChat([]llm.Message{
		{Role: "system", Content: lessonPlanSystemPrompt},
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
		return "", fmt.Errorf("LLM 流式调用失败: %v", err)
	}

	return fullContent.String(), nil
}

// retrieveContext RAG 检索
func (s *LessonPlanService) retrieveContext(query string) string {
	results, err := s.retriever.Search(query, "all", 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	return rag.BuildContext(results, 4000)
}

// buildLessonPlanPrompt 构建教案生成 Prompt
func (s *LessonPlanService) buildLessonPlanPrompt(outline *models.CourseOutline, ragContext string) string {
	outlineJSON, _ := json.MarshalIndent(outline, "", "  ")

	var sb strings.Builder
	sb.WriteString("请基于以下课程大纲，编写详细的教案。\n\n")
	sb.WriteString("## 课程大纲\n")
	sb.WriteString("```json\n")
	sb.WriteString(string(outlineJSON))
	sb.WriteString("\n```\n\n")

	if ragContext != "" {
		sb.WriteString("## 参考资料（来自知识库）\n")
		sb.WriteString(ragContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString(`## 教案要求

请按以下格式编写教案（Markdown 格式）：

# 《课程标题》教案

## 一、基本信息
- 学科：
- 年级：
- 课时：
- 课型：

## 二、教学目标
### 知识与技能
### 过程与方法
### 情感态度价值观

## 三、教学重难点
### 教学重点
### 教学难点

## 四、教学方法

## 五、教学过程

### （一）导入新课（X分钟）
#### 教师活动
（具体话术和操作）
#### 学生活动
（预期反应和互动方式）
#### 设计意图

### （二）新课讲授（X分钟）
#### 教师活动
#### 学生活动
#### 设计意图

### （三）课堂练习（X分钟）
#### 教师活动
#### 学生活动
#### 设计意图

### （四）课堂小结（X分钟）
#### 教师活动
#### 学生活动

### （五）布置作业（X分钟）

## 六、板书设计

## 七、教学反思（留白）

注意事项：
1. 教师活动要包含具体的教学话术
2. 要融入春秋战国时期的史料原文引用
3. 包含至少 2 个师生互动环节
4. 内容要准确，符合史实
5. 语言适合课堂教学，深入浅出`)

	return sb.String()
}

const lessonPlanSystemPrompt = `你是一位拥有20年教龄的春秋战国历史教学名师，擅长编写详细、生动、实用的教案。

你的教案特点：
1. 教师话术自然流畅，能引发学生思考
2. 善于运用史料原文（如《史记》《战国策》《左传》等）辅助教学
3. 注重师生互动，设计启发式问题
4. 教学环节过渡自然，逻辑清晰
5. 兼顾历史知识传授和历史思维培养

请用 Markdown 格式输出完整教案。`
