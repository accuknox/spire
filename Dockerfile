# syntax = docker/dockerfile:1.6.0@sha256:ac85f380a63b13dfcefa89046420e1781752bab202122f8f50032edf31be0021

# Build stage
FROM --platform=${BUILDPLATFORM} harbor.do.accuknox.com/baseimage/golang:1.26.10-alpine3.24-fips-rfcurated AS base
USER root
WORKDIR /spire
RUN apk --no-cache --update add file bash pkgconfig git make curl ca-certificates \
    build-base musl-dev libseccomp-dev libcrypto3 libssl3
COPY go.* ./
# https://go.dev/ref/mod#module-cache
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .

FROM base AS builder
ARG TAG

# DYNAMIC build: the OpenSSL backend must dlopen libcrypto, so never static.
# Pin .go-version to the image's Go so the Makefile's go-check doesn't
# download upstream Go (which has no OpenSSL/FIPS backend).
ENV CGO_ENABLED=1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go env GOVERSION | sed 's/^go//' > .go-version && \
    make build git_tag="${TAG}" git_dirty="" && \
    ls -l bin && \
    file bin/spire-server bin/spire-agent bin/oidc-discovery-provider

###################################################################
# Runtime base: Alpine FIPS (musl + libcrypto present)
###################################################################
FROM --platform=${BUILDPLATFORM} harbor.do.accuknox.com/baseimage/alpine:3.24-fips-rfcurated AS spire-base
USER root
RUN apk add --no-cache ca-certificates libcrypto3 libssl3 libseccomp
WORKDIR /opt/spire
CMD []

# SPIRE Server
FROM spire-base AS spire-server
ARG spireuid=0
ARG spiregid=0
RUN mkdir -p /opt/spire/bin /etc/spire/server /run/spire/server/private \
        /tmp/spire-server/private /var/lib/spire/server && \
    chown -R ${spireuid}:${spiregid} /opt/spire /etc/spire /run/spire /tmp/spire-server /var/lib/spire
COPY --link --from=builder --chown=${spireuid}:${spiregid} --chmod=755 /spire/bin/spire-server /opt/spire/bin/
USER ${spireuid}:${spiregid}
ENTRYPOINT ["/opt/spire/bin/spire-server", "run"]

# SPIRE Agent (runs as root by default so the agent socket can be shared in Kubernetes)
FROM spire-base AS spire-agent
ARG spireuid=0
ARG spiregid=0
RUN mkdir -p /opt/spire/bin /etc/spire/agent /run/spire/agent/public \
        /tmp/spire-agent/public /var/lib/spire/agent && \
    chown -R ${spireuid}:${spiregid} /opt/spire /etc/spire /run/spire /tmp/spire-agent /var/lib/spire
COPY --link --from=builder --chown=${spireuid}:${spiregid} --chmod=755 /spire/bin/spire-agent /opt/spire/bin/
USER ${spireuid}:${spiregid}
ENTRYPOINT ["/opt/spire/bin/spire-agent", "run"]

# OIDC Discovery Provider
FROM spire-base AS oidc-discovery-provider
ARG spireuid=1000
ARG spiregid=1000
RUN mkdir -p /opt/spire/bin && chown -R ${spireuid}:${spiregid} /opt/spire
COPY --link --from=builder --chown=${spireuid}:${spiregid} --chmod=755 /spire/bin/oidc-discovery-provider /opt/spire/bin/
USER ${spireuid}:${spiregid}
ENTRYPOINT ["/opt/spire/bin/oidc-discovery-provider"]
