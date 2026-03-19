package nlp

import (
	"regexp"
	"strings"
	"sync"

	"github.com/yanyiwu/gojieba"
)

// JiebaService jieba中文分词服务
type JiebaService struct {
	extractor *gojieba.Jieba
	mutex     sync.RWMutex
}

var (
	jiebaInstance *JiebaService
	jiebaOnce     sync.Once
)

// GetJiebaService 获取单例
func GetJiebaService() *JiebaService {
	jiebaOnce.Do(func() {
		jiebaInstance = &JiebaService{
			extractor: gojieba.NewJieba(),
		}
	})
	return jiebaInstance
}

// Close 释放资源
func (s *JiebaService) Close() {
	if s.extractor != nil {
		s.extractor.Free()
	}
}

// Segment 分词
func (s *JiebaService) Segment(text string) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.extractor == nil {
		return strings.Fields(text)
	}

	// 使用精确模式分词
	words := s.extractor.Cut(text, true)
	return filterEmpty(words)
}

// SegmentForSearch 搜索引擎模式分词（更细粒度）
func (s *JiebaService) SegmentForSearch(text string) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.extractor == nil {
		return strings.Fields(text)
	}

	// 使用搜索引擎模式，适合检索场景
	words := s.extractor.CutForSearch(text, true)
	return filterEmpty(words)
}

// ExtractKeywords 提取关键词（TF-IDF算法）
func (s *JiebaService) ExtractKeywords(text string, topK int) []Keyword {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.extractor == nil || text == "" {
		return nil
	}

	// 使用TF-IDF提取关键词
	wordWeights := s.extractor.ExtractWithWeight(text, topK)

	keywords := make([]Keyword, 0, len(wordWeights))
	for _, ww := range wordWeights {
		keywords = append(keywords, Keyword{
			Word:   ww.Word,
			Weight: ww.Weight,
		})
	}
	return keywords
}

// ExtractEntities 提取实体（基于词典和规则）
func (s *JiebaService) ExtractEntities(text string) []Entity {
	entities := make([]Entity, 0)

	// 使用自定义词典匹配历史实体
	words := s.Segment(text)
	wordSet := make(map[string]bool)
	for _, w := range words {
		wordSet[w] = true
	}

	// 匹配人名
	for person := range HistoryPersonDict {
		if strings.Contains(text, person) || wordSet[person] {
			entities = append(entities, Entity{
				Text:     person,
				Type:     EntityTypePerson,
				Category: "历史人物",
			})
		}
	}

	// 匹配地名（国家）
	for state := range HistoryStateDict {
		if strings.Contains(text, state) || wordSet[state] {
			entities = append(entities, Entity{
				Text:     state,
				Type:     EntityTypeState,
				Category: "诸侯国",
			})
		}
	}

	// 匹配事件
	for event := range HistoryEventDict {
		if strings.Contains(text, event) {
			entities = append(entities, Entity{
				Text:     event,
				Type:     EntityTypeEvent,
				Category: "历史事件",
			})
		}
	}

	// 正则匹配年份
	yearEntities := extractYears(text)
	entities = append(entities, yearEntities...)

	return deduplicateEntities(entities)
}

// AddCustomWord 添加自定义词
func (s *JiebaService) AddCustomWord(words ...string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.extractor != nil {
		for _, word := range words {
			s.extractor.AddWord(word)
		}
	}
}

// AddCustomWordsFromDict 从词典批量添加
func (s *JiebaService) AddCustomWordsFromDict(dict map[string]string) {
	words := make([]string, 0, len(dict))
	for word := range dict {
		words = append(words, word)
	}
	s.AddCustomWord(words...)
}

// 数据结构定义

type Keyword struct {
	Word   string
	Weight float64
}

type EntityType string

const (
	EntityTypePerson EntityType = "person" // 人物
	EntityTypeState  EntityType = "state"  // 国家/地区
	EntityTypeEvent  EntityType = "event"  // 事件
	EntityTypeTime   EntityType = "time"   // 时间
	EntityTypePlace  EntityType = "place"  // 地点
)

type Entity struct {
	Text     string     `json:"text"`
	Type     EntityType `json:"type"`
	Category string     `json:"category"`
}

// 历史实体词典（初始化时加载）
var (
	HistoryPersonDict = map[string]string{
		"齐桓公": "春秋五霸之首",
		"晋文公": "春秋五霸之一",
		"楚庄王": "春秋五霸之一",
		"秦穆公": "春秋五霸之一",
		"宋襄公": "春秋五霸之一",
		"孔子":   "儒家创始人",
		"老子":   "道家创始人",
		"墨子":   "墨家创始人",
		"孟子":   "儒家代表",
		"荀子":   "儒家代表",
		"管仲":   "齐国宰相",
		"鲍叔牙":  "齐国大夫",
		"晏婴":   "齐国宰相",
		"伍子胥": "吴国大夫",
		"范蠡":   "越国大夫",
		"勾践":   "越王",
		"夫差":   "吴王",
		"阖闾":   "吴王",
		"孙武":   "兵圣",
		"孙膑":   "军事家",
		"商鞅":   "变法家",
		"苏秦":   "纵横家",
		"张仪":   "纵横家",
		"白起":   "秦国名将",
		"廉颇":   "赵国名将",
		"蔺相如": "赵国上卿",
		"吕不韦": "秦国相国",
		"李斯":   "秦国丞相",
	}

	HistoryStateDict = map[string]string{
		"齐国": "春秋首霸",
		"晋国": "春秋强国",
		"楚国": "南方大国",
		"秦国": "战国霸主",
		"宋国": "春秋诸侯",
		"鲁国": "礼乐之邦",
		"卫国": "诸侯国",
		"郑国": "诸侯国",
		"吴国": "东南国家",
		"越国": "东南国家",
		"燕国": "战国七雄",
		"赵国": "战国七雄",
		"魏国": "战国七雄",
		"韩国": "战国七雄",
	}

	HistoryEventDict = map[string]string{
		"春秋五霸":   "春秋时期五位霸主",
		"战国七雄":   "战国时期七个强国",
		"三家分晋":   "晋国分裂为韩赵魏",
		"田氏代齐":   "田氏取代姜姓齐国",
		"城濮之战":   "晋楚争霸关键战役",
		"邲之战":     "晋楚争霸战役",
		"鞍之战":     "齐晋争霸战役",
		"鄢陵之战":   "晋楚争霸战役",
		"桂陵之战":   "齐国击败魏国",
		"马陵之战":   "齐国击败魏国",
		"长平之战":   "秦赵决定性战役",
		"合纵连横":   "战国纵横策略",
		"百家争鸣":   "思想繁荣时期",
		"商鞅变法":   "秦国变法图强",
	}
)

// 初始化时加载词典
func init() {
	// 延迟加载，避免启动时耗时
	go func() {
		jieba := GetJiebaService()
		jieba.AddCustomWordsFromDict(HistoryPersonDict)
		jieba.AddCustomWordsFromDict(HistoryStateDict)
		jieba.AddCustomWordsFromDict(HistoryEventDict)
	}()
}

// 辅助函数
func filterEmpty(words []string) []string {
	result := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w != "" && w != " " {
			result = append(result, w)
		}
	}
	return result
}

func extractYears(text string) []Entity {
	entities := make([]Entity, 0)

	// 匹配"公元前XXX年"或"前XXX年"
	patterns := []string{
		`公元前(\d{1,4})年`,
		`前(\d{1,4})年`,
		`(\d{1,4})BC`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 0 {
				entities = append(entities, Entity{
					Text:     match[0],
					Type:     EntityTypeTime,
					Category: "年份",
				})
			}
		}
	}

	return entities
}

func deduplicateEntities(entities []Entity) []Entity {
	seen := make(map[string]bool)
	result := make([]Entity, 0)
	for _, e := range entities {
		key := e.Text + "_" + string(e.Type)
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

// GetSynonyms 获取同义词（用于查询扩展）
func GetSynonyms(word string) []string {
	synonyms := map[string][]string{
		"齐桓公": {"桓公", "小白", "齐小白"},
		"晋文公": {"文公", "重耳"},
		"楚庄王": {"庄王"},
		"孔子":   {"孔丘", "仲尼", "孔夫子", "孔圣人"},
		"老子":   {"李耳", "老聃"},
		"墨子":   {"墨翟"},
		"孟子":   {"孟轲"},
		"管仲":   {"管子", "夷吾"},
	}

	if syns, ok := synonyms[word]; ok {
		return syns
	}
	return nil
}

// GetRelatedConcepts 获取相关概念（用于查询扩展）
func GetRelatedConcepts(word string) []string {
	related := map[string][]string{
		"齐桓公": {"管仲", "春秋五霸", "葵丘之盟", "尊王攘夷"},
		"晋文公": {"重耳", "城濮之战", "春秋五霸", "介子推"},
		"楚庄王": {"一鸣惊人", "邲之战", "春秋五霸", "孙叔敖"},
		"孔子":   {"儒家", "论语", "春秋", "周游列国", "孔门十哲"},
		"老子":   {"道家", "道德经", "无为而治", "庄子"},
		"墨子":   {"墨家", "兼爱", "非攻", "尚贤"},
		"商鞅":   {"商鞅变法", "秦国", "法家", "徙木立信"},
		"长平之战": {"白起", "赵括", "纸上谈兵", "秦国", "赵国"},
	}

	if rels, ok := related[word]; ok {
		return rels
	}
	return nil
}
