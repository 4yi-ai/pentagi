# Executor image (exec-agent), built from the backend/ module context.
#
# This is a sibling of cmd/exec-agent/Dockerfile with identical build steps.
# It lives at the backend/ root on purpose: the 4yi app-platform release
# preflight requires a service's build.context to equal the directory that
# contains its Dockerfile. exec-agent must be built with the whole backend Go
# module as context (it compiles ./cmd/exec-agent against the module), so the
# Dockerfile has to sit at backend/ for context "backend" to be accepted.
#
# Local equivalent:
#   docker build -f backend/Dockerfile -t local/pentagi-executor:latest backend

# --- build stage ---------------------------------------------------------
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/exec-agent ./cmd/exec-agent

# --- runtime stage -------------------------------------------------------
FROM vxcontrol/kali-linux:latest
COPY --from=build /out/exec-agent /usr/local/bin/exec-agent
ENV WORK_ROOT=/work \
    EXECUTOR_LISTEN=:8022
RUN mkdir -p /work
EXPOSE 8022
ENTRYPOINT ["/usr/local/bin/exec-agent"]
