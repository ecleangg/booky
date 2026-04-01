FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY config ./config
COPY internal ./internal
COPY migrations ./migrations

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/booky ./cmd/booky

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ENV BOOKY_CONFIG=/app/config/booky.yaml

COPY --from=builder /out/booky /app/booky
COPY --chown=nonroot:nonroot config ./config
COPY --chown=nonroot:nonroot migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/app/booky"]
