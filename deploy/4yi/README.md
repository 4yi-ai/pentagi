# PentAGI on the 4YI marketplace — deployment structure

This directory holds the **platform-conforming shape** of PentAGI so it can be
imported and deployed as a 4YI marketplace app (dedicated app, per-org install),
where apps run as **unprivileged Kubernetes pods with no Docker socket**.

## Why the repo needed restructuring

The successfully-listed apps (`4yi-ai/screenshot-to-code`, `4yi-ai/4yi-cad`)
share a structure the importer expects:

| Convention | screenshot-to-code | PentAGI (stock) | PentAGI (this) |
| --- | --- | --- | --- |
| Services built via `build: {context,dockerfile}` | ✅ | ❌ uses prebuilt `image:` | ✅ |
| Exactly one **public** service | ✅ frontend | ❌ several + spawns sandboxes | ✅ `pentagi` |
| **No** self-bundled stateful datastore | ✅ (no DB) | ❌ bundles `pgvector` | ✅ postgres declared as managed service |
| No Docker socket / privileged | ✅ | ❌ mounts docker.sock, spawns Kali | ✅ executor backend |

Stock PentAGI fails all four; this deployment shape fixes all four.

## The two changes that make it fit

1. **Execution backend** (`EXECUTION_BACKEND=executor`, implemented in
   `backend/pkg/executor` + `backend/cmd/exec-agent`): PentAGI no longer spawns a
   Docker container per flow. It runs commands by calling the resident
   `kali-executor` service over HTTP. No docker.sock, no privileged access.

2. **Service topology** (`docker-compose.yml` here):
   - `pentagi` — the single **public** web/API service (built from the root Dockerfile).
   - `kali-executor` — **internal** resident tools container (Kali + exec-agent),
     built from `backend/cmd/exec-agent/Dockerfile`.
   - `scraper` — **internal** service.
   - `pgvector` — **NOT** a container here. It is declared as a platform-managed
     `postgres` service during import (see below).

## How to import (operator steps)

1. **Publish a dedicated marketplace branch** of `4yi-ai/pentagi` whose **root**
   `docker-compose.yml` is `deploy/4yi/docker-compose.yml` (the importer only
   auto-detects a root compose; the build contexts here are written relative to
   the repo root for this reason).
2. In 4YI admin → **AI Marketplace Import**, enter the fork URL + that branch,
   deployment type = **Dedicated app**, click **Create**.
3. **AI Analysis / Deployment Proposal**: the scanner will flag the multi-service
   layout and (if pgvector is present) stateful services. In the proposal editor,
   shape the manifest to match `xclaw.app.yaml` below — in particular add the
   managed `postgres` service and route pgvector's connection to it.
4. **Secrets**: provide `EXECUTOR_TOKEN`, the LLM gateway key (`LLM_SERVER_KEY`),
   the postgres password, and `COOKIE_SIGNING_SALT` as platform secrets — the
   consumer fills them at install time.
5. **Release → Publish Gate**: smoke test hits `pentagi` `/healthz`. Ensure the
   gateway api-base/token and default models are configured for the shared-app
   runtime.

## Target manifest (`xclaw.app.yaml` v1 shape)

The Deployment Proposal should end up equivalent to:

```yaml
version: 1
app:
  name: pentagi
  type: dedicated_app
services:
  - name: pentagi
    type: web
    imageSource: build
    build: { context: ., dockerfile: Dockerfile }
    port: 8443
    route: public
    healthPath: /healthz
    resources: { cpu: 1, memoryMb: 2048, replicas: { min: 1, max: 1 } }
    env:
      EXECUTION_BACKEND: "executor"
      EXECUTOR_URL: "http://kali-executor:8022"
      LLM_SERVER_URL: "https://<gateway>/api/v1"
      LLM_SERVER_MODEL: "anthropic.claude-sonnet-4-6"
    secrets:
      EXECUTOR_TOKEN: "secret:executor-token"
      LLM_SERVER_KEY: "secret:llm-server-key"
      PENTAGI_POSTGRES_PASSWORD: "secret:pg-password"
      COOKIE_SIGNING_SALT: "secret:cookie-salt"

  - name: pgvector
    type: postgres            # platform-managed stateful service (allowed for dedicated apps)
    imageSource: registry
    image: vxcontrol/pgvector:latest
    port: 5432
    route: internal
    storage: { sizeGb: 10, mountPath: /var/lib/postgresql/data }
    resources: { cpu: 0.5, memoryMb: 1024, replicas: { min: 1, max: 1 } }
    secrets: { POSTGRES_PASSWORD: "secret:pg-password" }

  - name: kali-executor
    type: api
    imageSource: build
    build: { context: backend, dockerfile: cmd/exec-agent/Dockerfile }
    port: 8022
    route: internal
    healthPath: /v1/health
    resources: { cpu: 1, memoryMb: 2048, replicas: { min: 1, max: 1 } }

  - name: scraper
    type: api
    imageSource: build
    build: { context: deploy/4yi/scraper, dockerfile: Dockerfile }
    port: 443
    route: internal
    healthPath: /
    resources: { cpu: 0.5, memoryMb: 512, replicas: { min: 1, max: 1 } }
```

Constraint check: exactly one `route: public` (pentagi); pgvector is a managed
`postgres` service (allowed for `dedicated_app`, not a bundled compose datastore).

## Open items before a real deploy

- Confirm the platform allows a custom image for `type: postgres` (pgvector). If
  not, use the platform's base postgres and enable the `vector` extension on boot,
  or switch to a platform-managed vector store.
- Add a `/healthz` endpoint to the pentagi server (smoke test target).
- `nmap -sS` needs `CAP_NET_RAW`; on an unprivileged pod use `-sT` (TCP connect)
  or request that single capability.
- 4YI has no embeddings endpoint → vector memory is degraded unless a platform
  embedding is wired in.
