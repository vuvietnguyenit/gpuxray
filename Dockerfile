# ---------- Build stage ----------
FROM golang:1.25 AS builder

RUN apt-get update && apt-get install -y \
    clang \
    llvm \
    libbpf-dev \
    bpftool \
    make \
    git

WORKDIR /app
COPY . .

RUN go build -o gpuxray .

# ---------- runtime ----------
FROM debian:bookworm-slim
COPY --from=builder /app/gpuxray /usr/local/bin/gpuxray
ENTRYPOINT ["gpuxray"]