# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
#
# The base image is a *bootstrap* — its Go version (1.25 here) just needs
# to be recent enough to honour `GOTOOLCHAIN=auto`. The actual Go used to
# compile binaries is whatever `go.mod`'s `toolchain` line declares; the
# Go runtime downloads it transparently from proxy.golang.org and verifies
# it against the embedded release keys. Bumping `go.mod` therefore does
# NOT require a Dockerfile edit; just `scripts/docker-refresh-digests.sh`
# when you want to advance the security pin on the bootstrap.
FROM golang:1.25@sha256:699337d620559a59b4a2bb298ad59611e535d2ee755a34cf2d2a98f37578dc80 AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-api ./services/core-api/cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-jobs ./services/core-api/cmd/jobs
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-migrate ./services/core-api/cmd/migrate

FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2
WORKDIR /app
COPY --from=build /out/core-api /app/core-api
COPY --from=build /out/core-jobs /app/core-jobs
COPY --from=build /out/core-migrate /app/core-migrate
COPY infra/migrations /app/infra/migrations
COPY infra/demo /app/infra/demo
COPY docs/openapi.yaml /app/docs/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/app/core-api"]
