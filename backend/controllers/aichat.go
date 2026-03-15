package controllers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"

	"github.com/hock1024always/GoEdu/models"
	"log"
)

//func getAIResponse(prompt string) (string, error) {
//	// 这里可以替换为实际的AI接口调用，例如OpenAI的API
//	answer, ok := models.GetAIResponse(prompt)
//	if ok != nil {
//		return "", errors.New("Failed to get AI response")
//	}
//	// 以下是模拟返回
//	return fmt.Sprintf("AI回答: %s", answer), nil
//}

//func AddAiMsg(msg ClientMessage) error {
//	// 将 msg.Data 转换为 string 类型
//	messageData, ok := msg.Data.(string)
//	if !ok {
//		log.Printf("Failed to assert msg.Data as string: %v\n", msg.Data)
//		return errors.New("Failed to assert msg.Data as string")
//	}
//	// 创建消息实例
//	newMessage := Message{
//		Username: msg.Username,
//		Message:  messageData,
//	}
//	// 使用 GORM 插入消息到数据库中
//	if err := dao.Db.Create(&newMessage).Error; err != nil {
//		log.Println("Error creating message record:", err)
//		return err
//	}
//	return nil
//}

// WebSocket 处理函数
//func (u UserController) AIHandler(c *gin.Context) {
//	////接受Token
//	//token := c.DefaultPostForm("token", "")
//	////鉴权 检查是否存在
//	//username := models.ValidateToken(token)
//	//username := c.DefaultPostForm("username", "")
//	//user1, _ := models.CheckUserExist(username)
//	//if user1.Id == 0 {
//	//	models.ReturnError(c, 4012, "用户名不存在")
//	//	return
//	//}
//
//	// 升级 HTTP 连接为 WebSocket 连接
//	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
//	if err != nil {
//		log.Println("Failed to upgrade to WebSocket:", err)
//		return
//	}
//	defer ws.Close()
//
//	// 将新连接的客户端添加到集合中
//	mutex.Lock()
//	clients[ws] = true
//	mutex.Unlock()
//
//	magCounter := 0                //设置一个计数器，用于将消息按顺序存储在map中
//	msgMap := make(map[int]string) //消息map，用于存储消息
//
//	// 长连接逻辑
//	for {
//		// 读取消息
//		messageType, message, err := ws.ReadMessage()
//		if err != nil {
//			log.Println("Error reading message:", err)
//			break
//		}
//
//		// 解析消息
//		var msg ClientMessage
//		if err := json.Unmarshal(message, &msg); err != nil {
//			log.Println("Error parsing message:", err)
//			continue
//		}
//
//		// 处理消息
//		log.Printf("Received username: %s, data: %v\n", msg.Username, msg.Data)
//		msgMap[magCounter] = fmt.Sprintf("username: %s, data: %v\n", msg.Username, msg.Data)
//		magCounter++
//		//err1 := AddAiMsg(msg)
//		//if err1 != nil {
//		//	log.Println("Error adding message:", err1)
//		//	continue
//		//}
//
//		// 调用AI接口获取回答
//		responseText, err := getAIResponse(fmt.Sprintf("%v", msgMap))
//		if err != nil {
//			log.Println("Error calling AI API:", err)
//			continue
//		}
//		msgMap[magCounter] = fmt.Sprintf("username: %s, data: %v\n", config.AIUsername, msg.Data)
//		magCounter++
//
//		// 将AI的回答发送回客户端
//		response := map[string]string{
//			"username": "AI",
//			"data":     responseText,
//		}
//		responseBytes, _ := json.Marshal(response)
//		if err := ws.WriteMessage(messageType, responseBytes); err != nil {
//			log.Println("Error writing message:", err)
//			break
//		}
//	}
//}

//var (
//	upgrader = websocket.Upgrader{
//		ReadBufferSize:  1024,
//		WriteBufferSize: 1024,
//	}
//	clients   = make(map[*websocket.Conn]bool)
//	mutex     = &sync.Mutex{}
//)

type ClientMessageAI struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func (a AIController) AIHandler(c *gin.Context) {
	// 升级为WebSocket连接
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	// 添加客户端
	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	// 对话上下文
	var conversation []map[string]string

	for {
		// 读取客户端消息
		_, msgBytes, err := ws.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		// 解析消息
		var clientMsg ClientMessageAI
		if err := json.Unmarshal(msgBytes, &clientMsg); err != nil {
			log.Println("Message parse error:", err)
			continue
		}

		// 添加到对话上下文
		conversation = append(conversation, map[string]string{
			"role":    "user",
			"content": clientMsg.Content,
		})

		log.Printf("Received message from %s: %s\n", clientMsg.Username, clientMsg.Content)

		// 获取AI响应
		aiResponse, err := models.GetAIResponse(conversation)
		if err != nil {
			log.Println("AI response error:", err)
			ws.WriteJSON(map[string]string{
				"username": "AI",
				"content":  "Sorry, I encountered an error. Please try again.",
			})
			continue
		}

		// 添加AI响应到上下文
		conversation = append(conversation, map[string]string{
			"role":    "assistant",
			"content": aiResponse,
		})

		// 发送AI响应
		err = ws.WriteJSON(map[string]string{
			"username": "AI",
			"content":  aiResponse,
		})
		if err != nil {
			log.Println("Write error:", err)
			break
		}
	}

	// 移除客户端
	mutex.Lock()
	delete(clients, ws)
	mutex.Unlock()
}
