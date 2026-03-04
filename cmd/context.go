// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package cmd

import "context"

var (
	rootCtx    context.Context
	rootCancel context.CancelFunc
)

func RootContext() context.Context {
	return rootCtx
}
