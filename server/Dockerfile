FROM golang:1.25.13-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48 AS build

WORKDIR /src/server

COPY server/go.mod server/go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY="${GOPROXY}" go mod download

COPY server/ ./

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -buildvcs=false \
    -trimpath \
    -ldflags "-s -w -X github.com/yvvlee/kirby/server/internal/version.Version=${VERSION} -X github.com/yvvlee/kirby/server/internal/version.Commit=${COMMIT} -X github.com/yvvlee/kirby/server/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/kirby .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/kirby /kirby

USER 65532:65532
EXPOSE 8080 9090
ENTRYPOINT ["/kirby"]
CMD ["serve", "--config", "/etc/kirby/config.yaml"]
