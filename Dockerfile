# build web
FROM node:23-alpine3.21 AS web-builder
WORKDIR /frontend

# Copy package manifests
COPY frontend/package.json frontend/package.json ./

# Copy web source code (respecting .dockerignore)
COPY frontend/ .

# Install dependencies inside the container
RUN npm install --frozen-lockfile

# Build the web application
RUN ng build

# build app
FROM golang:1.24.2-alpine3.21 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME

RUN apk add --no-cache git make build-base tzdata

ENV SERVICE=syncyomi

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

COPY --from=web-builder /frontend/dist ./frontend/dist
COPY --from=web-builder /frontend/build.go ./frontend

RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/syncyomi main.go

# build final image
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github/SyncYomi/SyncYomi"

ENV HOME="/config" \
XDG_CONFIG_HOME="/config" \
XDG_DATA_HOME="/config"

RUN apk add --no-cache ca-certificates curl tzdata jq

WORKDIR /app

VOLUME /config

COPY --from=app-builder /src/bin/syncyomi /usr/local/bin/

EXPOSE 8282

ENTRYPOINT ["/usr/local/bin/syncyomi", "--config", "/config"]