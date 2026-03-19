package nlp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hock1024always/GoEdu/services/llm"
)

// LLMCoreferenceResolver LLM增强的指代消解器
type LLMCoreferenceResolver struct {
	ruleResolver *CoreferenceResolver
	llmClient    *llm.Client
}

// LLMResolveResult LLM消解结果
type LLMResolveResult struct {
	ResolvedQuery string            `json:"resolved_query"`
	Entities      map[string]string `json:"entities"` // 代词 -> 实体
	Reasoning     string            `json:"reasoning"`
}

// NewLLMCoreferenceResolver 创建LLM指代消解器
func NewLLMCoreferenceResolver() *LLMCoreferenceResolver {
	return &LLMCoreferenceResolver{
		ruleResolver: NewCoreferenceResolver(),
		llmClient:    llm.NewClient(),
	}
}

// Resolve 混合指代消解（规则优先，复杂情况使用LLM）
func (r *LLMCoreferenceResolver) Resolve(query string, context []string, turn int) (*ResolveResult, error) {
	// 1. 先用规则方法处理
	ruleResult := r.ruleResolver.Resolve(query, turn)

	// 2. 如果规则方法置信度高，直接返回
	if !ruleResult.NeedsLLM && ruleResult.Confidence >= 0.85 {
		return ruleResult, nil
	}

	// 3. 使用LLM处理复杂情况
	llmResult, err := r.resolveLLM(query, context)
	if err != nil {
		// LLM失败，降级到规则结果
		return ruleResult, nil
	}

	// 4. 合并结果
	finalResult := &ResolveResult{
		OriginalQuery: query,
		ResolvedQuery: llmResult.ResolvedQuery,
		Replacements:  make([]Replacement, 0),
		Confidence:    0.95, // LLM结果置信度较高
		NeedsLLM:      false,
	}

	for pronoun, entity := range llmResult.Entities {
		finalResult.Replacements = append(finalResult.Replacements, Replacement{
			Original: pronoun,
			Resolved: entity,
			Type:     "llm",
		})
	}

	return finalResult, nil
}

// resolveLLM 使用LLM进行指代消解
func (r *LLMCoreferenceResolver) resolveLLM(query string, context []string) (*LLMResolveResult, error) {
	// 构建上下文字符串
	contextStr := ""
	if len(context) > 0 {
		// 只取最近5轮对话
		start := 0
		if len(context) > 10 {
			start = len(context) - 10
		}
		contextStr = strings.Join(context[start:], "\n")
	}

	prompt := fmt.Sprintf(`你是一个专业的中文指代消解专家。请分析用户的当前问题，根据对话历史，将其中的代词（如"他"、"她"、"这个"等）替换为具体的实体名称。

对话历史：
%s

当前问题：%s

请按以下JSON格式输出结果：
{
  "resolved_query": "替换代词后的完整问题",
  "entities": {
    "代词1": "对应的实体",
    "代词2": "对应的实体"
  },
  "reasoning": "简要说明你的推理过程"
}

注意：
1. 如果问题中没有需要消解的代词，resolved_query保持原样，entities为空对象
2. 确保替换后的问题语义完整、通顺
3. 如果无法确定代词指代的实体，保留原代词
4. 只输出JSON，不要其他内容`, contextStr, query)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := r.llmClient.Chat(messages, nil)
	if err != nil {
		return nil, err
	}

	// 解析LLM响应
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	content := resp.Choices[0].Message.Content
	
	// 提取JSON部分
	content = extractJSON(content)

	var result LLMResolveResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// 如果解析失败，返回原查询
		return &LLMResolveResult{
			ResolvedQuery: query,
			Entities:      make(map[string]string),
			Reasoning:     "JSON解析失败",
		}, nil
	}

	return &result, nil
}

// extractJSON 从文本中提取JSON
func extractJSON(text string) string {
	// 查找JSON开始和结束位置
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	
	if start == -1 || end == -1 || start >= end {
		return text
	}
	
	return text[start : end+1]
}

// AddContext 添加对话历史上下文
func (r *LLMCoreferenceResolver) AddContext(entities []Entity, turn int) {
	r.ruleResolver.AddContext(entities, turn)
}

// Clear 清空上下文
func (r *LLMCoreferenceResolver) Clear() {
	r.ruleResolver.Clear()
}

// GetRuleResolver 获取规则消解器（用于直接调用规则方法）
func (r *LLMCoreferenceResolver) GetRuleResolver() *CoreferenceResolver {
	return r.ruleResolver
}
