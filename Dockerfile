FROM debian:bookworm-slim
WORKDIR /app
COPY gpuxray /usr/local/bin/gpuxray
ENTRYPOINT ["gpuxray"]