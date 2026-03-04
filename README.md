# GPUXRAY

The opensource debugging and monitoring for GPU server.
This tool will help troubleshooting problem that is related to GPU resources with each process is running on server.
Some functions are implemented in this tool was used by eBPF.
This tool is inspired by [pidstat](https://man7.org/linux/man-pages/man1/pidstat.1.html) but for GPU.

Welcome to anyone who want to contribute to this project.

## Usecases
- Work well to tracing and get stats of process is using GPU resource (ML jobs, AI jobs) by print it to console.
- Expose the Prometheus metrics of GPU resource that is corresponding with PID. 
- Very convinient to detect GPU memory leaked. It can show stacktraces in memory-leaked blocks in GPU. It will expose the function that isn't freed in memory.

## Notice
- At this time, this tool just inspect PID that is use by CUDA driver API, if PID is using by CUDA runtime API, it can be omit.
- Require kernel version >= 5.6. The Linux kernel in range 6.x is perfect.

## Quickstart
### Run GPU exporter
When you run GPU exporter, it will expose the metric that is related to PID is running in GPU server.
Run command:
```sh
# gpuxray mon
```
Example result:
```text
curl http://localhost:2112/metrics
...
# HELP gpu_process_active 1 for each process currently using a GPU.
# TYPE gpu_process_active gauge
gpu_process_active{args="python3 ./gpu_burn.py",comm="python3",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",pid="300101"} 1
# HELP gpu_process_sm_utilization_percent GPU SM utilisation of the process (0–100). Requires NVML r470+ drivers; returns 0 on older drivers.
# TYPE gpu_process_sm_utilization_percent gauge
gpu_process_sm_utilization_percent{args="python3 ./gpu_burn.py",comm="python3",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",pid="300101"} 37
# HELP gpu_process_used_memory_bytes GPU memory consumed by the process in bytes.
# TYPE gpu_process_used_memory_bytes gauge
gpu_process_used_memory_bytes{args="python3 ./gpu_burn.py",comm="python3",gpu="GPU-47def375-4603-e5fa-82d3-c7cddc81e65a",gpu_index="0",pid="300101"} 7.57071872e+08
...
```
### Run memory stat

This will report stat of GPU's memory usage. For example:
```sh
# ./gpuxray memtrace -p 332361 -i 1
TIME       PID      GPU  INUSE_MB     AL_CNT   FR_CNT   COMM            
15:58:36   332361   0    512 B        38       37       python3         
15:58:37   332361   0    1.00 KiB     80       78       python3         
15:58:38   332361   0    1.00 KiB     118      116      python3         
15:58:39   332361   0    512 B        156      155      python3         
15:58:40   332361   0    1.00 KiB     197      195      python3         
15:58:41   332361   0    1.00 KiB     237      235      python3         
15:58:42   332361   0    512 B        275      274      python3         
15:58:43   332361   0    2.00 KiB     315      311      python3         
15:58:44   332361   0    2.00 KiB     358      354      python3         
15:58:45   332361   0    1.00 KiB     400      398      python3         
15:58:46   332361   0    512 B        439      438      python3         
15:58:47   332361   0    1.00 KiB     482      480      python3         
15:58:48   332361   0    512 B        524      523      python3         
15:58:49   332361   0    1.00 KiB     566      564      python3         
^C2026/03/04 15:58:50 Received signal, exiting..
```

### Show memory-leaked stacktraces

Used to print stack trace that is making memory leaked.

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

