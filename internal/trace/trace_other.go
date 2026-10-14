//go:build !linux

package trace

import "context"

// Run is only implemented on Linux.
func Run(ctx context.Context, opts Options) (Result, error) {
	return Result{}, ErrUnsupported
}
