# Source-building image for local `make docker`. The release pipeline uses
# Dockerfile.goreleaser with prebuilt binaries instead.
FROM golang:1.27-alpine AS build

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /out/gcp-radar ./cmd/gcp-radar

FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app
COPY --from=build /out/gcp-radar /app/gcp-radar
COPY config.yaml /app/config.yaml

EXPOSE 9090

ENTRYPOINT ["./gcp-radar"]
