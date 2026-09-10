package agent

import (
	"context"
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"slices"
	"strings"
	"time"
)

type Verification struct {
	HostID, ExecutionID, ExpectedText, AgentName string
	At                                           time.Time
}

func (s *Service) checkKnownResults(id string) error {
	results, err := s.Store.Results(id)
	if err != nil {
		return err
	}
	for _, r := range results {
		if r.Status == "unknown" || r.Status == "running" {
			return errors.New("存在结果未知的操作，请先核实；禁止重放变更")
		}
	}
	return nil
}
func (s *Service) verifyResult(t domain.Task, ctx context.Context, results []domain.Result, id, expected string) error {
	if executionAgentName(ctx) != "operator" {
		return correctable("最终验证由主 Agent 完成，请返回证据编号")
	}
	var found *domain.Result
	for i := range results {
		r := &results[i]
		if r.Status == "unknown" || r.Status == "running" {
			return errors.New("仍有结果未确认的执行")
		}
		if r.ID == id {
			found = r
		}
	}
	if found == nil || !slices.Contains(t.Grant.HostIDs, found.Request.HostID) {
		return correctable("证据不属于本任务")
	}
	if strings.HasPrefix(found.Request.Operation, "terminal_") || remote.Mutates(found.Request.Operation) || found.Status != "succeeded" || !strings.Contains(found.Output, expected) {
		return correctable("验证证据不满足条件。请获取新的成功只读检查并核对实际预期内容")
	}
	for _, r := range results {
		if r.Request.HostID == found.Request.HostID && r.StartedAt.After(found.StartedAt) {
			return correctable("请使用该服务器最后一次只读检查的证据")
		}
	}
	return s.Store.Put("verification:"+t.ID, found.Request.HostID, Verification{found.Request.HostID, id, expected, executionAgentName(ctx), time.Now()})
}
func (s *Service) hasOperations(id string) bool {
	results, err := s.Store.Results(id)
	return err != nil || len(results) > 0
}
func (s *Service) allVerified(id string) bool {
	results, err := s.Store.Results(id)
	if err != nil || len(results) == 0 {
		return false
	}
	last := map[string]domain.Result{}
	for _, r := range results {
		if r.Status == "unknown" || r.Status == "running" {
			return false
		}
		last[r.Request.HostID] = r
	}
	for host, r := range last {
		var v Verification
		if s.Store.Load("verification:"+id, host, &v) != nil || v.ExecutionID != r.ID {
			return false
		}
	}
	return true
}
