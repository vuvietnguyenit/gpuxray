.PHONY: all build generate test clean install-deps vmlinux check help

# Variables
GO := go
BINARY := gpuxray
ARCH := $(shell uname -m)

# eBPF related
VMLINUX_H := vmlinux.h
EBPF_PROG_FOLDER := ./internal/bpf/headers
APP_FOLDER := ./internal/app
VMLINUX_BTF := /sys/kernel/btf/vmlinux

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

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY)
	rm -f $(EBPF_PROG_FOLDER)/$(VMLINUX_H)
	$(GO) clean
	
	@echo "Cleaning generated files in gen/ folders..."
	find . -type d -name "gen" -exec find {} -type f ! -name ".gitkeep" -delete \;

	@echo "✓ Clean complete"
