# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
FROM golang:1.25@sha256:f7c6f0bf2d22b0b840d2b4956466b155532de36351056eaba6ea5e1aa458dc3b AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM gcr.io/distroless/static-debian12@sha256:340ba156c899ddac5ba57c5188b8e7cd56448eb7ee65b280574465eac2718ad2
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/app/ssh-gateway"]
