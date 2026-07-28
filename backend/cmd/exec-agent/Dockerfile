# Executor image: a resident tools container that PentAGI execs into (instead of
# spawning per-flow Docker containers). Base is Kali so pentest tools are
# preinstalled; the exec-agent binary provides the HTTP control surface.
#
# Build from the backend/ directory:
#   docker build -f cmd/exec-agent/Dockerfile -t local/pentagi-executor:latest .

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
