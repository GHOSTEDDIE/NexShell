package store

import "github.com/zalando/go-keyring"

type Credentials struct{}

func (Credentials) Get(id string) (string, error) { return keyring.Get("NexShell", id) }
func (Credentials) Set(id, secret string) error   { return keyring.Set("NexShell", id, secret) }
func (Credentials) Delete(id string) error        { return keyring.Delete("NexShell", id) }
