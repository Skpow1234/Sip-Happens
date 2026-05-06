FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-api ./services/core-api/cmd/api

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/core-api /app/core-api
COPY infra/migrations /app/infra/migrations
EXPOSE 8080
ENTRYPOINT ["/app/core-api"]
