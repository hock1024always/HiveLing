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
	Milvus    MilvusConfig
	Embedding EmbeddingConfig
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

// MilvusConfig Milvus 向量数据库配置
type MilvusConfig struct {
	Host         string
	Port         int
	Collection   string // 集合名称
	Dimension    int    // 向量维度
	Enabled      bool   // 是否启用向量检索
}

// EmbeddingConfig Embedding 配置
type EmbeddingConfig struct {
	Provider string // deepseek, openai, local
	Model    string // 模型名称
	Dimension int   // 向量维度
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
		Milvus: MilvusConfig{
			Host:       getEnv("MILVUS_HOST", "localhost"),
			Port:       getEnvInt("MILVUS_PORT", 19530),
			Collection: getEnv("MILVUS_COLLECTION", "knowledge_vectors"),
			Dimension:  getEnvInt("MILVUS_DIMENSION", 1536),
			Enabled:    getEnvBool("MILVUS_ENABLED", false),
		},
		Embedding: EmbeddingConfig{
			Provider:  getEnv("EMBEDDING_PROVIDER", "deepseek"),
			Model:     getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
			Dimension: getEnvInt("EMBEDDING_DIMENSION", 1536),
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

// getEnvBool 获取布尔类型环境变量
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// GetDSN 生成 MySQL 连接字符串
func (c *MySQLConfig) GetDSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(c.Port) + ")/" + c.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
}
