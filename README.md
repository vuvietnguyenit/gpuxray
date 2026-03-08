# GPUXRAY

![GitHub release](https://img.shields.io/github/v/release/vuvietnguyenit/gpuxray)
![Go Version](https://img.shields.io/github/go-mod/go-version/vuvietnguyenit/gpuxray)
![License](https://img.shields.io/github/license/vuvietnguyenit/gpuxray)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/assets/gpuxray-dark.svg">
    <img src="./docs/assets/gpuxray-light.svg" width="420">
  </picture>
</p>


An opensource observability tool for debugging GPU workloads on Linux servers.

It traces CUDA activity using eBPF and provides:
- per-process GPU metrics
- GPU memory leak detection
- Prometheus metrics for monitoring systems
This tool is inspired by [pidstat](https://man7.org/linux/man-pages/man1/pidstat.1.html) but designed for GPU monitoring.
GPUXRAY is designed for AI/ML workloads running on GPU servers.

Welcome to anyone who want to contribute to this project.

### Why use gpuxray?
- `gpuxray` provides GPU observability at the process level, which is not fully supported by [DCGM exporter](https://github.com/NVIDIA/dcgm-exporter). It also leverages eBPF to perform deeper tracing of GPU workloads.
- It leverages eBPF to trace CUDA activity from the kernel, enabling low-overhead and deep tracing of GPU workloads.

## Usecases
- Works well for tracing and get stats from processes that use GPU resources through the CUDA API (e.g., ML jobs, AI workloads).
- Exposes Prometheus metrics fpr GPU resources associated with each PID.
- Very convinient for detecting GPU memory leaked. It can show stack traces of leaked GPU memory blocks and identify the CUDA functions responsible for allocations that were not freed.

## Notice
- Currently, this tool only inspects PIDs that use the CUDA Driver API. Processes that use the CUDA Runtime API may be omitted.
- Requires Linux kernel version >= 5.6. Kernel versions in the 6.x series are recommended.
- Currently supports only the `amd64` CPU architecture.

## Architecture

GPUXRAY collects GPU information using multiple techniques:

1. **NVML** – retrieves GPU metrics per process
2. **eBPF** – traces CUDA calls
3. **Go exporter** – exposes metrics for Prometheus


## Install

### Binary
Install gpuxray easily with one command:
```sh
curl -s https://raw.githubusercontent.com/vuvietnguyenit/gpuxray/main/install.sh | sh
```
### Docker

`gpuxray` could be run as container that use to trace processes are running in the host. Simply pull the image and start the container.
#### Run as Prometheus exporter

```sh
docker run --rm --gpus all --pid=host -p 2112:2112 ghcr.io/vuvietnguyenit/gpuxray:latest mon
```

#### Run as tracing tool
```sh
docker run --privileged --rm --gpus all --pid=host \
  -v /sys/kernel/debug:/sys/kernel/debug \
  -v /sys/kernel/tracing:/sys/kernel/tracing \
  ghcr.io/vuvietnguyenit/gpuxray:latest memtrace -p 1690251 -i 1
```

### Build from source

```sh
git clone https://github.com/vuvietnguyenit/gpuxray
cd gpuxray
go build -o gpuxray
```

## Quickstart
### Run GPU exporter

Running the exporter exposes metrics related to processes using GPU resources on the server.
```sh
# gpuxray mon
```
Metric definitions are available in: [metrics.txt](./metrics.txt)
<details>
<summary>Example result</summary>

```text
curl http://localhost:2112/metrics
...
# HELP gpu_free_memory_bytes Remaining available GPU memory for the process in bytes.
# TYPE gpu_free_memory_bytes gauge                                                                                  
gpu_free_memory_bytes{gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn"} 9.903734784e+09
# HELP gpu_process_active 1 for each process currently using a GPU.             
# TYPE gpu_process_active gauge
gpu_process_active{args="/usr/bin/gnome-shell",comm="gnome-shell",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="3112"} 1
gpu_process_active{args="/usr/lib/xorg/Xorg vt1 -displayfd 3 -auth /run/user/120/gdm/Xauthority -nolisten tcp -background none -noreset -keeptty -novtswitch -verbose 3",comm="Xorg",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_
index="0",hostname="gpu1.itim.vn",pid="2912"} 1
gpu_process_active{args="python -m src.models.classifier --train-file /shared_storage/ailab/intent-classifier/train/raw-click.gz --valid-file /shared_storage/ailab/intent-classifier/valid/raw-click.gz --model-name vinai/phobert-base
-v2",comm="python",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="401948"} 1
# HELP gpu_process_sm_utilization_percent GPU SM utilisation of the process (0–100). Requires NVML r470+ drivers; returns 0 on older drivers.
# TYPE gpu_process_sm_utilization_percent gauge
gpu_process_sm_utilization_percent{args="/usr/bin/gnome-shell",comm="gnome-shell",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="3112"} 0
gpu_process_sm_utilization_percent{args="/usr/lib/xorg/Xorg vt1 -displayfd 3 -auth /run/user/120/gdm/Xauthority -nolisten tcp -background none -noreset -keeptty -novtswitch -verbose 3",comm="Xorg",gpu="GPU-47def375-4603-e5fa-82d3-c7
cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="2912"} 0
gpu_process_sm_utilization_percent{args="python -m src.models.classifier --train-file /shared_storage/ailab/intent-classifier/train/raw-click.gz --valid-file /shared_storage/ailab/intent-classifier/valid/raw-click.gz --model-name vi
nai/phobert-base-v2",comm="python",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="401948"} 86
# HELP gpu_process_used_memory_bytes GPU memory consumed by the process in bytes.
# TYPE gpu_process_used_memory_bytes gauge
gpu_process_used_memory_bytes{args="/usr/bin/gnome-shell",comm="gnome-shell",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="3112"} 1.1296768e+07
gpu_process_used_memory_bytes{args="/usr/lib/xorg/Xorg vt1 -displayfd 3 -auth /run/user/120/gdm/Xauthority -nolisten tcp -background none -noreset -keeptty -novtswitch -verbose 3",comm="Xorg",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc8
1e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="2912"} 1.0575872e+07
gpu_process_used_memory_bytes{args="python -m src.models.classifier --train-file /shared_storage/ailab/intent-classifier/train/raw-click.gz --valid-file /shared_storage/ailab/intent-classifier/valid/raw-click.gz --model-name vinai/p
hobert-base-v2",comm="python",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn",pid="401948"} 2.3716691968e+10
# HELP gpu_total_memory_bytes Total GPU memory available in bytes.
# TYPE gpu_total_memory_bytes gauge
gpu_total_memory_bytes{gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn"} 3.4190917632e+10
# HELP gpu_used_memory_bytes GPU memory currently allocated in bytes.
# TYPE gpu_used_memory_bytes gauge
gpu_used_memory_bytes{gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",hostname="gpu1.itim.vn"} 2.4287182848e+10
...
```
</details>

### Memory statistics

This command reports statistics about GPU memory usage.
```sh
# ./gpuxray memtrace -p 2806854
TIME       PID      USER     GPU  INUSE_MB     AL_CNT   FR_CNT   COMM            
12:03:24   2806854  root     0    512 B        199      198      python3         
12:03:29   2806854  root     0    512 B        402      401      python3         
12:03:34   2806854  root     0    512 B        607      606      python3         
12:03:39   2806854  root     0    1.00 KiB     802      800      python3         
12:03:44   2806854  root     0    1.00 KiB     994      992      python3         
12:03:49   2806854  root     0    2.00 KiB     1197     1193     python3    
```
To see the meaning of each column, run: `./gpuxray memtrace -h` flag to see more information

### Show memory-leaked stacktraces

This command prints stack traces responsible for leaked GPU memory allocations.

```sh
# ./gpuxray memtrace -p 332361 -i 1 --print-stacks
2026-03-04T15:59:44+07:00
[1] PID: 332361   GPU: 0   StackID: 1908    Remaining Blocks: 1       TotalBytes: 512 B     
  #00  0x71263447d86e      libcudart_static_5382377d5c772c9d197c0cda9fd9742ee6ad893c
  #01  0x7126344491c3      libcudart_static_f74e2f2bcf2cf49bd1a61332e1d15bd1e748f9cf
  #02  0x71263448d993      cudaMalloc
  #03  0x712634420cde      __pyx_f_13cupy_backends_4cuda_3api_7runtime_malloc(unsigned long, int)

2026-03-04T15:59:45+07:00
[1] PID: 332361   GPU: 0   StackID: 1908    Remaining Blocks: 1       TotalBytes: 512 B     
  #00  0x71263447d86e      libcudart_static_5382377d5c772c9d197c0cda9fd9742ee6ad893c
  #01  0x7126344491c3      libcudart_static_f74e2f2bcf2cf49bd1a61332e1d15bd1e748f9cf
  #02  0x71263448d993      cudaMalloc
  #03  0x712634420cde      __pyx_f_13cupy_backends_4cuda_3api_7runtime_malloc(unsigned long, int)

2026-03-04T15:59:46+07:00
[1] PID: 332361   GPU: 0   StackID: 1908    Remaining Blocks: 2       TotalBytes: 1.00 KiB  
  #00  0x71263447d86e      libcudart_static_5382377d5c772c9d197c0cda9fd9742ee6ad893c
  #01  0x7126344491c3      libcudart_static_f74e2f2bcf2cf49bd1a61332e1d15bd1e748f9cf
  #02  0x71263448d993      cudaMalloc
  #03  0x712634420cde      __pyx_f_13cupy_backends_4cuda_3api_7runtime_malloc(unsigned long, int)

^C2026/03/04 15:59:46 Received signal, exiting..
```
## Debugging

Enable debug mode to help diagnose issues. `--log-level debug` flag.
It could help print the metrics of GPU process to the console. Like this:

```sh
# ./gpuxray mon --log-level debug
11:32:31 DBG prometheus scrape started method=GET path=/metrics remote=[::1]:43402
11:32:31 DBG metrics for GPU 0: total=34190917632 used=24287182848 free=9903734784
11:32:31 DBG process 401948 (python) on GPU 0: used_memory=23716691968 sm_util=93%
11:32:31 DBG process 2912 (Xorg) on GPU 0: used_memory=10575872 sm_util=0%
11:32:31 DBG process 3112 (gnome-shell) on GPU 0: used_memory=11296768 sm_util=0%
11:32:31 DBG prometheus scrape finished duration=2.600426
11:32:32 DBG prometheus scrape started method=GET path=/metrics remote=[::1]:43418
11:32:32 DBG metrics for GPU 0: total=34190917632 used=24287182848 free=9903734784
11:32:32 DBG process 401948 (python) on GPU 0: used_memory=23716691968 sm_util=93%
11:32:32 DBG process 2912 (Xorg) on GPU 0: used_memory=10575872 sm_util=0%
11:32:32 DBG process 3112 (gnome-shell) on GPU 0: used_memory=11296768 sm_util=0%
11:32:32 DBG prometheus scrape finished duration=3.039056
```

This mode helps observe:
- GPU metrics collection
- active GPU processes
- Prometheus scrape activity (client IP, duration, etc.)
- internal tracing events

It can also display detailed actions when running the `memtrace` feature.