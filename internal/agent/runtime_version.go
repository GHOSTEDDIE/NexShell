package agent

import (
	"errors"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
)

const RuntimeVersion = "deep-v1"

func checkpointID(t domain.Task) string { return RuntimeVersion + ":" + t.ID }
func (s *Service) prepareVersion(t *domain.Task) error {
	if t.RuntimeVersion == RuntimeVersion {
		return nil
	}
	if t.RuntimeVersion != "" {
		return errors.New("不支持的 Agent 运行版本")
	}
	return errors.New("该对话需要升级运行状态，请确认范围后点击恢复；旧审批不能继续使用")
}
