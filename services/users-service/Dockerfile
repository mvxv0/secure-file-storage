# build context: root repozitorijuma (docker compose: context: ., dockerfile: services/users-service/Dockerfile)

# ---- build faza ----
FROM golang:1.27-alpine AS build
WORKDIR /src

# kopiramo ceo monorepo (go.work povezuje common i sve servise)
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./services/users-service

# ---- runtime faza ----
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=build /out/app /app/app

USER app
ENTRYPOINT ["/app/app"]
