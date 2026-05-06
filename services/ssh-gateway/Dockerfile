FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/ssh-gateway ./services/ssh-gateway/cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/ssh-gateway /app/ssh-gateway
ENV SSH_ADDR=0.0.0.0:2222
EXPOSE 2222
ENTRYPOINT ["/app/ssh-gateway"]
