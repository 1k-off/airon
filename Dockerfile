# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.5

FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

ENV CGO_ENABLED=0
RUN go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -X github.com/1k-off/airon/internal/mcp.ClientVersion=${VERSION}" \
    -o /out/airon \
    ./cmd/airon/

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/airon /airon

USER nonroot:nonroot
ENTRYPOINT ["/airon"]
