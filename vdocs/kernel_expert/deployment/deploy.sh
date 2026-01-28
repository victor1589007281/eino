#!/bin/bash
# deploy.sh - Linux内核专家Agent一键部署脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 默认配置
NAMESPACE=${NAMESPACE:-"kernel-expert"}
RELEASE_NAME=${RELEASE_NAME:-"kernel-expert"}
CHART_PATH=${CHART_PATH:-"./helm"}
ENV=${ENV:-"dev"}

# 日志函数
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# 显示帮助
show_help() {
    cat << EOF
Linux内核专家Agent部署脚本

用法: $0 [选项]

选项:
    -n, --namespace     命名空间 (默认: kernel-expert)
    -r, --release       Release名称 (默认: kernel-expert)
    -e, --env           环境 (dev/staging/prod, 默认: dev)
    -c, --chart         Helm chart路径 (默认: ./helm)
    -h, --help          显示帮助
    
环境变量:
    OPENAI_API_KEY      OpenAI API密钥
    ANTHROPIC_API_KEY   Anthropic API密钥
    DASHSCOPE_API_KEY   通义千问API密钥
    ZHIPU_API_KEY       智谱API密钥
    MOONSHOT_API_KEY    Moonshot API密钥
    DEEPSEEK_API_KEY    DeepSeek API密钥
    POSTGRES_PASSWORD   PostgreSQL密码
    REDIS_PASSWORD      Redis密码
    JWT_SECRET          JWT密钥

示例:
    # 开发环境部署
    export OPENAI_API_KEY="sk-xxx"
    export POSTGRES_PASSWORD="password"
    export REDIS_PASSWORD="password"
    ./deploy.sh -e dev
    
    # 生产环境部署
    ./deploy.sh -e prod -n kernel-expert-prod
EOF
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace) NAMESPACE="$2"; shift 2 ;;
        -r|--release) RELEASE_NAME="$2"; shift 2 ;;
        -e|--env) ENV="$2"; shift 2 ;;
        -c|--chart) CHART_PATH="$2"; shift 2 ;;
        -h|--help) show_help; exit 0 ;;
        *) log_error "未知选项: $1"; show_help; exit 1 ;;
    esac
done

# 检查依赖
check_dependencies() {
    log_step "检查依赖工具..."
    
    local missing=()
    for cmd in kubectl helm; do
        if ! command -v $cmd &> /dev/null; then
            missing+=($cmd)
        fi
    done
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "缺少依赖: ${missing[*]}"
        exit 1
    fi
    
    # 检查kubectl连接
    if ! kubectl cluster-info &> /dev/null; then
        log_error "无法连接到Kubernetes集群"
        exit 1
    fi
    
    log_info "依赖检查通过"
}

# 检查环境变量
check_env_vars() {
    log_step "检查环境变量..."
    
    local warnings=()
    
    # 必需的环境变量
    if [ -z "$POSTGRES_PASSWORD" ]; then
        log_warn "POSTGRES_PASSWORD 未设置, 将使用默认值"
        POSTGRES_PASSWORD="kernel-expert-pg-password"
    fi
    
    if [ -z "$REDIS_PASSWORD" ]; then
        log_warn "REDIS_PASSWORD 未设置, 将使用默认值"
        REDIS_PASSWORD="kernel-expert-redis-password"
    fi
    
    if [ -z "$JWT_SECRET" ]; then
        log_warn "JWT_SECRET 未设置, 将生成随机值"
        JWT_SECRET=$(openssl rand -hex 32)
    fi
    
    # 模型API密钥警告
    if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$DASHSCOPE_API_KEY" ]; then
        log_warn "未设置任何模型API密钥, 部分功能可能不可用"
    fi
    
    log_info "环境变量检查完成"
}

# 创建命名空间
create_namespace() {
    log_step "创建命名空间: $NAMESPACE"
    
    if kubectl get namespace $NAMESPACE &> /dev/null; then
        log_info "命名空间已存在"
    else
        kubectl create namespace $NAMESPACE
        log_info "命名空间创建成功"
    fi
}

# 添加Helm仓库
add_helm_repos() {
    log_step "添加Helm仓库..."
    
    helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
    helm repo update
    
    log_info "Helm仓库更新完成"
}

# 构建Helm values
build_values() {
    log_step "构建部署配置..."
    
    local values_files="-f $CHART_PATH/values.yaml"
    
    # 环境特定配置
    if [ -f "$CHART_PATH/values-$ENV.yaml" ]; then
        values_files="$values_files -f $CHART_PATH/values-$ENV.yaml"
        log_info "使用环境配置: values-$ENV.yaml"
    fi
    
    echo "$values_files"
}

# 构建set values
build_set_values() {
    local set_values=""
    
    # 数据库密码
    set_values="$set_values --set postgresql.auth.postgresPassword=$POSTGRES_PASSWORD"
    set_values="$set_values --set redis.auth.password=$REDIS_PASSWORD"
    set_values="$set_values --set secrets.jwtSecret=$JWT_SECRET"
    
    # 模型API密钥
    [ -n "$OPENAI_API_KEY" ] && set_values="$set_values --set secrets.openaiApiKey=$OPENAI_API_KEY"
    [ -n "$ANTHROPIC_API_KEY" ] && set_values="$set_values --set secrets.anthropicApiKey=$ANTHROPIC_API_KEY"
    [ -n "$DASHSCOPE_API_KEY" ] && set_values="$set_values --set secrets.dashscopeApiKey=$DASHSCOPE_API_KEY"
    [ -n "$ZHIPU_API_KEY" ] && set_values="$set_values --set secrets.zhipuApiKey=$ZHIPU_API_KEY"
    [ -n "$MOONSHOT_API_KEY" ] && set_values="$set_values --set secrets.moonshotApiKey=$MOONSHOT_API_KEY"
    [ -n "$DEEPSEEK_API_KEY" ] && set_values="$set_values --set secrets.deepseekApiKey=$DEEPSEEK_API_KEY"
    [ -n "$BAICHUAN_API_KEY" ] && set_values="$set_values --set secrets.baichuanApiKey=$BAICHUAN_API_KEY"
    [ -n "$ERNIE_API_KEY" ] && set_values="$set_values --set secrets.ernieApiKey=$ERNIE_API_KEY"
    [ -n "$ERNIE_SECRET_KEY" ] && set_values="$set_values --set secrets.ernieSecretKey=$ERNIE_SECRET_KEY"
    [ -n "$AGENT_API_KEY" ] && set_values="$set_values --set secrets.agentApiKey=$AGENT_API_KEY"
    
    echo "$set_values"
}

# 部署应用
deploy_app() {
    log_step "部署应用: $RELEASE_NAME"
    
    local values_files=$(build_values)
    local set_values=$(build_set_values)
    
    # 更新依赖
    log_info "更新Helm依赖..."
    helm dependency update $CHART_PATH
    
    # 执行部署
    log_info "执行Helm部署..."
    eval helm upgrade --install $RELEASE_NAME $CHART_PATH \
        --namespace $NAMESPACE \
        $values_files \
        $set_values \
        --wait \
        --timeout 10m
    
    log_info "部署命令执行完成"
}

# 等待Pod就绪
wait_for_pods() {
    log_step "等待Pod就绪..."
    
    local timeout=300
    local start_time=$(date +%s)
    
    while true; do
        local ready=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=kernel-expert \
            -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
        
        if [ "$ready" == "True" ] || [ "$ready" == "True True True" ]; then
            log_info "所有Pod已就绪"
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -ge $timeout ]; then
            log_error "等待Pod超时"
            kubectl get pods -n $NAMESPACE
            return 1
        fi
        
        echo -n "."
        sleep 5
    done
}

# 验证部署
verify_deployment() {
    log_step "验证部署..."
    
    echo ""
    log_info "Pod状态:"
    kubectl get pods -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME
    
    echo ""
    log_info "Service状态:"
    kubectl get svc -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME
    
    echo ""
    log_info "Ingress状态:"
    kubectl get ingress -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME 2>/dev/null || true
}

# 显示访问信息
show_access_info() {
    log_step "访问信息"
    
    echo ""
    echo "============================================"
    echo "  Linux内核专家Agent 部署完成"
    echo "============================================"
    echo ""
    
    # 获取Service信息
    local svc_ip=$(kubectl get svc $RELEASE_NAME -n $NAMESPACE -o jsonpath='{.spec.clusterIP}' 2>/dev/null)
    local svc_port=$(kubectl get svc $RELEASE_NAME -n $NAMESPACE -o jsonpath='{.spec.ports[0].port}' 2>/dev/null)
    
    if [ -n "$svc_ip" ]; then
        echo "内部访问地址: http://$svc_ip:$svc_port"
    fi
    
    # 获取Ingress信息
    local ingress_host=$(kubectl get ingress -n $NAMESPACE -l app.kubernetes.io/instance=$RELEASE_NAME \
        -o jsonpath='{.items[0].spec.rules[0].host}' 2>/dev/null)
    
    if [ -n "$ingress_host" ]; then
        echo "外部访问地址: https://$ingress_host"
        echo ""
        echo "API端点:"
        echo "  - REST API: https://$ingress_host/api/v1"
        echo "  - A2A协议:  https://$ingress_host/a2a"
        echo "  - MCP协议:  https://$ingress_host/mcp"
        echo "  - 健康检查: https://$ingress_host/health"
        echo "  - Metrics:  https://$ingress_host/metrics"
    fi
    
    echo ""
    echo "常用命令:"
    echo "  查看日志: kubectl logs -f -n $NAMESPACE -l app.kubernetes.io/component=agent"
    echo "  进入Pod:  kubectl exec -it -n $NAMESPACE \$(kubectl get pod -n $NAMESPACE -l app.kubernetes.io/component=agent -o name | head -1) -- /bin/sh"
    echo "  卸载:     helm uninstall $RELEASE_NAME -n $NAMESPACE"
    echo ""
}

# 主函数
main() {
    echo ""
    echo "=========================================="
    echo "  Linux内核专家Agent 部署脚本"
    echo "=========================================="
    echo ""
    log_info "环境: $ENV"
    log_info "命名空间: $NAMESPACE"
    log_info "Release: $RELEASE_NAME"
    echo ""
    
    check_dependencies
    check_env_vars
    create_namespace
    add_helm_repos
    deploy_app
    wait_for_pods
    verify_deployment
    show_access_info
    
    log_info "部署成功完成!"
}

# 执行
main
