# codego-api

<p align="center">
  <img src="./web/default/public/code-go-logo.svg" alt="codego-api logo" width="128">
</p>

<h1 align="center">codego-api</h1>

<p align="center">An API unified management platform for teams and developers</p>

<p align="center">
  <a href="./README.md">简体中文</a>
</p>

<p align="center">
  <a href="https://shu26.cfd/">Website</a> ·
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

codego-api centralizes API providers, model routing, access policies, API keys,
quotas, and usage audit in one operational platform. Its architecture separates
the control plane from the data plane, keeps workers independently deployable,
and makes provider adapters and quota settlement explicit.

codego-api is an API unified management platform, not an API reseller or
intermediary service. Operators are responsible for ensuring that they have
the lawful authorization to use every connected API, account, key, and model.

## Features

- **Unified API access**: OpenAI Chat Completions, OpenAI Responses, Claude,
  Gemini, and multimodal provider capabilities through a consistent platform.
- **Provider and channel management**: Maintain providers, channels, model
  mappings, API keys, model pricing, and availability from one console.
- **Routing and resilience**: Use route pools, groups, fallback policies,
  rate limits, and channel health state to control traffic.
- **Access control**: Manage users, groups, API keys, permissions, OAuth,
  Passkeys, and security policies.
- **Usage and ledger**: Record requests, errors, model usage, and runtime
  state with one ordinary wallet model, a quota ledger, and idempotent
  settlement flows.
- **Asynchronous workflows**: Use Temporal and an independent workflow worker
  for long-running and polling tasks.
- **Observability and audit**: Inspect usage logs, quota statistics, errors,
  performance metrics, and operational settings.
- **Containerized deployment**: Deploy the control API, gateway API, workflow
  worker, ledger worker, and migration tools independently.

## Architecture

```text
                    +----------------------+
                    |      control-api     |
                    | admin, auth, config,  |
                    | and audit control     |
                    +----------+-----------+
                               |
                         PostgreSQL / Redis
                               |
+----------+       +-----------v-----------+       +------------------+
| API      | ----> |      gateway-api      | ----> | Provider APIs    |
| clients  |       | routing, adapters,    |       | OpenAI/Claude/...|
+----------+       | and streaming        |       +------------------+
                    +-----------+-----------+
                                |
                         usage / settlement events
                                |
                    +-----------v-----------+
                    |     ledger-worker     |
                    | quota and settlement  |
                    | consistency           |
                    +-----------------------+

                    +-----------------------+
                    |    workflow-worker    |
                    | Temporal workflows    |
                    +-----------------------+
```

## Components

| Component         | Responsibility                                                                   |
| ----------------- | -------------------------------------------------------------------------------- |
| `control-api`     | Control-plane API and Web console for users, channels, models, policies, audit.  |
| `gateway-api`     | Data-plane API for authentication, routing, provider adapters, streaming, usage. |
| `workflow-worker` | Temporal workflows and long-running or polling tasks.                            |
| `ledger-worker`   | Quota ledger, settlement events, and ledger consistency.                         |
| `db-migrate`      | Database initialization and schema/read-model migrations.                        |
| `v2-verify`       | Migration, wallet, token account, subscription, ledger, and outbox checks.       |

## Quick Start

### Docker Compose

The production Compose setup uses the Docker Hub image:

```text
docker.io/s2644752646/codego-api
```

```bash
git clone https://github.com/sh2001sh/CodeGo-Api.git
cd CodeGo-Api
cp .env.example .env
```

Set service names for Docker Compose dependencies in `.env`:

```dotenv
SQL_DSN=postgresql://codegoapi:replace-with-a-strong-password@postgres:5432/codegoapi
REDIS_CONN_STRING=redis://redis:6379/0
TEMPORAL_HOSTPORT=temporal:7233
POSTGRES_PASSWORD=replace-with-a-strong-password
SESSION_SECRET=replace-with-a-random-secret
```

The database password in `SQL_DSN` must match `POSTGRES_PASSWORD`. Replace
the example password and `SESSION_SECRET` with strong production values.

Start the platform:

```bash
docker compose pull
docker compose up -d
docker compose ps
```

The console is available at <http://localhost:3000>.

Run the optional consistency check:

```bash
docker compose --profile verify run --rm v2-verify --strict
```

If the Docker Hub repository is private, authenticate first:

```bash
docker login docker.io
```

### Development

The development Compose setup builds the backend locally. The frontend uses
Rsbuild:

```bash
docker compose -f docker-compose.dev.yml up -d --build
cd web/default
pnpm install
pnpm dev
```

- Control API: <http://localhost:3000>
- Frontend development server: <http://localhost:3001>

## Docker Images

GitHub Actions builds amd64 and arm64 images on updates to `main` and pushes
them to [Docker Hub](https://hub.docker.com/r/s2644752646/codego-api). The
published service tags are:

```text
docker.io/s2644752646/codego-api:latest-control-api
docker.io/s2644752646/codego-api:latest-gateway-api
docker.io/s2644752646/codego-api:latest-workflow-worker
docker.io/s2644752646/codego-api:latest-ledger-worker
docker.io/s2644752646/codego-api:latest-db-migrate
docker.io/s2644752646/codego-api:latest-v2-verify
```

Repository maintainers must configure these GitHub Actions secrets:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

Use a Docker Hub Access Token with image push permission. Never commit it to
the repository.

## API Documentation

- [Control API OpenAPI specification](./docs/openapi/api.json)
- [Gateway API OpenAPI specification](./docs/openapi/relay.json)
- [Environment variable example](./.env.example)

The Gateway API covers text chat, Responses, Claude, Gemini, image, audio,
embedding, rerank, video, and realtime provider capabilities. Actual
availability depends on provider configuration and the OpenAPI specifications.

## Development and Testing

Backend tests:

```bash
go test -p 1 ./...
```

Common frontend checks:

```bash
cd web/default
pnpm typecheck
pnpm lint
pnpm build
```

Repository structure:

```text
cmd/                 Entrypoints for independently deployable services
internal/gateway/    Provider execution, routing, streaming, channel runtime
internal/identity/   Users, tokens, authentication, and sessions
internal/adminops/   Control-plane management operations
internal/audit/      Usage logs, statistics, and audit read models
internal/billing/    Quota ledger, settlement, and outbox processing
internal/workflow/   Temporal workflows and task providers
internal/platform/   Database, cache, security, rate limiting, infrastructure
web/default/         Default Web console
docs/openapi/        Control API and Gateway API specifications
```

## Security and Compliance

- Connect only provider APIs, accounts, keys, and quotas that you are
  authorized to use.
- Never commit `.env` files, API keys, database passwords, or Docker tokens.
- Use strong passwords, a random session secret, a trusted proxy, and secure
  cookies in production.
- Report security issues privately through [SECURITY.md](./.github/SECURITY.md)
  instead of public Issues.

## License

codego-api is released under the
[GNU Affero General Public License v3.0](./LICENSE). See
[THIRD-PARTY-LICENSES.md](./THIRD-PARTY-LICENSES.md) for the direct dependency
license inventory. The inherited source attribution and license notices in
[`NOTICE`](./NOTICE) must be preserved in derivative distributions.

## Links

- Website: <https://shu26.cfd/>
- GitHub: <https://github.com/sh2001sh/CodeGo-Api>
- Docker Hub: <https://hub.docker.com/r/s2644752646/codego-api>
