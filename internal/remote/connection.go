package remote

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"golang.org/x/crypto/ssh"
	sshagent "golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"
)

type Hosts interface {
	Host(string) (domain.Host, error)
}
type Secrets interface{ Get(string) (string, error) }
type HostKeyError struct {
	Address, Fingerprint string
	Key                  ssh.PublicKey
	Changed              bool
}

func (e *HostKeyError) Error() string {
	if e.Changed {
		return "服务器身份已变化，已阻止连接：" + e.Address + " " + e.Fingerprint
	}
	return "首次连接，请核对服务器指纹：" + e.Address + " " + e.Fingerprint
}

type cached struct {
	client  *ssh.Client
	digest  string
	cleanup func()
}
type Manager struct {
	hosts     Hosts
	secrets   Secrets
	knownFile string
	mu        sync.Mutex
	clients   map[string]cached
	locks     sync.Map
	closed    bool
	Prompt    func(context.Context, string, string, []string, []bool) ([]string, error)
}

func NewManager(hosts Hosts, secrets Secrets, dir string) (*Manager, error) {
	p := filepath.Join(dir, "known_hosts")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	return &Manager{hosts: hosts, secrets: secrets, knownFile: p, clients: map[string]cached{}}, nil
}
func ValidateHost(h domain.Host) error {
	if h.ID == "" || h.Name == "" || h.Address == "" || h.User == "" {
		return errors.New("名称、地址和用户名不能为空")
	}
	if h.Port < 1 || h.Port > 65535 {
		return errors.New("端口必须在 1–65535 之间")
	}
	switch h.Auth {
	case "password", "key", "agent", "interactive":
	default:
		return errors.New("不支持的认证方式")
	}
	if h.Auth == "key" && h.KeyPath == "" {
		return errors.New("请选择私钥文件")
	}
	if h.Proxy != "" {
		u, e := url.Parse(h.Proxy)
		if e != nil || u.Scheme != "socks5" || u.Host == "" {
			return errors.New("代理地址必须是 socks5://主机:端口")
		}
		if u.User != nil {
			return errors.New("代理地址中不能包含凭据；当前支持无认证 SOCKS5 代理")
		}
	}
	return nil
}
func (m *Manager) Trust(e *HostKeyError) error {
	if e.Changed {
		return errors.New("主机密钥变化不能作为首次连接接受，请先核实并移除旧记录")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := os.OpenFile(m.knownFile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, knownhosts.Line([]string{e.Address}, e.Key))
	return err
}
func (m *Manager) Connect(ctx context.Context, id string) (*ssh.Client, error) {
	return m.connect(ctx, id, map[string]bool{})
}
func (m *Manager) connect(ctx context.Context, id string, seen map[string]bool) (*ssh.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if seen[id] {
		return nil, errors.New("跳板机配置形成循环")
	}
	seen[id] = true
	h, err := m.hosts.Host(id)
	if err != nil {
		return nil, err
	}
	if err = ValidateHost(h); err != nil {
		return nil, err
	}
	if a, ok := ctx.Value(authorityKey{}).(authority); ok && a.HostID == id && a.Digest != domain.Digest(h) {
		return nil, errors.New("主机配置已变化，执行被阻止")
	}
	// Resolve jumps before taking the per-host lock, avoiding cross-chain lock inversion.
	var jump *ssh.Client
	if h.JumpID != "" {
		jump, err = m.connect(ctx, h.JumpID, seen)
		if err != nil {
			return nil, err
		}
	}
	l, _ := m.locks.LoadOrStore(id, &sync.Mutex{})
	lock := l.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("连接管理器已关闭")
	}
	old, ok := m.clients[id]
	m.mu.Unlock()
	if ok && old.digest == domain.Digest(h) {
		return old.client, nil
	}
	if ok {
		m.Disconnect(id)
	}
	addr := net.JoinHostPort(h.Address, strconv.Itoa(h.Port))
	auth, cleanup, err := m.auth(ctx, h)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			cleanup()
		}
	}()
	callback, err := knownhosts.New(m.knownFile)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{User: h.User, Auth: auth, Timeout: 20 * time.Second, HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}
		var ke *knownhosts.KeyError
		if errors.As(err, &ke) {
			return &HostKeyError{Address: addr, Fingerprint: ssh.FingerprintSHA256(key), Key: key, Changed: len(ke.Want) > 0}
		}
		return err
	}}
	var c net.Conn
	if jump != nil {
		c, err = jump.DialContext(ctx, "tcp", addr)
	} else {
		var d proxy.Dialer = &net.Dialer{Timeout: 20 * time.Second}
		if h.Proxy != "" {
			u, e := url.Parse(h.Proxy)
			if e != nil {
				return nil, e
			}
			d, err = proxy.FromURL(u, d)
			if err != nil {
				return nil, err
			}
		}
		cd, ok := d.(proxy.ContextDialer)
		if !ok {
			return nil, errors.New("代理不支持可取消的连接")
		}
		c, err = cd.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	c.SetDeadline(time.Now().Add(20 * time.Second))
	stop := context.AfterFunc(ctx, func() { c.Close() })
	cc, ch, req, err := ssh.NewClientConn(c, addr, cfg)
	stop()
	if err != nil {
		c.Close()
		return nil, err
	}
	c.SetDeadline(time.Time{})
	client := ssh.NewClient(cc, ch, req)
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		client.Close()
		return nil, errors.New("应用正在退出")
	}
	m.clients[id] = cached{client, domain.Digest(h), cleanup}
	m.mu.Unlock()
	success = true
	go func() {
		_ = client.Wait()
		m.mu.Lock()
		if cur, exists := m.clients[id]; exists && cur.client == client {
			delete(m.clients, id)
			cleanup()
		}
		m.mu.Unlock()
	}()
	return client, nil
}
func (m *Manager) auth(ctx context.Context, h domain.Host) ([]ssh.AuthMethod, func(), error) {
	cleanup := func() {}
	switch h.Auth {
	case "password":
		p, err := m.secrets.Get("host:" + h.ID)
		if err != nil {
			return nil, cleanup, fmt.Errorf("读取登录凭据: %w", err)
		}
		return []ssh.AuthMethod{ssh.Password(p)}, cleanup, nil
	case "key":
		b, err := os.ReadFile(h.KeyPath)
		if err != nil {
			return nil, cleanup, err
		}
		signer, err := ssh.ParsePrivateKey(b)
		var encrypted *ssh.PassphraseMissingError
		if errors.As(err, &encrypted) {
			p, e := m.secrets.Get("host:" + h.ID)
			if e != nil {
				return nil, cleanup, e
			}
			signer, err = ssh.ParsePrivateKeyWithPassphrase(b, []byte(p))
		}
		if err != nil {
			return nil, cleanup, err
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, cleanup, nil
	case "agent":
		c, err := dialAgent(ctx)
		if err != nil {
			return nil, cleanup, err
		}
		return []ssh.AuthMethod{ssh.PublicKeysCallback(sshagent.NewClient(c).Signers)}, func() { c.Close() }, nil
	case "interactive":
		if m.Prompt == nil {
			return nil, cleanup, errors.New("尚未配置交互认证")
		}
		return []ssh.AuthMethod{ssh.KeyboardInteractive(func(user, instruction string, questions []string, echo []bool) ([]string, error) {
			return m.Prompt(ctx, user, instruction, questions, echo)
		})}, cleanup, nil
	}
	return nil, cleanup, errors.New("unknown authentication")
}
func (m *Manager) Disconnect(id string) {
	m.mu.Lock()
	c, ok := m.clients[id]
	delete(m.clients, id)
	m.mu.Unlock()
	if ok {
		c.client.Close()
		c.cleanup()
	}
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	all := m.clients
	m.clients = map[string]cached{}
	m.mu.Unlock()
	for _, c := range all {
		c.client.Close()
		c.cleanup()
	}
}

func Quote(s string) string { return "'" + replaceQuotes(s) + "'" }
