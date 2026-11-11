package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/remote"
)

const activeConnectionTimeout = 10 * time.Second

type terminalConnectResult struct {
	session *remote.TerminalSession
	err     error
}

func connectTerminalWithin(parent context.Context, timeout time.Duration, open func(context.Context) (*remote.TerminalSession, error), disconnect func()) (*remote.TerminalSession, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	result := make(chan terminalConnectResult)
	abandoned := make(chan struct{})
	go func() {
		session, err := open(ctx)
		select {
		case result <- terminalConnectResult{session, err}:
		case <-abandoned:
			if session != nil {
				session.Close()
			}
		}
	}()
	select {
	case current := <-result:
		close(abandoned)
		if err := ctx.Err(); err != nil {
			if current.session != nil {
				current.session.Close()
			}
			disconnect()
			return nil, activeConnectionError(err)
		}
		if isTimeoutError(current.err) {
			if current.session != nil {
				current.session.Close()
			}
			disconnect()
			return nil, activeConnectionError(current.err)
		}
		return current.session, current.err
	case <-ctx.Done():
		close(abandoned)
		disconnect()
		return nil, activeConnectionError(ctx.Err())
	}
}

func activeConnectionError(err error) error {
	if isTimeoutError(err) {
		return fmt.Errorf("连接超时（timeout），已断开连接: %w", err)
	}
	return err
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var network net.Error
	return errors.As(err, &network) && network.Timeout()
}
