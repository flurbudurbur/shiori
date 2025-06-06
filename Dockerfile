# build web
FROM node:23-alpine3.21 AS web-builder
WORKDIR /web

# Copy package manifests
COPY web/package.json web/package.json ./

# Copy web source code (respecting .dockerignore)
COPY web/ .

# Install dependencies inside the container
RUN npm install --frozen-lockfile

# Build the web application
RUN ng build

# build app
FROM golang:1.24.2-alpine3.21 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME=unknown

RUN apk add --no-cache git make build-base tzdata

ENV SERVICE=shiori

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

COPY --from=web-builder /web/dist ./web/dist
COPY --from=web-builder /web/build.go ./web

RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/shiori main.go

# build final image
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/flurbudurbur/Shiori"

ENV HOME="/config" \
XDG_CONFIG_HOME="/config" \
XDG_DATA_HOME="/config"

RUN apk add --no-cache ca-certificates curl tzdata jq

WORKDIR /app

VOLUME /config

COPY --from=app-builder /src/bin/shiori /usr/local/bin/

EXPOSE 8282

ENTRYPOINT ["/usr/local/bin/shiori", "--config", "/config"]