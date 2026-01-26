.PHONY: all build generate test clean install-deps vmlinux check help

# Variables
GO := go
NVCC := nvcc
CLANG := clang
BINARY := cuda_tracer
TEST_CUDA := test_cuda
ARCH := $(shell uname -m)

# eBPF related
BPF_SOURCE := cuda_trace.c
VMLINUX_H := vmlinux.h
EBPF_PROG_FOLDER := internal/ebpf
VMLINUX_BTF := /sys/kernel/btf/vmlinux

all: vmlinux generate build

# Generate vmlinux.h from BTF
vmlinux:
	@echo "Generating vmlinux.h from BTF..."
	@if [ ! -f $(VMLINUX_BTF) ]; then \
		echo "Error: BTF not found at $(VMLINUX_BTF)"; \
		echo "Your kernel may not have BTF support enabled"; \
		exit 1; \
	fi
	@if ! command -v bpftool > /dev/null; then \
		echo "Error: bpftool not found. Please install it."; \
		exit 1; \
	fi
	bpftool btf dump file $(VMLINUX_BTF) format c > $(EBPF_PROG_FOLDER)/$(VMLINUX_H)
	@echo "✓ Generated $(VMLINUX_H)"

# Generate Go bindings from eBPF code
generate: vmlinux
	@echo "Generating eBPF Go bindings..."
	$(GO) generate ./...
	@echo "✓ Generated eBPF bindings"

# Build the tracer
build: generate
	@echo "Building CUDA tracer..."
	$(GO) build -o $(BINARY) .
	@echo "✓ Build complete: $(BINARY)"


# Build test CUDA program
test_cuda: test_cuda.cu
	@echo "Building test CUDA program..."
	@if ! command -v $(NVCC) > /dev/null; then \
		echo "Error: nvcc not found. Please install CUDA toolkit."; \
		exit 1; \
	fi
	$(NVCC) -o $(TEST_CUDA) test_cuda.cu
	@echo "✓ Test program built: $(TEST_CUDA)"

# Run the tracer (requires root)
run: build
	@echo "Running CUDA tracer (requires root)..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: This program must be run as root"; \
		echo "Try: sudo make run"; \
		exit 1; \
	fi
	./$(BINARY)

# Run tracer with test program
test: build test_cuda
	@echo "Starting tracer with test program..."
	@if [ "$$(id -u)" != "0" ]; then \
		echo "Error: This program must be run as root"; \
		echo "Try: sudo make test"; \
		exit 1; \
	fi
	@echo "Starting tracer in background..."
	@./$(BINARY) & \
	TRACER_PID=$$!; \
	sleep 2; \
	echo "Running test CUDA program..."; \
	./$(TEST_CUDA); \
	sleep 2; \
	echo "Stopping tracer..."; \
	kill -INT $$TRACER_PID; \
	wait $$TRACER_PID 2>/dev/null || true

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY) $(TEST_CUDA)
	rm -f bpf_bpfe*.go bpf_bpfe*.o
	rm -f $(VMLINUX_H)
	$(GO) clean
	@echo "✓ Clean complete"

# Install system dependencies (Ubuntu/Debian)
install-deps-ubuntu:
	@echo "Installing system dependencies for Ubuntu/Debian..."
	sudo apt-get update
	sudo apt-get install -y \
		clang \
		llvm \
		libbpf-dev \
		linux-headers-$$(uname -r) \
		linux-tools-common \
		linux-tools-generic \
		linux-tools-$$(uname -r) \
		build-essential \
		golang-go \
		pkg-config
	@echo "✓ System dependencies installed"

# Install system dependencies (Fedora/RHEL)
install-deps-fedora:
	@echo "Installing system dependencies for Fedora/RHEL..."
	sudo dnf install -y \
		clang \
		llvm \
		libbpf-devel \
		kernel-devel \
		bpftool \
		golang \
		make
	@echo "✓ System dependencies installed"

# Install system dependencies (Arch Linux)
install-deps-arch:
	@echo "Installing system dependencies for Arch Linux..."
	sudo pacman -S --needed \
		clang \
		llvm \
		libbpf \
		linux-headers \
		bpf \
		go \
		make
	@echo "✓ System dependencies installed"

# Check system requirements
check:
	@echo "Checking system requirements..."
	@echo ""
	@echo "=== Build Tools ==="
	@echo -n "Go version: "
	@$(GO) version 2>/dev/null || echo "❌ Not found"
	@echo -n "Clang version: "
	@$(CLANG) --version 2>/dev/null | head -n1 || echo "❌ Not found"
	@echo -n "NVCC version: "
	@$(NVCC) --version 2>/dev/null | grep "release" || echo "❌ Not found (optional for testing)"
	@echo ""
	@echo "=== eBPF Support ==="
	@echo -n "bpftool: "
	@command -v bpftool > /dev/null 2>&1 && echo "✓ Found" || echo "❌ Not found"
	@echo -n "BTF support: "
	@[ -f $(VMLINUX_BTF) ] && echo "✓ Available at $(VMLINUX_BTF)" || echo "❌ Not found"
	@echo -n "Kernel headers: "
	@[ -d /lib/modules/$$(uname -r)/build ] && echo "✓ Found" || echo "❌ Not found"
	@echo ""
	@echo "=== Runtime ==="
	@echo -n "Running as root: "
	@[ "$$(id -u)" = "0" ] && echo "✓ Yes" || echo "❌ No (required for eBPF)"
	@echo -n "Kernel version: "
	@uname -r
	@echo ""
	@echo "=== eBPF Features ==="
	@if [ -f /proc/sys/kernel/unprivileged_bpf_disabled ]; then \
		echo -n "Unprivileged BPF: "; \
		[ "$$(cat /proc/sys/kernel/unprivileged_bpf_disabled)" = "0" ] && echo "Enabled" || echo "Disabled (root required)"; \
	fi
	@echo ""

# Verify eBPF program
verify: generate
	@echo "Verifying eBPF program..."
	@if [ -f bpf_bpfel.o ]; then \
		llvm-objdump -S bpf_bpfel.o; \
		echo ""; \
		echo "✓ eBPF object file generated successfully"; \
	else \
		echo "❌ eBPF object file not found"; \
		exit 1; \
	fi

help:
	@echo "CUDA Memory Tracer (BTF/CO-RE) - Makefile targets:"
	@echo ""
	@echo "  make all                  - Generate vmlinux.h, build everything"
	@echo "  make vmlinux              - Generate vmlinux.h from BTF"
	@echo "  make generate             - Generate eBPF Go bindings"
	@echo "  make deps                 - Install Go dependencies"
	@echo "  make build                - Build the tracer"
	@echo "  make test_cuda            - Build test CUDA program"
	@echo "  make run                  - Run the tracer (requires root)"
	@echo "  make test                 - Run tracer with test program"
	@echo "  make clean                - Clean build artifacts"
	@echo "  make verify               - Verify eBPF program compilation"
	@echo "  make check                - Check system requirements"
	@echo ""
	@echo "  make install-deps-ubuntu  - Install system deps (Ubuntu/Debian)"
	@echo "  make install-deps-fedora  - Install system deps (Fedora/RHEL)"
	@echo "  make install-deps-arch    - Install system deps (Arch Linux)"
	@echo ""
	@echo "  make help                 - Show this help message"
	@echo ""
	@echo "Requirements:"
	@echo "  - Linux kernel 5.8+ with BTF support"
	@echo "  - clang/LLVM 10+"
	@echo "  - libbpf-dev"
	@echo "  - bpftool"
	@echo "  - Go 1.18+"
	@echo "  - CUDA toolkit (for test program)"
	@echo "  - Root privileges to run"
	@echo ""
	@echo "Quick Start:"
	@echo "  1. Install dependencies: make install-deps-ubuntu"
	@echo "  2. Install Go deps: make deps"
	@echo "  3. Build: make all"
	@echo "  4. Run: sudo make run"