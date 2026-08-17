# syntax=docker/dockerfile:1.7
FROM golang:1.26.6-alpine AS build
ARG SERVICE=gateway
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/service /app/service
COPY web /app/web
USER nonroot:nonroot
EXPOSE 8080 8081 8082 9090
ENTRYPOINT ["/app/service"]
