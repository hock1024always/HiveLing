package config

import (
	"os"
	"strconv"
)

// Config 全局配置结构
type Config struct {
	DeepSeek  DeepSeekConfig
	Serper    SerperConfig
	MySQL     MySQLConfig
	Redis     RedisConfig
	Server    ServerConfig
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type SerperConfig struct {
	APIKey  string
	BaseURL string
}

type MySQLConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Address  string
	Password string
}

type ServerConfig struct {
	Port    int
	GinMode string
}

// AppConfig 全局配置实例
var AppConfig Config

// InitConfig 初始化配置，从环境变量读取
func InitConfig() {
	AppConfig = Config{
		DeepSeek: DeepSeekConfig{
			APIKey:  getEnv("DEEPSEEK_API_KEY", ""),
			BaseURL: getEnv("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
			Model:   getEnv("DEEPSEEK_MODEL", "deepseek-chat"),
		},
		Serper: SerperConfig{
			APIKey:  getEnv("SERPER_API_KEY", ""),
			BaseURL: getEnv("SERPER_BASE_URL", "https://google.serper.dev"),
		},
		MySQL: MySQLConfig{
			Host:     getEnv("MYSQL_HOST", "localhost"),
			Port:     getEnvInt("MYSQL_PORT", 3306),
			User:     getEnv("MYSQL_USER", "root"),
			Password: getEnv("MYSQL_PASSWORD", ""),
			Database: getEnv("MYSQL_DATABASE", "goedu"),
		},
		Redis: RedisConfig{
			Address:  getEnv("REDIS_ADDRESS", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Server: ServerConfig{
			Port:    getEnvInt("SERVER_PORT", 9090),
			GinMode: getEnv("GIN_MODE", "debug"),
		},
	}
}

// getEnv 获取环境变量，支持默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数类型环境变量
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetDSN 生成 MySQL 连接字符串
func (c *MySQLConfig) GetDSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(c.Port) + ")/" + c.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
}
