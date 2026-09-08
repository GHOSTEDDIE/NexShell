package remote

import (
	"context"
	"errors"
	"github.com/pkg/sftp"
	"path"
)

type authorityKey struct{}
type authority struct {
	HostID, Digest string
	Check          func() error
}

func WithAuthority(ctx context.Context, host, digest string, check func() error) context.Context {
	return context.WithValue(ctx, authorityKey{}, authority{host, digest, check})
}
func checkAuthority(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a, ok := ctx.Value(authorityKey{}).(authority); ok && a.Check != nil {
		return a.Check()
	}
	return ctx.Err()
}
func canonicalFile(ctx context.Context, c *sftp.Client, p string) error {
	if _, restricted := ctx.Value(authorityKey{}).(authority); !restricted {
		return nil
	}
	real, err := c.RealPath(p)
	if err != nil {
		parent, e := c.RealPath(path.Dir(p))
		if e != nil {
			return err
		}
		real = path.Join(parent, path.Base(p))
	}
	if real != p {
		return errors.New("授权文件经过符号链接，请使用实际绝对路径重新授权")
	}
	return nil
}
