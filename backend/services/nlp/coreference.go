package nlp

import (
	"regexp"
	"strings"
	"sync"
)

// CoreferenceResolver 指代消解器
type CoreferenceResolver struct {
	jieba        *JiebaService
	entityStack  *EntityStack // 实体栈，记录最近提到的实体
	pronounMap   map[string][]string // 代词到实体类型的映射
	mutex        sync.RWMutex
}

// EntityStack 实体栈（用于跟踪对话中提到的实体）
type EntityStack struct {
	persons  []EntityMention // 人物
	states   []EntityMention // 国家
	events   []EntityMention // 事件
	times    []EntityMention // 时间
	maxSize  int
}

// EntityMention 实体提及
type EntityMention struct {
	Text      string  // 实体文本
	Type      string  // 实体类型
	Turn      int     // 对话轮次
	Position  int     // 在句子中的位置
	Frequency int     // 提及次数
}

// ResolveResult 消解结果
type ResolveResult struct {
	OriginalQuery  string            `json:"original_query"`
	ResolvedQuery  string            `json:"resolved_query"`
	Replacements   []Replacement     `json:"replacements"`
	Confidence     float64           `json:"confidence"`
	NeedsLLM       bool              `json:"needs_llm"` // 是否需要LLM进一步处理
}

// Replacement 替换记录
type Replacement struct {
	Original  string `json:"original"`
	Resolved  string `json:"resolved"`
	Type      string `json:"type"` // pronoun, ellipsis, implicit
}

// NewCoreferenceResolver 创建指代消解器
func NewCoreferenceResolver() *CoreferenceResolver {
	return &CoreferenceResolver{
		jieba:       GetJiebaService(),
		entityStack: NewEntityStack(10),
		pronounMap:  initPronounMap(),
	}
}

// NewEntityStack 创建实体栈
func NewEntityStack(maxSize int) *EntityStack {
	return &EntityStack{
		persons:  make([]EntityMention, 0),
		states:   make([]EntityMention, 0),
		events:   make([]EntityMention, 0),
		times:    make([]EntityMention, 0),
		maxSize:  maxSize,
	}
}

// initPronounMap 初始化代词映射
func initPronounMap() map[string][]string {
	return map[string][]string{
		// 人称代词 -> 优先匹配人物
		"他":   {"person"},
		"她":   {"person"},
		"他们": {"person"},
		"她们": {"person"},
		"此人": {"person"},
		"该人": {"person"},
		"这位": {"person"},
		"那位": {"person"},
		"其":   {"person", "state"}, // "其"可以指人也可以指国家
		
		// 指示代词 -> 根据上下文
		"这个": {"person", "event", "state"},
		"那个": {"person", "event", "state"},
		"此":   {"person", "event", "state"},
		"该":   {"person", "event", "state"},
		
		// 国家相关代词
		"该国":   {"state"},
		"此国":   {"state"},
		"这个国家": {"state"},
		"那个国家": {"state"},
		
		// 事件相关代词
		"这场战争": {"event"},
		"那场战争": {"event"},
		"这件事":   {"event"},
		"那件事":   {"event"},
		"此事":     {"event"},
		"该事件":   {"event"},
		
		// 时间相关
		"这个时期": {"time"},
		"那个时期": {"time"},
		"当时":     {"time"},
		"那时":     {"time"},
		"此时":     {"time"},
	}
}

// Resolve 执行指代消解
func (r *CoreferenceResolver) Resolve(query string, turn int) *ResolveResult {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	result := &ResolveResult{
		OriginalQuery: query,
		ResolvedQuery: query,
		Replacements:  make([]Replacement, 0),
		Confidence:    1.0,
	}

	// 1. 提取当前查询中的实体，更新实体栈
	entities := r.jieba.ExtractEntities(query)
	r.updateEntityStack(entities, turn)

	// 2. 检测并替换代词
	result = r.resolvePronoun(result, turn)

	// 3. 检测并处理省略（主语省略）
	result = r.resolveEllipsis(result, turn)

	// 4. 检测是否需要LLM处理复杂情况
	result.NeedsLLM = r.needsLLMResolution(result)

	return result
}

// updateEntityStack 更新实体栈
func (r *CoreferenceResolver) updateEntityStack(entities []Entity, turn int) {
	for i, entity := range entities {
		mention := EntityMention{
			Text:     entity.Text,
			Type:     string(entity.Type),
			Turn:     turn,
			Position: i,
		}

		switch entity.Type {
		case EntityTypePerson:
			r.entityStack.pushPerson(mention)
		case EntityTypeState:
			r.entityStack.pushState(mention)
		case EntityTypeEvent:
			r.entityStack.pushEvent(mention)
		case EntityTypeTime:
			r.entityStack.pushTime(mention)
		}
	}
}

// resolvePronoun 解析代词
func (r *CoreferenceResolver) resolvePronoun(result *ResolveResult, turn int) *ResolveResult {
	query := result.ResolvedQuery

	for pronoun, types := range r.pronounMap {
		if !strings.Contains(query, pronoun) {
			continue
		}

		// 根据类型优先级查找最近的实体
		var replacement string
		for _, entityType := range types {
			switch entityType {
			case "person":
				if entity := r.entityStack.getLatestPerson(turn); entity != nil {
					replacement = entity.Text
				}
			case "state":
				if entity := r.entityStack.getLatestState(turn); entity != nil {
					replacement = entity.Text
				}
			case "event":
				if entity := r.entityStack.getLatestEvent(turn); entity != nil {
					replacement = entity.Text
				}
			case "time":
				if entity := r.entityStack.getLatestTime(turn); entity != nil {
					replacement = entity.Text
				}
			}
			if replacement != "" {
				break
			}
		}

		if replacement != "" {
			// 执行替换
			query = strings.Replace(query, pronoun, replacement, 1)
			result.Replacements = append(result.Replacements, Replacement{
				Original: pronoun,
				Resolved: replacement,
				Type:     "pronoun",
			})
			// 降低置信度（因为进行了推断）
			result.Confidence *= 0.9
		}
	}

	result.ResolvedQuery = query
	return result
}

// resolveEllipsis 解析省略（主语省略等）
func (r *CoreferenceResolver) resolveEllipsis(result *ResolveResult, turn int) *ResolveResult {
	query := result.ResolvedQuery

	// 检测常见的省略模式
	ellipsisPatterns := []struct {
		pattern     *regexp.Regexp
		entityType  string
		prefix      string // 在查询前添加
	}{
		// "和xxx比呢？" -> 需要补全比较对象
		{regexp.MustCompile(`^和(.+)(比|相比|比较)`), "person", ""},
		// "是怎么xxx的？" -> 需要补全主语
		{regexp.MustCompile(`^是(怎么|如何)`), "person", ""},
		// "为什么xxx？" -> 可能需要补全主语
		{regexp.MustCompile(`^为什么`), "person", ""},
		// "什么时候xxx？" -> 可能需要补全主语
		{regexp.MustCompile(`^什么时候`), "person", ""},
		// "后来呢？" -> 需要补全主语
		{regexp.MustCompile(`^后来(呢|怎么样|如何)`), "person", ""},
		// "然后呢？" -> 需要补全主语
		{regexp.MustCompile(`^然后(呢)?$`), "person", ""},
	}

	for _, ep := range ellipsisPatterns {
		if ep.pattern.MatchString(query) {
			var subject string
			switch ep.entityType {
			case "person":
				if entity := r.entityStack.getLatestPerson(turn); entity != nil {
					subject = entity.Text
				}
			case "state":
				if entity := r.entityStack.getLatestState(turn); entity != nil {
					subject = entity.Text
				}
			case "event":
				if entity := r.entityStack.getLatestEvent(turn); entity != nil {
					subject = entity.Text
				}
			}

			if subject != "" {
				// 在句首补全主语
				newQuery := subject + query
				result.Replacements = append(result.Replacements, Replacement{
					Original: "(省略)",
					Resolved: subject,
					Type:     "ellipsis",
				})
				result.ResolvedQuery = newQuery
				result.Confidence *= 0.85
				break
			}
		}
	}

	return result
}

// needsLLMResolution 判断是否需要LLM处理
func (r *CoreferenceResolver) needsLLMResolution(result *ResolveResult) bool {
	// 如果置信度低于阈值，建议使用LLM
	if result.Confidence < 0.7 {
		return true
	}

	// 如果有多个代词需要解析
	if len(result.Replacements) > 2 {
		return true
	}

	// 检测复杂指代模式
	complexPatterns := []string{
		"他们之间",    // 多人关系
		"两者",       // 比较关系
		"前者", "后者", // 对比关系
		"彼此",       // 相互关系
	}
	for _, pattern := range complexPatterns {
		if strings.Contains(result.OriginalQuery, pattern) {
			return true
		}
	}

	return false
}

// Clear 清空实体栈（新对话时调用）
func (r *CoreferenceResolver) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.entityStack = NewEntityStack(10)
}

// AddContext 手动添加上下文实体（用于从对话历史恢复）
func (r *CoreferenceResolver) AddContext(entities []Entity, turn int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.updateEntityStack(entities, turn)
}

// --- EntityStack 方法 ---

func (s *EntityStack) pushPerson(mention EntityMention) {
	// 检查是否已存在，如果存在则增加频率
	for i := range s.persons {
		if s.persons[i].Text == mention.Text {
			s.persons[i].Frequency++
			s.persons[i].Turn = mention.Turn
			// 移到最前面
			if i > 0 {
				s.persons = append([]EntityMention{s.persons[i]}, append(s.persons[:i], s.persons[i+1:]...)...)
			}
			return
		}
	}
	// 新实体，添加到栈顶
	s.persons = append([]EntityMention{mention}, s.persons...)
	if len(s.persons) > s.maxSize {
		s.persons = s.persons[:s.maxSize]
	}
}

func (s *EntityStack) pushState(mention EntityMention) {
	for i := range s.states {
		if s.states[i].Text == mention.Text {
			s.states[i].Frequency++
			s.states[i].Turn = mention.Turn
			if i > 0 {
				s.states = append([]EntityMention{s.states[i]}, append(s.states[:i], s.states[i+1:]...)...)
			}
			return
		}
	}
	s.states = append([]EntityMention{mention}, s.states...)
	if len(s.states) > s.maxSize {
		s.states = s.states[:s.maxSize]
	}
}

func (s *EntityStack) pushEvent(mention EntityMention) {
	for i := range s.events {
		if s.events[i].Text == mention.Text {
			s.events[i].Frequency++
			s.events[i].Turn = mention.Turn
			if i > 0 {
				s.events = append([]EntityMention{s.events[i]}, append(s.events[:i], s.events[i+1:]...)...)
			}
			return
		}
	}
	s.events = append([]EntityMention{mention}, s.events...)
	if len(s.events) > s.maxSize {
		s.events = s.events[:s.maxSize]
	}
}

func (s *EntityStack) pushTime(mention EntityMention) {
	for i := range s.times {
		if s.times[i].Text == mention.Text {
			s.times[i].Frequency++
			s.times[i].Turn = mention.Turn
			if i > 0 {
				s.times = append([]EntityMention{s.times[i]}, append(s.times[:i], s.times[i+1:]...)...)
			}
			return
		}
	}
	s.times = append([]EntityMention{mention}, s.times...)
	if len(s.times) > s.maxSize {
		s.times = s.times[:s.maxSize]
	}
}

func (s *EntityStack) getLatestPerson(currentTurn int) *EntityMention {
	if len(s.persons) == 0 {
		return nil
	}
	// 优先返回最近轮次的实体，但不能是当前轮次（避免自引用）
	for i := range s.persons {
		if s.persons[i].Turn < currentTurn {
			return &s.persons[i]
		}
	}
	return nil
}

func (s *EntityStack) getLatestState(currentTurn int) *EntityMention {
	if len(s.states) == 0 {
		return nil
	}
	for i := range s.states {
		if s.states[i].Turn < currentTurn {
			return &s.states[i]
		}
	}
	return nil
}

func (s *EntityStack) getLatestEvent(currentTurn int) *EntityMention {
	if len(s.events) == 0 {
		return nil
	}
	for i := range s.events {
		if s.events[i].Turn < currentTurn {
			return &s.events[i]
		}
	}
	return nil
}

func (s *EntityStack) getLatestTime(currentTurn int) *EntityMention {
	if len(s.times) == 0 {
		return nil
	}
	for i := range s.times {
		if s.times[i].Turn < currentTurn {
			return &s.times[i]
		}
	}
	return nil
}

// GetAllEntities 获取所有实体（用于调试）
func (s *EntityStack) GetAllEntities() map[string][]EntityMention {
	return map[string][]EntityMention{
		"persons": s.persons,
		"states":  s.states,
		"events":  s.events,
		"times":   s.times,
	}
}
