package mail

import (
	"fmt"
	"math/rand"
	"time"
)

func GenerateVerificationCode() string {
	rand.Seed(time.Now().UnixNano())
	code := rand.Intn(899999) + 100000 // 生成6位随机数字
	return fmt.Sprintf("%06d", code)
}
