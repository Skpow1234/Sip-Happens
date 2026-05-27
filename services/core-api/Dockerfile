# Base digests: infra/docker/base-images.env — see docs/ops/docker-image-security.md
FROM golang:1.25@sha256:f7c6f0bf2d22b0b840d2b4956466b155532de36351056eaba6ea5e1aa458dc3b AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-api ./services/core-api/cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-jobs ./services/core-api/cmd/jobs
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/core-migrate ./services/core-api/cmd/migrate

FROM gcr.io/distroless/static-debian12@sha256:340ba156c899ddac5ba57c5188b8e7cd56448eb7ee65b280574465eac2718ad2
WORKDIR /app
COPY --from=build /out/core-api /app/core-api
COPY --from=build /out/core-jobs /app/core-jobs
COPY --from=build /out/core-migrate /app/core-migrate
COPY infra/migrations /app/infra/migrations
COPY infra/demo /app/infra/demo
COPY docs/openapi.yaml /app/docs/openapi.yaml
EXPOSE 8080
ENTRYPOINT ["/app/core-api"]
