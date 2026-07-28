# PentAGI (4YI marketplace build)

Autonomous AI-agent penetration-testing platform, packaged for the 4YI app
marketplace.

This branch deploys three services (see `docker-compose.yml`):

- **pentagi** — the public web/API service (Go backend that also serves the UI).
- **kali-executor** — internal tools runner; agent commands are executed here over
  HTTP via the executor backend (`EXECUTION_BACKEND=executor`), so no host
  container-runtime access is required.
- **scraper** — internal web-content fetch service.

PostgreSQL (with the pgvector extension) is provided as a platform-managed
service and wired in during the deployment proposal.

See `deploy/4yi/README.md` for the full deployment/onboarding notes and the
`xclaw.app.yaml` service manifest reference.

Upstream project: PentAGI by vxcontrol (MIT).
