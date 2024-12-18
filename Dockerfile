FROM golang:1.22-alpine3.18
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go build -trimpath -ldflags="-w -s" -o "mqtt-speedtest"
CMD ["./mqtt-speedtest"]
