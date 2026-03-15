# 教学对话助手 — RAG 实战

## 一、实现概览

本项目 RAG 基于 **MySQL 全文检索 + LIKE 降级 + BM25 简化打分**，不依赖向量库，适合中小规模教育知识库。

---

## 二、知识库数据模型（`models/knowledge.go`）

```sql
CREATE TABLE knowledge_chunks (
    id       BIGINT AUTO_INCREMENT PRIMARY KEY,
    title    VARCHAR(500),
    content  LONGTEXT,
    keywords TEXT,          -- 关键词字符串，用于检索加权
    category VARCHAR(50),   -- person/event/battle/school/state/culture
    source   VARCHAR(255),  -- 来源（如《史记》《战国策》）
    FULLTEXT INDEX ft_idx (title, content, keywords)
);
```

**category 枚举（`services/mcp/tools.go:269`）：**

| category | 说明 |
|----------|------|
| `person` | 历史人物（孔子、商鞅、秦始皇等）|
| `event` | 历史事件（三家分晋、商鞅变法等）|
| `battle` | 战役（长平之战、城濮之战等）|
| `school` | 思想流派（儒家、道家、法家等）|
| `state` | 诸侯国（秦、齐、楚、燕、赵、魏、韩等）|
| `culture` | 制度文化（井田制、分封制等）|

---

## 三、检索实现（`services/rag/retriever.go`）

### 两级检索策略

**第一级：MySQL FULLTEXT 全文检索**（`retriever.go:48`）

```go
db.Where("MATCH(title, content, keywords) AGAINST(? IN NATURAL LANGUAGE MODE)", query).
   Order("MATCH(title, content, keywords) AGAINST(? IN NATURAL LANGUAGE MODE) DESC", query).
   Limit(limit).Find(&chunks)
```

要求 `knowledge_chunks` 表存在 `FULLTEXT` 索引，检索速度快，语言分析由 MySQL 内置分词处理。

**第二级：LIKE 模糊检索**（降级，`retriever.go:54`）

当全文检索失败或无结果时，自动降级：

```go
keywords := extractKeywords(query)  // 去停用词提取关键词
for _, kw := range keywords {
    // (title LIKE '%kw%' OR content LIKE '%kw%' OR keywords LIKE '%kw%')
}
```

取 `limit×2` 条候选后进行二次打分排序。

### 关键词提取（`retriever.go:105`）

```go
// 简单分词：按空格分词，过滤停用词（的/是/在/了/和...）
// 保留长度 >= 2 的词
// TODO: 可接入 jieba 分词提升精度
```

---

## 四、相关性打分（BM25 简化版，`retriever.go:129`）

```go
func calculateScore(query string, chunk *models.KnowledgeChunk) float64 {
    for _, term := range queryTerms {
        if strings.Contains(title, term)    { score += 3.0 }  // 标题命中权重最高
        if strings.Contains(keywords, term) { score += 2.0 }  // keywords 字段命中
        score += float64(strings.Count(content, term)) * 0.5  // 内容出现次数
    }
    return score
}
```

全部候选按 score 降序排序后取 TopK。

---

## 五、上下文构建（`rag/retriever.go:161`）

```go
func BuildContext(results []SearchResult, maxLen int) string {
    var builder strings.Builder
    builder.WriteString("\n【参考资料】\n")
    for i, result := range results {
        chunkText := fmt.Sprintf("\n%d. [%s] %s\n%s\n",
            i+1, result.Chunk.Category, result.Chunk.Title, result.Chunk.Content)
        if totalLen+len(chunkText) > maxLen { break }
        builder.WriteString(chunkText)
    }
    return builder.String()
}
```

`maxLen` 参数控制总字符数：
- 对话助手调用：`BuildContext(results, 3000)`
- 教案生成调用：`BuildContext(results, 4000)`

---

## 六、知识库导入（`backend/scripts/import_knowledge.go`）

离线批量导入脚本，支持从结构化数据批量写入 `knowledge_chunks` 表。每条记录包含 title/content/keywords/category/source 五个字段。

---

## 七、在对话助手中的调用链

```
DialogController.Chat
    ↓
ToolExecutor.ExecuteTool("search_knowledge", args)
    ↓
rag.Retriever.Search(query, category, limit=5)
    ├─ MySQL FULLTEXT 检索
    └─ (降级) LIKE 检索
    ↓
calculateScore 打分排序
    ↓
FormatResults → JSON 字符串返回给 LLM
```

在备课工作流中（`outline.go:110`/`lessonplan.go:71`）：

```go
ragContext := s.retrieveContext(topic)
// 直接构建 BuildContext 字符串，拼入 Prompt
```

---

## 八、与向量 RAG 的对比

| 维度 | 本项目（MySQL 全文+LIKE）| 向量 RAG（Milvus/pgvector）|
|------|------------------------|--------------------------|
| 语义理解 | 较弱（关键词匹配）| 强（语义相似度）|
| 部署复杂度 | 低（仅 MySQL）| 高（需向量数据库）|
| 召回精度 | 关键词命中率高 | 语义近义词命中率高 |
| 适用场景 | 历史专有名词密集（人名/地名/事件名）| 通用开放问答 |

> 春秋战国历史问答中，专有名词（孔子、管仲、长平之战等）精确匹配比语义相似度更重要，MySQL FULLTEXT 在此场景下表现良好。
