//go:build !darwin || ci

package nativefiles

import "context"

func Choose(ctx context.Context, r Request) ([]string, error) { return chooseZenity(ctx, r) }
