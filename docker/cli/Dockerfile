# syntax=docker/dockerfile:1
FROM --platform=$TARGETPLATFORM alpine:3
ARG TARGETARCH
RUN apk add --no-cache ca-certificates git
COPY event-spec-${TARGETARCH} /usr/local/bin/event-spec
ENTRYPOINT ["event-spec"]
