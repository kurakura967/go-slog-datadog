FROM golang:1.21.0

WORKDIR /home/go-slog-datadog


COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

ENV DD_TRACE_SAMPLE_RATE 1

RUN go install github.com/cosmtrek/air@latest
#ENTRYPOINT ["./main"]
CMD ["air", "-c", ".air.toml"]
