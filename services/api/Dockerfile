# Built by Compose so the host machine does not need a Go toolchain.
FROM golang:1.23-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget \
    && adduser -D -H -u 65532 appuser
COPY --from=build /out/api /api
USER appuser
EXPOSE 8080
ENTRYPOINT ["/api"]
