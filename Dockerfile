# Étape 1 : Compilation
FROM golang:latest AS builder
WORKDIR /app
COPY . .
# Compilation statique pour Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o xaladownloader .

# Étape 2 : Image finale légère
FROM alpine:latest
WORKDIR /root/
RUN apk --no-cache add ca-certificates
# On récupère le binaire statique
COPY --from=builder /app/xaladownloader .

EXPOSE 8080
ENV IS_DOCKER=true

CMD ["./xaladownloader"]