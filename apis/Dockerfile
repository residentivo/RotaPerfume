# syntax=docker/dockerfile:1
# API rotaperfumes-api - build multi-stage.
#
# Contexto de build = apis/ (NAO apis/rotaperfumes-api): o go.mod da API usa
#   replace github.com/rotaperfumes/shared => ../shared
# entao o modulo shared precisa estar dentro do contexto.
#
#   docker build -t rotaperfumes-api apis/
#
# Runtime: distroless static (sem shell, sem package manager), usuario nonroot
# (uid 65532). Ja traz ca-certificates (SMTP/TLS e Turnstile via HTTPS) e
# /usr/share/zoneinfo. O fuso do processo e fixado em -03:00 pelo pacote
# shared/tz (time.FixedZone), que nao depende de base tz.

ARG GO_VERSION=1.26

# ---------- build ----------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS=-mod=readonly

# Cache das dependencias: so os go.mod/go.sum primeiro.
COPY shared/go.mod shared/go.sum ./shared/
COPY rotaperfumes-api/go.mod rotaperfumes-api/go.sum ./rotaperfumes-api/
RUN cd rotaperfumes-api && go mod download

COPY shared/ ./shared/
COPY rotaperfumes-api/ ./rotaperfumes-api/

RUN cd rotaperfumes-api && \
    go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
