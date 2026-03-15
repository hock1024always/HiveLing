package mail

import (
	"fmt"
	"log"

	"gopkg.in/gomail.v2"
)

// SendMail 发送邮件
func SendMail(to, subject, body string) error {
	// 创建一个新的邮件消息
	mailer := gomail.NewMessage()
	// 设置邮件的发件人
	mailer.SetHeader("From", "15637700031@163.com")
	// 设置邮件的收件人
	mailer.SetHeader("To", to)
	// 设置邮件的主题
	mailer.SetHeader("Subject", subject)
	// 设置邮件的内容
	mailer.SetBody("text/plain", body)

	// 替换为你的授权码
	authorizationCode := "VBNmfMgQ5sFLb2m5"

	// 创建一个新的SMTP拨号器
	// 使用端口 465（SSL）或 587（TLS）
	dialer := gomail.NewDialer("smtp.163.com", 465, "15637700031@163.com", authorizationCode)

	// 使用拨号器发送邮件
	if err := dialer.DialAndSend(mailer); err != nil {
		// 如果发送邮件失败，打印错误信息
		log.Println("Failed to send email:", err)
		// 返回错误信息
		return fmt.Errorf("邮件发送失败: %v", err)
	}

	// 如果发送邮件成功，返回nil
	return nil
}
