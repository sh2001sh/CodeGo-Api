# codego-api

面向团队与开发者的 API 统一管理平台。

<p align="center">
  <a href="https://shu26.cfd/">官网</a> ·
  <a href="https://github.com/sh2001sh/CodeGo-Api">GitHub</a> ·
  <a href="https://hub.docker.com/r/s2644752646/codego-api">Docker Hub</a>
</p>

<p align="center">
  <a href="https://github.com/sh2001sh/CodeGo-Api/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/sh2001sh/CodeGo-Api" alt="AGPLv3 license">
  </a>
  <a href="https://hub.docker.com/r/s2644752646/codego-api">
    <img src="https://img.shields.io/docker/pulls/s2644752646/codego-api?logo=docker&logoColor=white" alt="Docker pulls">
  </a>
</p>

codego-api 用于集中管理 API provider、模型路由、访问策略、API keys、额度和使用审计。它强调控制面与数据面分离、独立 worker、可组合的 provider 适配器以及可验证的额度账本。

codego-api 是 API 统一管理平台，不是 API 中转站或 API 转售服务。部署方应确保已获得所接入 API、账号、密钥和模型服务的合法授权。

## 核心能力

- **统一 API 接入**：提供 OpenAI Chat Completions、OpenAI Responses 等兼容接口，并支持 Claude、Gemini 及多种多模态能力。
- **Provider 与渠道管理**：集中维护 provider、渠道、模型映射、API keys、模型价格和可用状态。
- **路由与容错**：通过路由池、分组、fallback、限流和渠道健康状态控制请求流量。
- **访问控制**：管理用户、组织分组、API keys、权限、OAuth/Passkey 和安全策略。
- **用量与账本**：记录请求、错误、模型用量和运行状态；使用单一普通钱包、额度账本和幂等结算流程。
- **异步工作流**：使用 Temporal 和独立 workflow worker 处理长任务、轮询任务及可靠后台流程。
- **可观测与审计**：提供使用日志、配额统计、错误审计、性能指标和运维配置。
- **容器化部署**：control API、gateway API、workflow worker、ledger worker 和迁移工具可以独立部署。

## 架构概览

```text
                    +----------------------+
                    |      control-api      |
                    | 管理、认证、配置、审计 |
                    +----------+-----------+
                               |
                         PostgreSQL / Redis
                               |
+----------+       +-----------v-----------+       +------------------+
| API 客户端 | ----> |      gateway-api      | ----> | Provider APIs    |
+----------+       | 路由、适配、流式响应   |       | OpenAI/Claude/...|
                    +-----------+-----------+       +------------------+
                                |
                         用量事件 / 结算事件
                                |
                    +-----------v-----------+
                    |     ledger-worker      |
                    | 额度、账本、结算一致性 |
                    +------------------------+

                    +------------------------+
                    |    workflow-worker     |
                    | Temporal 持久化工作流  |
                    +------------------------+
```

## 运行组件

| 组件            | 职责                                                                    |
| --------------- | ----------------------------------------------------------------------- |
| control-api     | 控制面 API 和 Web 控制台，负责用户、渠道、模型、策略和审计配置。        |
| gateway-api     | 数据面 API，负责请求认证、模型路由、provider 适配、流式响应和用量记录。 |
| workflow-worker | 执行 Temporal 工作流及长耗时、轮询型任务。                              |
| ledger-worker   | 处理额度账本、结算事件和账本一致性。                                    |
| db-migrate      | 初始化数据库并执行版本迁移及审计读模型迁移。                            |
| v2-verify       | 检查迁移、钱包账户、Token 账户、订阅账户、账本和待处理 outbox 事件。    |

## 快速开始

### Docker Compose

生产 Compose 默认使用 Docker Hub 镜像：

```text
docker.io/s2644752646/codego-api
```

```bash
git clone https://github.com/sh2001sh/CodeGo-Api.git
cd CodeGo-Api
cp .env.example .env
```

编辑 .env，Docker Compose 环境中的服务地址应使用 Compose 服务名：

```dotenv
SQL_DSN=postgresql://codegoapi:replace-with-a-strong-password@postgres:5432/codegoapi
REDIS_CONN_STRING=redis://redis:6379/0
TEMPORAL_HOSTPORT=temporal:7233
POSTGRES_PASSWORD=replace-with-a-strong-password
SESSION_SECRET=replace-with-a-random-secret
```

SQL_DSN 中的数据库密码必须与 POSTGRES_PASSWORD 保持一致。生产环境请替换为强密码和随机 SESSION_SECRET。

启动服务：

```bash
docker compose pull
docker compose up -d
docker compose ps
```

控制台默认地址：<http://localhost:3000>

运行可选的一致性检查：

```bash
docker compose --profile verify run --rm v2-verify --strict
```

如果 Docker Hub 仓库设置为私有，请先执行：

```bash
docker login docker.io
```

### 开发环境

开发环境使用本地源码构建后端，前端使用 Rsbuild：

```bash
docker compose -f docker-compose.dev.yml up -d --build
cd web/default
pnpm install
pnpm dev
```

- 后端控制 API：<http://localhost:3000>
- 前端开发服务：<http://localhost:3001>

## Docker 镜像

GitHub Actions 会在 main 更新时构建 amd64/arm64 镜像，并推送到 [Docker Hub](https://hub.docker.com/r/s2644752646/codego-api)。当前镜像标签包括：

```text
docker.io/s2644752646/codego-api:latest-control-api
docker.io/s2644752646/codego-api:latest-gateway-api
docker.io/s2644752646/codego-api:latest-workflow-worker
docker.io/s2644752646/codego-api:latest-ledger-worker
docker.io/s2644752646/codego-api:latest-db-migrate
docker.io/s2644752646/codego-api:latest-v2-verify
```

维护者需要在 GitHub 仓库的 Actions Secrets 中配置：

- DOCKERHUB_USERNAME
- DOCKERHUB_TOKEN

Token 应使用 Docker Hub 的具有镜像推送权限的 Access Token，不要提交到仓库。

## API 文档

- [Control API OpenAPI 文档](./docs/openapi/api.json)
- [Gateway API OpenAPI 文档](./docs/openapi/relay.json)
- [环境变量示例](./.env.example)

Gateway API 当前覆盖文本对话、Responses、Claude、Gemini、图像、音频、Embedding、Rerank、视频和实时连接等 provider 能力，具体可用接口以 provider 配置和 OpenAPI 文档为准。

## 开发与测试

后端测试：

```bash
go test -p 1 ./...
```

前端常用命令：

```bash
cd web/default
pnpm typecheck
pnpm lint
pnpm build
```

代码结构：

```text
cmd/                 可独立构建的运行组件入口
internal/gateway/    provider 执行、路由、流式响应和渠道运行时
internal/identity/   用户、Token、认证和会话
internal/adminops/   控制面管理操作
internal/audit/      使用日志、统计和审计读模型
internal/billing/    额度账本、结算和 outbox
internal/workflow/   Temporal 工作流和任务
internal/platform/   数据库、缓存、安全、限流和基础设施
web/default/         默认 Web 控制台
docs/openapi/        Control API 与 Gateway API 接口描述
```

## 安全与合规

- 只接入部署方已获得合法授权的 provider API、账号、密钥和额度。
- 不要将 .env、API keys、数据库密码或 Docker Token 提交到 Git。
- 生产环境请配置强密码、随机会话密钥、可信代理和安全 Cookie。
- 安全问题请参考 [SECURITY.md](./.github/SECURITY.md)，不要在公开 Issue 中提交敏感信息。

## 许可证

codego-api 使用 [GNU Affero General Public License v3.0](./LICENSE) 发布。第三方依赖及其许可证见 [THIRD-PARTY-LICENSES.md](./THIRD-PARTY-LICENSES.md)。

## 官网与社区

- 官网：<https://shu26.cfd/>
- GitHub：<https://github.com/sh2001sh/CodeGo-Api>
- Docker Hub：<https://hub.docker.com/r/s2644752646/codego-api>
