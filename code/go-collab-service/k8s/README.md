# K8s 部署配置说明

## 目录挂载

Go 服务部署在 K8s 中需要挂载以下宿主机目录：

### 1. OpenClaw 数据目录
- **宿主机路径**: `/home/victor/.openclaw`
- **容器内路径**: `/root/.openclaw`
- **用途**: 存储 OpenClaw 的 agent 配置、会话记录等
- **权限**: 只读

### 2. Git 仓库 Base 目录
- **宿主机路径**: `/home/victor/base/git`
- **容器内路径**: `/home/victor/base/git`
- **用途**: 存储所有 Git 仓库的基目录
- **权限**: 只读
- **说明**: 项目配置的代码仓库路径必须在此目录下

## 环境变量

### GIT_BASE_DIR
- **值**: `/home/victor/base/git`
- **用途**: 指定 Git 仓库的基目录，用于路径验证

### OPENCLAW_DIR
- **值**: `/root/.openclaw`
- **用途**: 指定 OpenClaw 数据目录

## 路径验证规则

为了保证安全性，服务会对项目配置的仓库路径进行验证：

### 允许的 Base 目录
只有以下目录内的路径是被允许的：
1. `/home/victor/base/git` - Git 仓库目录
2. `/home/victor/.openclaw` - OpenClaw 目录
3. `/tmp` - 临时目录
4. `/root/.openclaw` - Pod 内的 OpenClaw 目录

### 验证逻辑
```go
// 示例：合法的路径
/home/victor/base/git/myproject/backend      ✅
/home/victor/base/git/myproject/frontend     ✅
/home/victor/.openclaw/workspace             ✅

// 示例：非法的路径
/etc/passwd                                   ❌
/home/victor/other                            ❌
/root/something                               ❌
```

## 文档目录结构

所有项目的文档输出使用统一的目录结构：

```
<docs_base_dir>/
├── <project_id>/
│   ├── designs/    # 功能设计文档
│   ├── research/   # 调研分析报告
│   ├── system/     # 模块实现文档
│   └── reports/    # AI 任务汇总报告
```

### 示例配置
```yaml
# 项目配置示例
{
  "id": "db-k8s-platform",
  "name": "数据库 K8s 部署平台",
  "docs_base_dir": "/home/victor/docs",  # 文档根目录
  "repositories": {
    "code_repos": [
      {
        "name": "backend",
        "type": "main",
        "git_url": "https://github.com/org/db-k8s-backend.git",
        "local_path": "/home/victor/base/git/db-k8s-platform/backend"
      },
      {
        "name": "frontend",
        "type": "reference",
        "git_url": "https://github.com/org/db-k8s-frontend.git",
        "local_path": "/home/victor/base/git/db-k8s-platform/frontend"
      }
    ]
  }
}
```

## 安全说明

1. **只读挂载**: 所有宿主机目录都以只读方式挂载，防止意外修改
2. **路径白名单**: 只允许访问配置的 base 目录，防止路径遍历攻击
3. **路径验证**: 保存项目配置时会验证路径是否存在且在允许的范围内
4. **警告模式**: 在 K8s 环境中，如果路径暂时不存在，只记录警告不阻止创建

## 故障排查

### 问题：路径验证失败
```
error: code_repos[0]: path /xxx/yyy is not in allowed base directories
```
**解决方案**: 确保配置的 `local_path` 在允许的 base 目录内

### 问题：目录不存在警告
```
[WARN] code_repos[0]: path does not exist yet: /home/victor/base/git/xxx (may be created later)
```
**说明**: 这是正常警告，表示目录还未创建，不影响服务运行

### 问题：无法访问挂载目录
```
permission denied
```
**解决方案**: 检查宿主机的目录权限，确保 K8s Pod 有读取权限
