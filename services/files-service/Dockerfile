# build context: root repozitorijuma (docker compose: context: ., dockerfile: services/files-service/Dockerfile)

# ---- build faza ----
FROM golang:1.27-alpine AS build
WORKDIR /src

# kopiramo ceo monorepo (go.work povezuje common i sve servise)
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./services/files-service

# ---- runtime faza ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

# direktorijum za binarni sadrzaj datoteka (montira se kao volume)
RUN mkdir -p /data/uploads && chown -R app:app /data

WORKDIR /app
COPY --from=build /out/app /app/app

USER app
ENTRYPOINT ["/app/app"]
