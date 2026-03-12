# Debugging

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