# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
#
# See services/core-api/Dockerfile for the GOTOOLCHAIN=auto rationale —
# bumping go.mod does NOT require editing this file.
FROM golang:1.25@sha256:699337d620559a59b4a2bb298ad59611e535d2ee755a34cf2d2a98f37578dc80 AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/app/ssh-gateway"]
