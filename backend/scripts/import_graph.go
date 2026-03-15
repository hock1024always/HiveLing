package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/hock1024always/GoEdu/config"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
)

// GraphData 知识图谱数据结构
type GraphData struct {
	Nodes []NodeData `json:"nodes"`
	Edges []EdgeData `json:"edges"`
}

type NodeData struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type EdgeData struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Relation    string `json:"relation"`
	Description string `json:"description"`
}

func main() {
	// 加载配置
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}
	config.InitConfig()

	// 初始化数据库
	if err := dao.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dao.CloseDB()

	// 读取图谱数据
	data, err := os.ReadFile("./knowledge/graph_seed.json")
	if err != nil {
		log.Fatalf("Failed to read graph data: %v", err)
	}

	var graphData GraphData
	if err := json.Unmarshal(data, &graphData); err != nil {
		log.Fatalf("Failed to parse graph data: %v", err)
	}

	// 导入节点
	nodeMap := make(map[string]uint) // name -> ID
	for _, nodeData := range graphData.Nodes {
		node := models.KGNode{
			Name:        nodeData.Name,
			Type:        nodeData.Type,
			Description: nodeData.Description,
		}

		// 检查是否已存在
		var existing models.KGNode
		result := dao.Db.Where("name = ? AND type = ?", node.Name, node.Type).First(&existing)
		if result.Error == nil {
			nodeMap[node.Name] = existing.ID
			continue
		}

		if err := dao.Db.Create(&node).Error; err != nil {
			log.Printf("Error creating node %s: %v", node.Name, err)
			continue
		}
		nodeMap[node.Name] = node.ID
	}
	log.Printf("Imported %d nodes", len(nodeMap))

	// 导入边
	edgeCount := 0
	for _, edgeData := range graphData.Edges {
		fromID, fromOK := nodeMap[edgeData.From]
		toID, toOK := nodeMap[edgeData.To]

		if !fromOK || !toOK {
			log.Printf("Node not found: %s -> %s", edgeData.From, edgeData.To)
			continue
		}

		edge := models.KGEdge{
			FromNodeID:   fromID,
			ToNodeID:     toID,
			RelationType: edgeData.Relation,
			Description:  edgeData.Description,
		}

		// 检查是否已存在
		var existing models.KGEdge
		result := dao.Db.Where("from_node_id = ? AND to_node_id = ? AND relation_type = ?",
			fromID, toID, edgeData.Relation).First(&existing)
		if result.Error == nil {
			continue
		}

		if err := dao.Db.Create(&edge).Error; err != nil {
			log.Printf("Error creating edge %s -> %s: %v", edgeData.From, edgeData.To, err)
			continue
		}
		edgeCount++
	}
	log.Printf("Imported %d edges", edgeCount)

	log.Println("Knowledge graph import completed!")
}
