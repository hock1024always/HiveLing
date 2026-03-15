package controllers

import (
	"github.com/gin-gonic/gin"
	"log"
)

// WebSocket 处理函数
func (u UserController) LiveHandler(c *gin.Context) {
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

		// 广播消息给所有连接的客户端
		mutex.Lock()
		for client := range clients {
			if err := client.WriteMessage(messageType, message); err != nil {
				log.Println("Error broadcasting message:", err)
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}
