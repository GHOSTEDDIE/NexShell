package remote

import (
	"context"
	"errors"
	"github.com/armon/go-socks5"
	"io"
	"net"
	"sync"
)

type Tunnel struct {
	Address  string
	cancel   context.CancelFunc
	listener net.Listener
	once     sync.Once
}

func (t *Tunnel) Close() { t.once.Do(func() { t.cancel(); t.listener.Close() }) }
func (m *Manager) Tunnel(ctx context.Context, host, kind, bind, destination string) (*Tunnel, error) {
	c, err := m.Connect(ctx, host)
	if err != nil {
		return nil, err
	}
	if _, _, err = net.SplitHostPort(bind); err != nil {
		return nil, err
	}
	var l net.Listener
	if kind == "remote" {
		l, err = c.Listen("tcp", bind)
	} else if kind == "local" || kind == "dynamic" {
		l, err = net.Listen("tcp", bind)
	} else {
		return nil, errors.New("未知转发类型")
	}
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &Tunnel{Address: l.Addr().String(), cancel: cancel, listener: l}
	go func() { <-ctx.Done(); t.Close() }()
	if kind == "dynamic" {
		server, e := socks5.New(&socks5.Config{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.DialContext(ctx, network, addr)
		}})
		if e != nil {
			t.Close()
			return nil, e
		}
		go func() { _ = server.Serve(l); t.Close() }()
		return t, nil
	}
	if _, _, err = net.SplitHostPort(destination); err != nil {
		t.Close()
		return nil, err
	}
	go func() {
		defer t.Close()
		for {
			conn, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				var out net.Conn
				var e error
				if kind == "remote" {
					out, e = (&net.Dialer{}).DialContext(ctx, "tcp", destination)
				} else {
					out, e = c.DialContext(ctx, "tcp", destination)
				}
				if e != nil {
					return
				}
				defer out.Close()
				stop := context.AfterFunc(ctx, func() { out.Close(); conn.Close() })
				defer stop()
				done := make(chan struct{})
				go func() {
					io.Copy(out, conn)
					if half, ok := out.(interface{ CloseWrite() error }); ok {
						half.CloseWrite()
					}
					close(done)
				}()
				io.Copy(conn, out)
				conn.Close()
				out.Close()
				<-done
			}()
		}
	}()
	return t, nil
}
