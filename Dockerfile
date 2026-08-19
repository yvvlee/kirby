FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS web-build

WORKDIR /src/web

COPY web/.npmrc web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build

WORKDIR /src/server

COPY server/go.mod server/go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY="${GOPROXY}" go mod download

COPY server/ ./

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build \
    -buildvcs=false \
    -trimpath \
    -ldflags "-s -w -X github.com/yvvlee/kirby/server/internal/version.Version=${VERSION} -X github.com/yvvlee/kirby/server/internal/version.Commit=${COMMIT} -X github.com/yvvlee/kirby/server/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/kirby .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/kirby /kirby
COPY --from=web-build /src/web/dist/ /srv/

USER 65532:65532
EXPOSE 8080 9090
ENTRYPOINT ["/kirby"]
CMD ["serve", "--config", "/etc/kirby/config.yaml", "--web-root", "/srv"]
