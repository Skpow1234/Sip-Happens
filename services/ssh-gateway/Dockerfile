# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
#
# See services/core-api/Dockerfile for the GOTOOLCHAIN=auto rationale —
# bumping go.mod does NOT require editing this file.
FROM golang:1.25@sha256:f7c6f0bf2d22b0b840d2b4956466b155532de36351056eaba6ea5e1aa458dc3b AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM gcr.io/distroless/static-debian12@sha256:a9fcaedd4c9b59e12dd65d954f0b5044f19b0647a8a3712e77205df9e7b102cd
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/app/ssh-gateway"]
