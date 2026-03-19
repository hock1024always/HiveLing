# 慧备灵师 - 部署指南

> 一键部署 AI 教学平台，支持 Docker 和本地两种部署方式

## 快速开始

### 方式一：Docker 部署（推荐）

```bash
# 1. 克隆代码
git clone <your-repo-url>
cd GoEdu

# 2. 运行部署脚本
./deploy.sh docker
```

### 方式二：本地部署

```bash
# 1. 确保已安装 Go 1.21+ 和 MySQL 8.0

# 2. 运行部署脚本
./deploy.sh local
```

## 系统要求

| 组件 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核 |
| 内存 | 4 GB | 8 GB |
| 磁盘 | 20 GB | 50 GB |
| 网络 | 可访问互联网 | 稳定互联网连接 |

## Docker 部署详解

### 1. 环境准备

```bash
# 安装 Docker
curl -fsSL https://get.docker.com | sh

# 安装 Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

### 2. 配置文件

创建 `backend/config/config.yaml`：

```yaml
server:
  port: 8080
  mode: release
  frontend_path: ./frontend

database:
  host: mysql
  port: 3306
  username: root
  password: goedu123456
  database: goedu
  charset: utf8mb4

redis:
  host: redis
  port: 6379
  password: ""
  db: 0

milvus:
  enabled: true
  host: milvus-standalone
  port: 19530
  collection: goedu_knowledge

deepseek:
  api_key: "your_deepseek_api_key_here"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"

serper:
  api_key: "your_serper_api_key_here"
  base_url: "https://google.serper.dev"

log:
  level: info
  path: ./logs
  max_size: 100
  max_age: 30
  max_backups: 10
```

### 3. 启动服务

```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f backend
```

### 4. 服务访问

| 服务 | 地址 | 说明 |
|------|------|------|
| Web 前端 | http://localhost:8080 | 教学平台主界面 |
| API 服务 | http://localhost:8080/api | REST API |
| MySQL | localhost:3306 | 数据库 |
| Redis | localhost:6379 | 缓存 |
| Milvus | localhost:19530 | 向量数据库 |

## 本地部署详解

### 1. 环境准备

**安装 Go 1.21+**

```bash
# macOS
brew install go

# Ubuntu/Debian
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

**安装 MySQL 8.0**

```bash
# macOS
brew install mysql
brew services start mysql

# Ubuntu/Debian
sudo apt-get install mysql-server
sudo systemctl start mysql
```

**安装 Redis（可选）**

```bash
# macOS
brew install redis
brew services start redis
```

### 2. 数据库初始化

```bash
# 登录 MySQL
mysql -u root -p

# 创建数据库
CREATE DATABASE goedu CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 创建用户（可选）
CREATE USER 'goedu'@'localhost' IDENTIFIED BY 'your_password';
GRANT ALL PRIVILEGES ON goedu.* TO 'goedu'@'localhost';
FLUSH PRIVILEGES;
```

### 3. 配置文件

创建 `backend/config/config.yaml`，修改数据库连接信息：

```yaml
database:
  host: localhost
  port: 3306
  username: root
  password: "your_mysql_password"
  database: goedu
```

### 4. 启动服务

```bash
# 使用部署脚本
./deploy.sh local

# 或手动启动
cd backend
go mod download
go run main.go
```

## 部署脚本命令

```bash
# Docker 部署
./deploy.sh docker

# 本地部署
./deploy.sh local

# 停止服务
./deploy.sh stop

# 重启服务
./deploy.sh restart

# 查看状态
./deploy.sh status

# 查看日志
./deploy.sh logs

# 更新到最新版本
./deploy.sh update
```

## API 密钥配置

### DeepSeek API

1. 访问 [DeepSeek 开放平台](https://platform.deepseek.com/)
2. 注册账号并创建 API Key
3. 修改 `config.yaml` 中的 `deepseek.api_key`

### Serper API（联网搜索）

1. 访问 [Serper](https://serper.dev/)
2. 注册账号获取 API Key
3. 修改 `config.yaml` 中的 `serper.api_key`

## 生产环境部署

### 使用 Nginx 反向代理

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        root /path/to/GoEdu/frontend;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### HTTPS 配置

```bash
# 使用 Certbot 申请 SSL 证书
sudo certbot --nginx -d your-domain.com
```

### 启用生产模式

```bash
# 使用 docker-compose 生产配置
docker-compose --profile production up -d
```

## 常见问题

### Q: 服务启动失败

```bash
# 检查日志
docker-compose logs backend

# 检查端口占用
sudo lsof -i :8080
```

### Q: 数据库连接失败

```bash
# 检查 MySQL 状态
docker-compose ps mysql

# 手动连接测试
mysql -h127.0.0.1 -P3306 -uroot -p
```

### Q: Milvus 启动慢

Milvus 首次启动需要初始化，请耐心等待 1-2 分钟。

### Q: 前端无法访问 API

检查 `frontend/*.html` 中的 `API_BASE` 配置：

```javascript
const API_BASE = window.location.origin;  // 自动检测
// 或
const API_BASE = 'http://your-api-server:8080';  // 手动指定
```

## 监控与维护

### 查看系统状态

访问 `http://localhost:8080/dashboard.html` 查看：
- 系统运行状态
- Agent 性能统计
- 对话趋势分析
- 实时日志

### 备份数据

```bash
# 备份 MySQL
docker exec goedu-mysql mysqldump -uroot -pgoedu123456 goedu > backup.sql

# 备份上传文件
tar -czvf uploads-backup.tar.gz uploads/
```

### 更新服务

```bash
# 拉取最新代码
git pull origin main

# 重新部署
./deploy.sh update
```

## 技术支持

- 文档：https://github.com/your-repo/GoEdu/wiki
- 问题反馈：https://github.com/your-repo/GoEdu/issues
- 邮箱：support@goedu.com

---

**版本**: v1.1.0  
**更新日期**: 2026-03-18
