//go:build !windows

package remote

import (
	"context"
	"errors"
	"net"
	"os"
)

func dialAgent(ctx context.Context) (net.Conn, error) {
	p := os.Getenv("SSH_AUTH_SOCK")
	if p == "" {
		return nil, errors.New("SSH_AUTH_SOCK 未设置")
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", p)
}
