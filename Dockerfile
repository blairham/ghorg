# ==================================================
# Runtime image for GoReleaser
# ==================================================
FROM alpine:latest

ARG TARGETPLATFORM
ARG USER=ghorg
ARG GROUP=ghorg
ARG UID=1111
ARG GID=2222

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

# Copy compiled binary from GoReleaser (dockers_v2 uses $TARGETPLATFORM)
COPY --chown=$USER:$GROUP $TARGETPLATFORM/ghorg /usr/local/bin/ghorg

VOLUME /data

ENTRYPOINT ["/sbin/tini", "--", "ghorg"]
CMD ["--help"]
