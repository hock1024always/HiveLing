package graph

import (
	"encoding/json"
	"fmt"

	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// QueryService 知识图谱查询服务
type QueryService struct{}

// NewQueryService 创建查询服务
func NewQueryService() *QueryService {
	return &QueryService{}
}

// QueryResult 查询结果
type QueryResult struct {
	Entity      string       `json:"entity"`
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Relations   []Relation   `json:"relations"`
}

// Relation 关系
type Relation struct {
	Target      string `json:"target"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Direction   string `json:"direction"` // "out" 或 "in"
}

// QueryByName 根据名称查询实体及其关系
func (s *QueryService) QueryByName(name string) (*QueryResult, error) {
	// 查找节点
	var node models.KGNode
	err := dao.Db.Where("name = ?", name).First(&node).Error
	if err != nil {
		return nil, fmt.Errorf("entity not found: %s", name)
	}

	result := &QueryResult{
		Entity:      node.Name,
		Type:        node.Type,
		Description: node.Description,
		Relations:   make([]Relation, 0),
	}

	// 查找出边（该实体指向其他实体的关系）
	var outEdges []models.KGEdge
	dao.Db.Where("from_node_id = ?", node.ID).Find(&outEdges)

	for _, edge := range outEdges {
		var targetNode models.KGNode
		if err := dao.Db.First(&targetNode, edge.ToNodeID).Error; err == nil {
			result.Relations = append(result.Relations, Relation{
				Target:      targetNode.Name,
				Type:        edge.RelationType,
				Description: edge.Description,
				Direction:   "out",
			})
		}
	}

	// 查询入边（其他实体指向该实体的关系）
	var inEdges []models.KGEdge
	dao.Db.Where("to_node_id = ?", node.ID).Find(&inEdges)

	for _, edge := range inEdges {
		var sourceNode models.KGNode
		if err := dao.Db.First(&sourceNode, edge.FromNodeID).Error; err == nil {
			result.Relations = append(result.Relations, Relation{
				Target:      sourceNode.Name,
				Type:        edge.RelationType,
				Description: edge.Description,
				Direction:   "in",
			})
		}
	}

	return result, nil
}

// QueryByType 按类型查询实体
func (s *QueryService) QueryByType(entityType string) ([]models.KGNode, error) {
	var nodes []models.KGNode
	err := dao.Db.Where("type = ?", entityType).Find(&nodes).Error
	return nodes, err
}

// QueryRelations 查询两个实体之间的关系
func (s *QueryService) QueryRelations(entity1, entity2 string) ([]models.KGEdge, error) {
	var node1, node2 models.KGNode

	if err := dao.Db.Where("name = ?", entity1).First(&node1).Error; err != nil {
		return nil, fmt.Errorf("entity not found: %s", entity1)
	}
	if err := dao.Db.Where("name = ?", entity2).First(&node2).Error; err != nil {
		return nil, fmt.Errorf("entity not found: %s", entity2)
	}

	var edges []models.KGEdge
	dao.Db.Where("from_node_id = ? AND to_node_id = ?", node1.ID, node2.ID).Find(&edges)
	dao.Db.Or("from_node_id = ? AND to_node_id = ?", node2.ID, node1.ID).Find(&edges)

	return edges, nil
}

// SearchEntities 搜索实体
func (s *QueryService) SearchEntities(keyword string, limit int) ([]models.KGNode, error) {
	if limit <= 0 {
		limit = 10
	}

	var nodes []models.KGNode
	err := dao.Db.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
		Limit(limit).
		Find(&nodes).Error

	return nodes, err
}

// GetRelatedEntities 获取相关实体（通过关系连接的实体）
func (s *QueryService) GetRelatedEntities(name string, relationType string) ([]models.KGNode, error) {
	var node models.KGNode
	if err := dao.Db.Where("name = ?", name).First(&node).Error; err != nil {
		return nil, fmt.Errorf("entity not found: %s", name)
	}

	var relatedIDs []uint

	if relationType == "" {
		// 获取所有相关实体
		dao.Db.Model(&models.KGEdge{}).
			Where("from_node_id = ?", node.ID).
			Pluck("to_node_id", &relatedIDs)
		dao.Db.Model(&models.KGEdge{}).
			Where("to_node_id = ?", node.ID).
			Pluck("from_node_id", &relatedIDs)
	} else {
		// 获取特定关系类型的相关实体
		dao.Db.Model(&models.KGEdge{}).
			Where("from_node_id = ? AND relation_type = ?", node.ID, relationType).
			Pluck("to_node_id", &relatedIDs)
		dao.Db.Model(&models.KGEdge{}).
			Where("to_node_id = ? AND relation_type = ?", node.ID, relationType).
			Pluck("from_node_id", &relatedIDs)
	}

	if len(relatedIDs) == 0 {
		return []models.KGNode{}, nil
	}

	var nodes []models.KGNode
	dao.Db.Where("id IN ?", relatedIDs).Find(&nodes)

	return nodes, nil
}

// FormatResult 格式化查询结果为字符串
func (s *QueryService) FormatResult(result *QueryResult) string {
	output := fmt.Sprintf("【%s】(%s)\n%s\n\n", result.Entity, result.Type, result.Description)

	if len(result.Relations) > 0 {
		output += "关系：\n"
		for _, rel := range result.Relations {
			if rel.Direction == "out" {
				output += fmt.Sprintf("  → %s [%s]: %s\n", rel.Target, rel.Type, rel.Description)
			} else {
				output += fmt.Sprintf("  ← %s [%s]: %s\n", rel.Target, rel.Type, rel.Description)
			}
		}
	}

	return output
}

// FormatResultJSON 格式化查询结果为 JSON
func (s *QueryService) FormatResultJSON(result *QueryResult) string {
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}
