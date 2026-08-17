FROM --platform=$BUILDPLATFORM node:20.19.5-bookworm-slim@sha256:9e70124bd00f47dd023e349cd587132ae61892acc0e47ed641416c3e18f401c3 AS web-build

WORKDIR /src/web

COPY web/.npmrc web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25.13-bookworm@sha256:e401dae1bf814e29204a8cb7915682e1780951e609ca0dd8865ee1937f510c48 AS server-build

WORKDIR /src

COPY web/cmd/static-server/go.mod ./
COPY web/cmd/static-server/main.go ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" go build -buildvcs=false -trimpath -ldflags "-s -w" -o /out/kirby-web .

FROM scratch

COPY --from=server-build /out/kirby-web /kirby-web
COPY --from=web-build /src/web/dist/ /srv/

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/kirby-web"]
