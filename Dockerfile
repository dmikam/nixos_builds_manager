FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/nixos_builds_manager main.go

FROM scratch
COPY --from=builder /app/bin/nixos_builds_manager /nixos_builds_manager
ENTRYPOINT ["/nixos_builds_manager"]