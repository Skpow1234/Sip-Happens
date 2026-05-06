FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates openssh-keygen
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
COPY services/ssh-gateway/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]
