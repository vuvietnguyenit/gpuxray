// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package internal

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
)

func FilterByRegex(input []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, s := range input {
		if re.MatchString(s) {
			result = append(result, s)
		}
	}
	return result, nil
}

func FilterTreeSetRegex(
	set *treeset.Set,
	pattern string,
) []string {

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	result := []string{}
	it := set.Iterator()
	for it.Next() {
		val := it.Value().(string)
		if re.MatchString(val) {
			result = append(result, val)
		}
	}

	return result
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

func FilterValidCUDASharedObjects(in *treeset.Set) *treeset.Set {
	out := treeset.NewWithStringComparator()

	if in == nil || in.Empty() {
		return out
	}

	for _, v := range in.Values() {
		path, ok := v.(string)
		if !ok {
			continue
		}

		if !isCUDALib(path) {
			continue
		}

		if isRejectedPath(path) {
			continue
		}

		if isSystemPath(path) {
			out.Add(path)
		}
	}

	return out
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
