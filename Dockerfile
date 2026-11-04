# syntax = docker/dockerfile:1.4.2@sha256:443aab4ca21183e069e7d8b2dc68006594f40bddf1b15bbd83f5137bd93e80e2

# Build stage
FROM --platform=${BUILDPLATFORM} harbor.do.accuknox.com/baseimage/golang:1.26.10-alpine3.24-fips-rfcurated AS base

USER root

WORKDIR /spire
RUN apk --no-cache --update add file bash pkgconfig git make curl ca-certificates \
    build-base musl-dev libseccomp-dev libcrypto3 libssl3
COPY go.* ./
COPY . .

# https://go.dev/ref/mod#module-cache
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM --platform=${BUILDPLATFORM} base AS builder

USER root

RUN mkdir -p /config/plugin/
ARG TARGETPLATFORM
ARG TARGETARCH

# For users that wish to run SPIRE containers as a non-root user,
# provide a default unprivileged user such that the default paths
# that SPIRE will try to read from, write to, and create at runtime
# can be given the correct file ownership/permissions at build time.
ARG spireuid=1000
ARG spiregid=1000

# Set up directories that SPIRE expects by default
# Set up base directories
RUN install -d -o root -g root -m 777 /spireroot
RUN install -d -o root -g root -m 755 /spireroot/etc/ssl/certs
RUN install -d -o root -g root -m 755 /spireroot/run
RUN install -d -o root -g root -m 755 /spireroot/var/lib
RUN install -d -o root -g root -m 1777 /spireroot/tmp

# Set up directories used by SPIRE
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireroot/etc/spire
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireroot/run/spire
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireroot/var/lib/spire

# Set up spire-server directories
RUN cp -r /spireroot /spireserverroot
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireserverroot/etc/spire/server
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireserverroot/run/spire/server/private
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireserverroot/var/lib/spire/server

# Set up spire-agent directories
RUN cp -r /spireroot /spireagentroot
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireagentroot/etc/spire/agent
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireagentroot/run/spire/agent/public
RUN install -d -o ${spireuid} -g ${spiregid} -m 755 /spireagentroot/var/lib/spire/agent

# DYNAMIC build: the OpenSSL backend must dlopen libcrypto, so never static.
# Pin .go-version to the image's Go so the Makefile's go-check doesn't
# download upstream Go (which has no OpenSSL/FIPS backend).
ENV CGO_ENABLED=1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go env GOVERSION | sed 's/^go//' > .go-version && \
    make build && \
    ls -l bin && \
    file bin/spire-server bin/spire-agent bin/oidc-discovery-provider bin/k8s-sat bin/keymanager-k8s

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
USER root
ENTRYPOINT ["/opt/spire/bin/spire-server", "run"]
COPY --link --from=builder /spireserverroot /
COPY --link --from=builder /spire/bin/spire-server bin/

# SPIRE Agent
FROM spire-base AS spire-agent
USER root
ENTRYPOINT ["/opt/spire/bin/spire-agent", "run"]
COPY --link --from=builder /spireagentroot /
COPY --link --from=builder /spire/bin/spire-agent bin/
COPY --link --from=builder /spire/bin/k8s-sat /config/plugin/
COPY --link --from=builder /spire/bin/keymanager-k8s /config/plugin/
COPY --link --from=builder /etc/ssl/certs/ca-certificates.crt /config/ca-certificates.crt

# OIDC Discovery Provider
FROM spire-base AS oidc-discovery-provider
USER root
ENTRYPOINT ["/opt/spire/bin/oidc-discovery-provider"]
COPY --link --from=builder /spire/bin/oidc-discovery-provider bin/