// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package internal

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func FilterSliceRegex(
	in []string,
	pattern string,
) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	out := make([]string, 0, len(in))
	for _, v := range in {
		if re.MatchString(v) {
			out = append(out, v)
		}
	}

	return out
}

func Deduplicate[T comparable](in []T) []T {
	seen := make(map[T]struct{}, len(in))
	out := make([]T, 0, len(in))

	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

var (
	validCUDALibs = []string{
		"libcuda.so",
		"libcudart.so",
	}

	systemPrefixes = []string{
		"/usr/lib/",
		"/usr/lib64/",
		"/lib/",
		"/lib64/",
		"/usr/local/cuda/",
	}

	rejectHints = []string{
		"/site-packages/",
		"/.venv/",
		"/venv/",
		"/home/",
	}
)

func FilterValidCUDASharedObjects(in []string) []string {
	var out []string

	if len(in) == 0 {
		return out
	}

	for _, path := range in {
		if !isCUDALib(path) {
			continue
		}

		if isRejectedPath(path) {
			continue
		}

		if isSystemPath(path) {
			out = append(out, path)
		}
	}
	return Deduplicate(out)
}

func isCUDALib(path string) bool {
	base := filepath.Base(path)
	for _, lib := range validCUDALibs {
		if strings.HasPrefix(base, lib) {
			return true
		}
	}
	return false
}

func isRejectedPath(path string) bool {
	for _, hint := range rejectHints {
		if strings.Contains(path, hint) {
			return true
		}
	}
	return false
}

func isSystemPath(path string) bool {
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func HumanBytes(b uint64) string {
	const unit = 1024

	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	value := float64(b) / float64(div)
	suffix := "KMGTPE"[exp]

	return fmt.Sprintf("%.2f %ciB", value, suffix)
}
