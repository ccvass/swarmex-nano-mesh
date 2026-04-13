FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /swarmex-nano-mesh ./cmd

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget curl \
    && ARCH=$(uname -m | sed 's/x86_64/x86_64/;s/aarch64/aarch64/') \
    && curl -fsSL "https://github.com/EasyTier/EasyTier/releases/latest/download/easytier-linux-${ARCH}-v2.4.5.zip" -o /tmp/et.zip \
    && unzip /tmp/et.zip -d /tmp/et \
    && cp /tmp/et/*/easytier-core /usr/local/bin/ \
    && chmod +x /usr/local/bin/easytier-core \
    && rm -rf /tmp/et /tmp/et.zip
COPY --from=build /swarmex-nano-mesh /usr/local/bin/swarmex-nano-mesh
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["swarmex-nano-mesh"]
