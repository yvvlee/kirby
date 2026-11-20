# Kirby

<p align="center">
  <strong>面向微服务与云原生架构的现代、轻量级、多租户配置管理平台。</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8.svg" alt="Go Version"></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/React-19-61DAFB.svg" alt="React 19"></a>
  <a href="https://hub.docker.com/r/mshadow/kirby"><img src="https://img.shields.io/badge/Docker-mshadow%2Fkirby-2496ED.svg" alt="Docker Image"></a>
</p>

<p align="center">
  <a href="README.md">English (默认)</a> | <b>简体中文</b>
</p>

---

## 项目简介

Kirby 是一个现代、独立的统一配置管理平台，专为微服务架构和云原生应用设计。单套 Kirby 部署即可管理多个独立项目，每个项目包含完全隔离的环境（如开发、测试、预发布、生产等）。

Kirby 基于无状态的 Go 后端与现代化的 React 19 管理后台构建，提供涵盖配置 Schema 约束、草稿编辑、快照版本控制、导入导出、权限控制，以及高并发、低延迟的 HTTP 与 gRPC 运行时分发全生命周期管理能力。

## 核心特性

- 🏢 **多项目与多环境隔离**：严格的数据层级划分（`项目 -> 环境 -> 配置`）。不同环境之间配置与鉴权凭据完全隔离，同一项目下的不同环境支持配置同名且互不干扰。
- ⚡ **高性能双协议运行时**：提供毫秒级延迟的 HTTP 和 gRPC 配置拉取 API，内置 Redis 缓存加速，通过环境专属 API Key 严格鉴权。
- 🎨 **现代化管理控制台**：基于 React 19、TypeScript 和 Ant Design 构建，内置 Monaco Editor 代码编辑器，支持 JSON Schema 格式校验及配置可视化 Diff 比对。
- 🔒 **细粒度权限控制 (RBAC)**：项目角色（Owner / Maintainer / Member / Guest）与环境权限矩阵（读、写、发布/下线、API Key 管理）双层隔离，严格防止越权。
- 📦 **配置快照与原子回滚**：支持草稿与发布态分离、版本快照保存、跨环境导入导出以及零停机原子回滚。
- 🗄️ **S3 兼容对象存储**：集成上传凭证与同源反向代理机制，安全管理配置关联的静态资源与文件资产。
- 🌐 **全栈原生国际化 (i18n)**：管理后台界面与后端业务错误响应原生支持简体中文 (`zh-CN`) 与英文 (`en-US`) 无缝切换。
- 🚀 **云原生与极简部署**：单进程 Scratch 极简容器镜像，首次启动自动执行数据库表结构初始化，支持 MySQL 8.4 与 PostgreSQL。

## 系统架构

Kirby 在架构上划分为两个独立的平面：

```text
               ┌────────────────────────────────────────────────────────┐
               │                      Kirby Server                      │
               │                                                        │
浏览器    ─────►│  管理平面 (HTTP / React 19 Web App)                    │
               │  - 用户 JWT 会话鉴权                                   │
               │  - 项目、环境及成员权限管理                            │
               │  - 配置结构、草稿编辑、快照比对与版本发布              │
               │                                                        │
业务微服务 ─────►│  运行时平面 (HTTP / gRPC)                             │
               │  - 环境专属 API Key 鉴权                               │
               │  - 高并发、低延迟配置分发 (Redis 缓存加速)             │
               └───────────┬───────────────────┬───────────────────┬────┘
                           │                   │                   │
                           ▼                   ▼                   ▼
                  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
                  │ 关系型数据库     │ │ Redis 缓存      │ │ 对象存储        │
                  │ (MySQL / PgSQL) │ │ (缓存/限流计数) │ │ (S3 / MinIO)    │
                  └─────────────────┘ └─────────────────┘ └─────────────────┘
```

- **管理平面**：Web 控制台与管理 HTTP API。用户通过 JWT 认证，系统根据项目角色与环境权限矩阵进行授权。
- **运行时平面**：面向业务微服务集群，通过环境绑定的 API Key 提供只读的高性能 HTTP 和 gRPC 配置获取接口。
- **存储层**：关系型数据库存储所有持久化业务状态与审计记录；Redis 提供运行时配置缓存和限流；对象存储负责文件资产。

## 快速开始

### 方式一：Docker Compose（推荐体验）

一键在本地启动完整的 Kirby 运行栈（包含 Kirby 进程、MySQL、Redis 及 MinIO）：

```sh
# 1. 克隆代码仓库
git clone https://github.com/mshadow/kirby.git && cd kirby

# 2. 准备环境变量配置
cp deploy/.env.example deploy/.env

# 3. 启动所有服务
./scripts/compose-up.sh
```

服务启动后：
- 浏览器打开 `http://localhost:8000/setup` 创建初始系统管理员账号。
- 需要停止时运行 `./scripts/compose-down.sh`。

### 方式二：本地源码开发

适合对 Go 后端或 React 前端进行二次开发与贡献：

**环境要求**：
- Go 1.27+
- Node.js 24+ 与 npm
- 运行中的 MySQL 8.4 和 Redis 7 实例

```sh
# 1. 准备本地配置与对象存储目录
mkdir -p .local/objects
cp deploy/config.local.example.yaml .local/config.yaml

# 2. 启动 Go 后端（自动完成数据库表结构与密钥初始化）
cd server
go run . serve --config ../.local/config.yaml

# 3. 启动 React 前端开发服务器（在另一个终端窗口中执行）
cd web
npm ci
KIRBY_DEV_API_TARGET=http://127.0.0.1:8080 npm run dev
```

在浏览器中打开 `http://localhost:15173/setup` 完成首次管理员初始化，然后继续登录。

> [!TIP]
> 也可以通过命令行直接创建管理员账号：
> ```sh
> cd server
> go run . create-admin --config ../.local/config.yaml \
>   --username admin --nickname Administrator --email admin@example.com
> ```

## 运行时 API 读取示例

业务客户端使用在 Web 控制台中为环境创建的 API Key 读取已发布配置：

### HTTP 请求

```sh
curl -H "X-Kirby-API-Key: YOUR_API_KEY" \
  "http://127.0.0.1:8080/api/v1/config?key=AppConfig"
```

### gRPC 请求

```sh
grpcurl -plaintext \
  -H "x-kirby-api-key: YOUR_API_KEY" \
  -d '{"key":"AppConfig"}' \
  127.0.0.1:9090 kirby.runtime.v1.Api/Config
```

> API Key 在创建时已严格绑定目标项目与环境，请求参数中只需传入具体的配置 `key`。

## 文档索引

> 📖 **语言说明**：项目 README 提供 [English (默认)](README.md) 与 [简体中文](README.zh-CN.md) 双语对照版本。[`docs/`](docs/) 目录下的深入技术文档目前提供英文版本。

详细的技术文档均维护在 [`docs/`](docs/) 目录中：

| 分类 | 文档 | 说明 |
|---|---|---|
| **架构与安全** | [系统架构](docs/architecture.md) | 系统拓扑、双平面划分及数据流转 |
| | [权限模型](docs/authorization.md) | 角色定义、环境权限矩阵、成员邀请与项目删除 |
| | [环境数据隔离](docs/environment-isolation.md) | 数据归属模型与数据库表隔离要求 |
| | [安全设计](docs/security.md) | 安全边界、凭据存储及会话管理 |
| | [威胁模型](docs/threat-model.md) | 受保护资产与信任边界分析 |
| **部署与运维** | [生产部署指南](docs/deployment.md) | 生产环境拓扑、Ingress 路由与反向代理规范 |
| | [配置参数参考](docs/configuration.md) | 完整配置文件选项与环境变量映射 |
| | [日常运维指南](docs/operations.md) | 健康检查、日志格式、备份恢复与故障排查 |
| | [Docker Compose 指南](deploy/README.md) | 容器编排、S3 存储桶策略与生命周期规则 |
| **开发与扩展** | [本地开发指南](docs/development.md) | 工具链、本地调试、Protobuf 代码生成与测试套件 |
| | [环境 API Key 规范](docs/api-key.md) | 运行时凭据创建、安全存储、轮转与吊销 |
| | [快照导入与导出](docs/import-export.md) | 跨环境/跨部署配置迁移机制 |
| | [国际化规范](docs/internationalization.md) | 前后端多语言国际化方案 (zh-CN / en-US) |
| | [开源源码追溯](docs/source-provenance.md) | 代码来源与开源合规说明 |

## 参与贡献

欢迎提交 Issue 和 Pull Request！参与贡献前请参阅 [CONTRIBUTING.md](CONTRIBUTING.md) 与 [本地开发指南](docs/development.md)。

## 开源协议

Kirby 基于 [MIT 协议](LICENSE) 开源。第三方依赖与相关声明参见 [NOTICE](NOTICE)、[第三方开源协议说明](docs/third-party-licenses.md) 及 `dist/sbom`。
