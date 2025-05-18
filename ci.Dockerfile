# build app
FROM golang:1.24-alpine AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME

RUN apk update && apk upgrade && apk add --no-cache git make build-base tzdata

ENV SERVICE=shiori

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/shiori main.go

# build final image
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github/flurbudurbur/Shiori"

ENV HOME="/config" \
XDG_CONFIG_HOME="/config" \
XDG_DATA_HOME="/config"

RUN apk update && apk upgrade && apk add --no-cache ca-certificates curl tzdata jq

WORKDIR /app

VOLUME /config

COPY --from=app-builder /src/bin/shiori /usr/local/bin/

EXPOSE 8282

ENTRYPOINT ["/usr/local/bin/shiori", "--config", "/config"]