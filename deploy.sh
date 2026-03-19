#!/bin/bash

# 慧备灵师 - 一键部署脚本
# 支持: Docker 部署 或 本地部署

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的信息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 显示帮助
show_help() {
    cat << EOF
慧备灵师 - 一键部署脚本

用法: ./deploy.sh [选项] [命令]

命令:
    docker      使用 Docker 部署（推荐）
    local       本地部署（需要 Go 1.21+ 和 MySQL）
    stop        停止服务
    restart     重启服务
    status      查看服务状态
    logs        查看日志
    update      更新到最新版本

选项:
    -h, --help  显示帮助信息
    -e, --env   指定环境 (dev/prod)，默认 prod

示例:
    ./deploy.sh docker          # Docker 部署
    ./deploy.sh local           # 本地部署
    ./deploy.sh -e dev local    # 本地开发环境部署
    ./deploy.sh stop            # 停止服务
    ./deploy.sh logs            # 查看日志

EOF
}

# 检查系统要求
check_requirements() {
    print_info "检查系统要求..."

    # 检查操作系统
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    else
        print_error "不支持的操作系统: $OSTYPE"
        exit 1
    fi

    print_success "操作系统检查通过: $OS"
}

# Docker 部署
deploy_docker() {
    print_info "开始使用 Docker 部署..."

    # 检查 Docker
    if ! command_exists docker; then
        print_error "Docker 未安装，请先安装 Docker"
        print_info "安装指南: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! command_exists docker-compose; then
        print_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi

    print_success "Docker 环境检查通过"

    # 创建必要的目录
    mkdir -p data/mysql data/redis data/milvus logs uploads

    # 检查配置文件
    if [ ! -f "backend/config/config.yaml" ]; then
        print_warning "配置文件不存在，创建默认配置..."
        cp backend/config/config.example.yaml backend/config/config.yaml 2>/dev/null || create_default_config
    fi

    # 构建并启动服务
    print_info "构建 Docker 镜像..."
    docker-compose build

    print_info "启动服务..."
    docker-compose up -d

    # 等待服务启动
    print_info "等待服务启动..."
    sleep 10

    # 检查服务状态
    check_service_health

    print_success "Docker 部署完成！"
    print_info "前端访问: http://localhost:8080"
    print_info "API 访问: http://localhost:8080/api"
}

# 本地部署
deploy_local() {
    print_info "开始本地部署..."

    # 检查 Go
    if ! command_exists go; then
        print_error "Go 未安装，请先安装 Go 1.21+"
        print_info "安装指南: https://golang.org/doc/install"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_success "Go 版本: $GO_VERSION"

    # 检查 MySQL
    if ! command_exists mysql; then
        print_warning "MySQL 客户端未安装，请确保 MySQL 服务可访问"
    fi

    # 安装依赖
    print_info "安装后端依赖..."
    cd backend
    go mod download
    go mod tidy

    # 编译
    print_info "编译后端服务..."
    go build -o ../bin/goedu-server main.go
    cd ..

    # 创建必要的目录
    mkdir -p bin logs uploads

    # 检查配置文件
    if [ ! -f "backend/config/config.yaml" ]; then
        print_warning "配置文件不存在，创建默认配置..."
        create_default_config
    fi

    # 初始化数据库
    print_info "初始化数据库..."
    init_database

    # 启动服务
    print_info "启动服务..."
    ./bin/goedu-server &
    echo $! > bin/server.pid

    print_success "本地部署完成！"
    print_info "前端访问: http://localhost:8080"
    print_info "API 访问: http://localhost:8080/api"
    print_info "服务 PID: $(cat bin/server.pid)"
}

# 创建默认配置
create_default_config() {
    mkdir -p backend/config
    cat > backend/config/config.yaml << 'EOF'
server:
  port: 8080
  mode: release
  frontend_path: ./frontend

database:
  host: localhost
  port: 3306
  username: root
  password: ""
  database: goedu
  charset: utf8mb4

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

milvus:
  enabled: false
  host: localhost
  port: 19530
  collection: goedu_knowledge

deepseek:
  api_key: "your_deepseek_api_key"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"

serper:
  api_key: "your_serper_api_key"
  base_url: "https://google.serper.dev"

log:
  level: info
  path: ./logs
  max_size: 100
  max_age: 30
  max_backups: 10
EOF
    print_success "默认配置已创建: backend/config/config.yaml"
    print_warning "请修改配置文件中的 API 密钥和数据库连接信息"
}

# 初始化数据库
init_database() {
    print_info "检查数据库连接..."

    # 读取配置
    DB_HOST=$(grep "host:" backend/config/config.yaml | head -1 | awk '{print $2}')
    DB_PORT=$(grep "port:" backend/config/config.yaml | head -1 | awk '{print $2}')
    DB_USER=$(grep "username:" backend/config/config.yaml | awk '{print $2}')
    DB_PASS=$(grep "password:" backend/config/config.yaml | awk '{print $2}')
    DB_NAME=$(grep "database:" backend/config/config.yaml | head -1 | awk '{print $2}')

    # 尝试连接数据库
    if mysql -h$DB_HOST -P$DB_PORT -u$DB_USER -p$DB_PASS -e "SELECT 1;" >/dev/null 2>&1; then
        print_success "数据库连接成功"

        # 检查数据库是否存在
        if ! mysql -h$DB_HOST -P$DB_PORT -u$DB_USER -p$DB_PASS -e "USE $DB_NAME;" >/dev/null 2>&1; then
            print_info "创建数据库 $DB_NAME..."
            mysql -h$DB_HOST -P$DB_PORT -u$DB_USER -p$DB_PASS -e "CREATE DATABASE $DB_NAME CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
        fi

        # 执行迁移
        print_info "执行数据库迁移..."
        cd backend
        go run scripts/migrate.go
        cd ..
    else
        print_error "无法连接到数据库，请检查配置"
        exit 1
    fi
}

# 检查服务健康状态
check_service_health() {
    print_info "检查服务健康状态..."

    MAX_RETRIES=30
    RETRY_COUNT=0

    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        if curl -s http://localhost:8080/api/health >/dev/null 2>&1; then
            print_success "服务运行正常"
            return 0
        fi

        RETRY_COUNT=$((RETRY_COUNT + 1))
        print_info "等待服务启动... ($RETRY_COUNT/$MAX_RETRIES)"
        sleep 2
    done

    print_error "服务启动超时，请检查日志"
    return 1
}

# 停止服务
stop_service() {
    print_info "停止服务..."

    # 停止 Docker 服务
    if [ -f "docker-compose.yml" ]; then
        docker-compose down 2>/dev/null || true
    fi

    # 停止本地服务
    if [ -f "bin/server.pid" ]; then
        PID=$(cat bin/server.pid)
        if kill -0 $PID 2>/dev/null; then
            kill $PID
            print_success "本地服务已停止 (PID: $PID)"
        fi
        rm -f bin/server.pid
    fi

    print_success "所有服务已停止"
}

# 查看服务状态
show_status() {
    print_info "服务状态:"

    # Docker 状态
    if [ -f "docker-compose.yml" ]; then
        echo ""
        echo "Docker 服务:"
        docker-compose ps 2>/dev/null || echo "  未运行"
    fi

    # 本地服务状态
    if [ -f "bin/server.pid" ]; then
        PID=$(cat bin/server.pid)
        if kill -0 $PID 2>/dev/null; then
            echo ""
            echo "本地服务: 运行中 (PID: $PID)"
        else
            echo ""
            echo "本地服务: 未运行"
        fi
    fi

    # 健康检查
    echo ""
    echo "API 健康检查:"
    if curl -s http://localhost:8080/api/health 2>/dev/null; then
        echo ""
    else
        echo "  无法连接"
    fi
}

# 查看日志
show_logs() {
    print_info "查看日志..."

    if [ -f "docker-compose.yml" ]; then
        docker-compose logs -f --tail=100
    elif [ -d "logs" ]; then
        tail -f logs/*.log 2>/dev/null || echo "暂无日志文件"
    else
        echo "暂无日志"
    fi
}

# 更新服务
update_service() {
    print_info "更新到最新版本..."

    # 拉取最新代码
    git pull origin main 2>/dev/null || print_warning "无法拉取代码，请手动更新"

    # 重新部署
    if [ -f "docker-compose.yml" ]; then
        docker-compose down
        docker-compose pull
        docker-compose up -d --build
    else
        stop_service
        deploy_local
    fi

    print_success "更新完成"
}

# 主函数
main() {
    # 显示欢迎信息
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║     慧备灵师 - AI 教学平台部署脚本      ║"
    echo "║         一键部署，快速启动              ║"
    echo "╚════════════════════════════════════════╝"
    echo ""

    # 检查参数
    COMMAND=${1:-help}
    ENV="prod"

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -e|--env)
                ENV="$2"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                COMMAND="$1"
                shift
                ;;
        esac
    done

    # 执行命令
    case $COMMAND in
        docker)
            check_requirements
            deploy_docker
            ;;
        local)
            check_requirements
            deploy_local
            ;;
        stop)
            stop_service
            ;;
        restart)
            stop_service
            sleep 2
            if [ -f "docker-compose.yml" ]; then
                deploy_docker
            else
                deploy_local
            fi
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        update)
            update_service
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"
