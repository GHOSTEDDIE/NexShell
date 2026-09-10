package agent

import (
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"time"
)

// A conversation can start without a server; access is limited to selected hosts.
func (s *Service) NewConversation(profile string, hosts []string) (domain.Task, error) {
	var model domain.ModelProfile
	if err := s.Store.Load("models", profile, &model); err != nil {
		return domain.Task{}, err
	}
	t := domain.Task{RuntimeVersion: RuntimeVersion, ID: domain.ID(), Mode: "conversation", Goal: "与用户持续协作，解释问题并按需操作服务器", ProfileID: profile, Status: "ready", CreatedAt: time.Now(), Grant: domain.Grant{ID: domain.ID(), HostIDs: append([]string(nil), hosts...), Operations: []string{"observe", "service_status", "logs", "terminal_connect", "terminal_read"}, Identities: map[string]string{}, ExpiresAt: time.Now().Add(8 * time.Hour)}}
	for _, id := range hosts {
		h, err := s.Store.Host(id)
		if err != nil {
			return t, err
		}
		t.Grant.Identities[id] = domain.Digest(h)
	}
	return t, s.Store.Put("tasks", t.ID, t)
}
