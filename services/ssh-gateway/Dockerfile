# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
FROM golang:1.24@sha256:46fdd02b6cbcd624a4087ea298e4c8505e5d400c4ee5181e4dd06e2297d647ae AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM gcr.io/distroless/static-debian12@sha256:9c346e4be81b5ca7ff31a0d89eaeade58b0f95cfd3baed1f36083ddb47ca3160
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/app/ssh-gateway"]
