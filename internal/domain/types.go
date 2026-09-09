package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type Host struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Address string   `json:"address"`
	Port    int      `json:"port"`
	User    string   `json:"user"`
	Group   string   `json:"group"`
	Tags    []string `json:"tags,omitempty"`
	Auth    string   `json:"auth"` // password, key, agent, interactive
	KeyPath string   `json:"key_path,omitempty"`
	JumpID  string   `json:"jump_id,omitempty"`
	Proxy   string   `json:"proxy,omitempty"` // socks5://host:port
}

type ModelProfile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	BaseURL       string `json:"base_url"`
	Model         string `json:"model"`
	ContextTokens int    `json:"context_tokens"`
}

type Grant struct {
	ID         string            `json:"id"`
	HostIDs    []string          `json:"host_ids"`
	Identities map[string]string `json:"identities"` // snapshot digest of host connection configuration
	Operations []string          `json:"operations"`
	Resources  []string          `json:"resources"`
	ExpiresAt  time.Time         `json:"expires_at"`
}

type Task struct {
	Mode      string    `json:"mode,omitempty"`
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	ProfileID string    `json:"profile_id"`
	Grant     Grant     `json:"grant"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Summary   string    `json:"summary,omitempty"`
	Updates   []string  `json:"updates,omitempty"`
}

type Request struct {
	TaskID         string `json:"task_id"`
	CallID         string `json:"call_id"`
	HostID         string `json:"host_id"`
	Operation      string `json:"operation"`
	Resource       string `json:"resource,omitempty"`
	Command        string `json:"command,omitempty"`
	Content        string `json:"content,omitempty"`
	ExpectedHash   string `json:"expected_hash,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type Result struct {
	ID         string    `json:"id"`
	Request    Request   `json:"request"`
	Status     string    `json:"status"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `json:"output,omitempty"`
	Artifact   string    `json:"artifact,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}

type Event struct {
	Sequence int64     `json:"sequence"`
	TaskID   string    `json:"task_id"`
	Kind     string    `json:"kind"`
	Text     string    `json:"text"`
	Time     time.Time `json:"time"`
}

type Snippet struct{ ID, Name, Command string }
