package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/hock1024always/GoEdu/dao"
	"github.com/hock1024always/GoEdu/models"
	"log"
	"net/http"
	"sync"
	"time"
)

// TableName 方法用于指定表名
func (Message) TableName() string {
	return "message"
}

type Message struct {
	ID       uint      `gorm:"primaryKey;autoIncrement"` // 主键且自动递增
	Username string    `gorm:"not_null"`
	Time     time.Time `gorm:"autoCreateTime;autoUpdateTime"` // 自动设置创建时间和更新时间
	Message  string    `gorm:"not_null"`
}

type ClientMessage struct {
	Username string      `json:"username"`
	Data     interface{} `json:"data"`
}

type MessageChicker struct {
	Username string `json:"username"`
	Msg      string `json:"data"`
}

// 定义 WebSocket Upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有跨域请求，实际项目中可根据需要限制
	},
}

// 定义一个全局的客户端连接集合
var clients = make(map[*websocket.Conn]bool)
var mutex = &sync.Mutex{}

// WebSocket 处理函数
func (u UserController) WsHandler(c *gin.Context) {
	//接受Token
	token := c.DefaultPostForm("token", "")
	//鉴权
	username := models.ValidateToken(token)
	user1, _ := dao.CheckUserExist(username)
	if user1.Id == 0 {
		models.ReturnError(c, 4012, "用户名不存在")
		return
	}

	// 升级 HTTP 连接为 WebSocket 连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to upgrade to WebSocket:", err)
		return
	}
	defer ws.Close()

	// 将新连接的客户端添加到集合中
	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	// 长连接逻辑
	for {
		// 读取消息
		messageType, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			// 如果客户端断开连接，从集合中移除
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			break
		}

		// 解析消息
		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("Error parsing message:", err)
			continue
		}

		// 处理消息
		log.Printf("Received username: %s, data: %v\n", msg.Username, msg.Data)

		// 广播消息给所有连接的客户端
		broadcastMessage(msg)

		err1 := AddMsg(msg)
		if err1 != nil {
			log.Println("Error adding message:", err1)
			continue
		}

		// 发送响应
		response := fmt.Sprintf("Server received event: %s", msg.Username)
		if err := ws.WriteMessage(messageType, []byte(response)); err != nil {
			log.Println("Error writing message:", err)
			break
		}
	}
}

func AddMsg(msg ClientMessage) error {
	// 将 msg.Data 转换为 string 类型
	messageData, ok := msg.Data.(string)
	if !ok {
		log.Printf("Failed to assert msg.Data as string: %v\n", msg.Data)
		return errors.New("Failed to assert msg.Data as string")
	}
	// 创建消息实例
	newMessage := Message{
		Username: msg.Username,
		Message:  messageData,
	}
	// 使用 GORM 插入消息到数据库中
	if err := dao.Db.Create(&newMessage).Error; err != nil {
		log.Println("Error creating message record:", err)
		return err
	}
	return nil
}

// 广播消息给所有连接的客户端
func broadcastMessage(msg ClientMessage) {
	// 将消息转换为 JSON 格式
	messageData, ok := msg.Data.(string)
	if !ok {
		log.Printf("Failed to assert msg.Data as string: %v\n", msg.Data)
		return
	}
	broadcastMsg := map[string]string{
		"username": msg.Username,
		"data":     messageData,
	}
	broadcastBytes, err := json.Marshal(broadcastMsg)
	if err != nil {
		log.Println("Error marshaling broadcast message:", err)
		return
	}

	// 遍历所有客户端并发送消息
	mutex.Lock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, broadcastBytes)
		if err != nil {
			log.Println("Error broadcasting message:", err)
			client.Close()
			delete(clients, client)
		}
	}
	mutex.Unlock()
}

// 定义AI接口的请求和响应结构
type AIRequest struct {
	Prompt string `json:"prompt"`
}
type AIResponse struct {
	Text string `json:"text"`
}

// 查找消息
//func GetVoteList(username string, sort string) ([]Vote, error) {
//	var votes []Vote
//	err := dao.Db.Where("username =?", username).Order(sort).Find(&votes).Error
//	return votes, err
//}
