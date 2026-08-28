# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w \
    -X github.com/ashutosh0x/infra-control/pkg/version.Version=${VERSION} \
    -X github.com/ashutosh0x/infra-control/pkg/version.Commit=${COMMIT} \
    -X github.com/ashutosh0x/infra-control/pkg/version.BuildDate=${BUILD_DATE}" \
    -o /bin/infra-controller ./cmd/controller

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w" \
    -o /bin/infra-worker ./cmd/worker

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w" \
    -o /bin/infra-mcp-server ./cmd/mcp-server

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w" \
    -o /bin/infractl ./cmd/infractl

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S infra && adduser -S infra -G infra

COPY --from=builder /bin/infra-controller /usr/local/bin/
COPY --from=builder /bin/infra-worker /usr/local/bin/
COPY --from=builder /bin/infra-mcp-server /usr/local/bin/
COPY --from=builder /bin/infractl /usr/local/bin/
COPY configs/default.yaml /etc/infra-control/config.yaml

USER infra

EXPOSE 8080 8443 9090 50051

ENTRYPOINT ["infra-controller"]
CMD ["--config", "/etc/infra-control/config.yaml"]
