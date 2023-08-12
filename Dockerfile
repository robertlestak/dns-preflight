FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -o /bin/preflight-dns cmd/preflight-dns/*.go

FROM alpine:3.14 as app

RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/preflight-dns /bin/preflight-dns

ENTRYPOINT ["/bin/preflight-dns"]