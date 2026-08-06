# pgvector as a build-context service for the 4YI app-platform declaration
# (4yi-app.json). The platform's declarative service schema only accepts
# build services (dockerfile + context), not registry-image managed datastores,
# so we wrap the upstream pgvector image in a trivial build. It runs as an
# internal service with a persistent volume (see 4yi-app.json → services →
# pgvector → storage). Functionally identical to the previously hand-mapped
# `type: postgres` managed service.
FROM vxcontrol/pgvector:latest
