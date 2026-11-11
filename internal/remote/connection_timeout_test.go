package remote

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
)

type timeoutHost struct{ host domain.Host }

func (h timeoutHost) Host(string) (domain.Host, error) { return h.host, nil }

type timeoutSecret struct{}

func (timeoutSecret) Get(string) (string, error) { return "unused", nil }

func TestConnectDeadlineClosesHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	address, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	host := domain.Host{ID: "timeout", Name: "timeout", Address: address, Port: port, User: "test", Auth: "password"}
	manager, err := NewManager(timeoutHost{host}, timeoutSecret{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = manager.Connect(ctx, host.ID)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("connection error = %v, want deadline exceeded", err)
	}
	select {
	case conn := <-accepted:
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err = io.Copy(io.Discard, conn); err != nil {
			t.Fatalf("timed-out socket was not closed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection was not accepted")
	}
}
