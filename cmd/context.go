package cmd

import "context"

var (
	rootCtx    context.Context
	rootCancel context.CancelFunc
)

func RootContext() context.Context {
	return rootCtx
}
