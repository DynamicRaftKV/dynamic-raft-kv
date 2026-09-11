FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/raftkv ./cmd/raftkv

FROM debian:bookworm-slim
COPY --from=build /out/raftkv /usr/local/bin/raftkv
WORKDIR /data
ENTRYPOINT ["/usr/local/bin/raftkv"]
