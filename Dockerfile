FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /out/aide-bot ./cmd/aide-bot

FROM alpine:3.21

WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY sertificate/russiantrustedca.pem /usr/local/share/ca-certificates/russiantrustedca.crt
COPY sertificate/russiantrustedca2024.pem /usr/local/share/ca-certificates/russiantrustedca2024.crt
COPY sertificate/russian_trusted_root_ca_gost_2025.cer /usr/local/share/ca-certificates/russian_trusted_root_ca_gost_2025.crt
COPY sertificate/russian_trusted_sub_ca_gost_2025.cer /usr/local/share/ca-certificates/russian_trusted_sub_ca_gost_2025.crt
RUN update-ca-certificates
COPY --from=builder /out/aide-bot /usr/local/bin/aide-bot

CMD ["aide-bot"]
