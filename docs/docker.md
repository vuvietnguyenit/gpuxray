# Docker environment

`gpuxray` could be run as container that use to trace processes are running in the host. Simply pull the image and start the container.

## Run as exporter

```bash
docker run --rm --gpus all --pid=host -p 2112:2112 ghcr.io/vuvietnguyenit/gpuxray:latest mon
```
The `--gpus all` and `--pid=host` is important because gpuxray will collect metrics of processes are running in host.

## Run as tracing tool

For convenience, we provide Docker image, just pull it and run immediately on GPU servers

```sh
docker run --privileged --rm --gpus all --pid=host \
  -v /sys/kernel/debug:/sys/kernel/debug \
  -v /sys/kernel/tracing:/sys/kernel/tracing \
  ghcr.io/vuvietnguyenit/gpuxray:latest memtrace -p 1690251 -i 1
```

The `-v /sys/kernel/debug:/sys/kernel/debug` and `-v /sys/kernel/tracing:/sys/kernel/tracing` is required because it use eBPF to trace CUDA calls.