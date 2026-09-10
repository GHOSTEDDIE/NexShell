package agent

import (
	"context"
	"fmt"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"gopkg.in/yaml.v3"
	"strings"
)

type skillBackend struct{ library SkillLibrary }

func (b skillBackend) Get(ctx context.Context, name string) (skill.Skill, error) {
	if err := ctx.Err(); err != nil {
		return skill.Skill{}, err
	}
	text, err := b.library.Read(name)
	if err != nil {
		return skill.Skill{}, err
	}
	parts := strings.SplitN(text, "---", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[0]) != "" {
		return skill.Skill{}, fmt.Errorf("技能缺少 YAML 头: %s", name)
	}
	var meta skill.FrontMatter
	if err = yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return skill.Skill{}, err
	}
	if meta.Context != "" || meta.Agent != "" || meta.Model != "" {
		return skill.Skill{}, fmt.Errorf("技能不能自行创建子 Agent 或切换模型，请通过主 Agent 委派: %s", name)
	}
	meta.Name = name
	return skill.Skill{FrontMatter: meta, Content: parts[2]}, nil
}
func (b skillBackend) List(ctx context.Context) ([]skill.FrontMatter, error) {
	names, err := b.library.Read("")
	if err != nil {
		return nil, err
	}
	out := []skill.FrontMatter{}
	for _, name := range strings.Split(names, "\n") {
		if name == "" {
			continue
		}
		s, err := b.Get(ctx, name)
		if err != nil {
			return nil, err
		}
		out = append(out, s.FrontMatter)
	}
	return out, nil
}
