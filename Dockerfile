# ghorg's container image, from one Dockerfile:
#   target `release` -> what GoReleaser publishes; copies its binary
#   target `ghorg`   -> the same image, compiled from source (CI's build check)
#
# Base images are pinned by digest and pulled from AWS's public mirror of
# Docker Hub (no anonymous rate limits). The build stage cross-compiles rather
# than emulating the target; the runtime stage installs packages, so a
# multi-platform build of it needs QEMU.
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.26-alpine@sha256:c95332c2af86b6d89b91bd0500f4b9529ccbd090a0d1855c6d1ceaa142ae8615 AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
ARG VERSION=dev
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} && \
    go build -trimpath -ldflags "-s -w -X github.com/blairham/ghorg/internal/cmd.version=${VERSION}" -o /out/ghorg .

FROM public.ecr.aws/docker/library/alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0 AS runtime

ARG USER=ghorg
ARG GROUP=ghorg
ARG UID=1111
ARG GID=2222

LABEL org.opencontainers.image.source="https://github.com/blairham/ghorg" \
      org.opencontainers.image.licenses="Apache-2.0"

ENV XDG_CONFIG_HOME=/config
ENV GHORG_CONFIG=/config/conf.yaml
ENV GHORG_RECLONE_PATH=/config/reclone.yaml
ENV GHORG_ABSOLUTE_PATH_TO_CLONE_TO=/data

RUN apk add -U --no-cache ca-certificates openssh-client tzdata git curl tini \
    && mkdir -p /data $XDG_CONFIG_HOME \
    && addgroup --gid $GID $GROUP \
    && adduser -D -H --gecos "" \
                     --home "/home" \
                     --ingroup "$GROUP" \
                     --uid "$UID" \
                     "$USER" \
    && chown -R $USER:$GROUP /home /data $XDG_CONFIG_HOME \
    && rm -rf /tmp/* /var/{cache,log}/* /var/lib/apt/lists/*

USER $USER
WORKDIR /data

# Sample config
COPY --chown=$USER:$GROUP sample-conf.yaml /config/conf.yaml
COPY --chown=$USER:$GROUP sample-reclone.yaml /config/reclone.yaml

VOLUME /data

ENTRYPOINT ["/sbin/tini", "--", "ghorg"]
CMD ["--help"]

# GoReleaser's dockers_v2 puts each platform's binary at $TARGETPLATFORM/ghorg.
FROM runtime AS release
ARG TARGETPLATFORM
COPY --chown=ghorg:ghorg $TARGETPLATFORM/ghorg /usr/local/bin/ghorg

FROM runtime AS ghorg
COPY --from=build --chown=ghorg:ghorg /out/ghorg /usr/local/bin/ghorg
