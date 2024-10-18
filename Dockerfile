FROM alpine:latest

COPY out/kmesh-dns /app/kmesh-dns

ENTRYPOINT ["/app/kmesh-dns"]