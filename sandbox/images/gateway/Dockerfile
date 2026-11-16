# Gateway container: runs "chamber serve" for one run. Build from the
# repository root:
#   docker build -t chamber/gateway:0.1 -f sandbox/images/gateway/Dockerfile .
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY chamber/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/chamber ./cmd/chamber

FROM debian:bookworm-slim
COPY --from=build /out/chamber /usr/local/bin/chamber
COPY configs /chamber/configs
COPY personas /chamber/personas
ENTRYPOINT ["chamber"]
