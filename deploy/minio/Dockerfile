FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS build

ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOBIN=/out

# Build official source revisions because the upstream container repositories
# no longer allow anonymous pulls. Go verifies downloads with the checksum DB.
RUN go install -trimpath -ldflags="-s -w" github.com/minio/minio@v0.0.0-20260212201848-7aac2a2c5b7c \
    && cp /go/pkg/mod/github.com/minio/minio@v0.0.0-20260212201848-7aac2a2c5b7c/LICENSE /out/minio.LICENSE
RUN go install -trimpath -ldflags="-s -w" github.com/minio/mc@v0.0.0-20251106162529-77f82e18b540 \
    && cp /go/pkg/mod/github.com/minio/mc@v0.0.0-20251106162529-77f82e18b540/LICENSE /out/mc.LICENSE

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache ca-certificates curl

COPY --from=build /out/minio /out/mc /usr/local/bin/
COPY --from=build /out/minio.LICENSE /out/mc.LICENSE /usr/share/licenses/minio/

LABEL org.opencontainers.image.licenses="AGPL-3.0-only" \
    io.kirby.minio.source="https://github.com/minio/minio/tree/7aac2a2c5b7c882e68c1ce017d8256be2feea27f" \
    io.kirby.mc.source="https://github.com/minio/mc/tree/77f82e18b5401a65958f1619df6ebb994634bd88"

EXPOSE 9000
ENTRYPOINT ["minio"]
CMD ["server", "/data", "--address", ":9000"]
