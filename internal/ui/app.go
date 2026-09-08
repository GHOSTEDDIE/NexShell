package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/agent"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
)

type App struct {
	UI         fyne.App
	Window     fyne.Window
	Store      *store.Store
	Manager    *remote.Manager
	Agent      *agent.Service
	Executor   *remote.Executor
	ctx        context.Context
	cancel     context.CancelFunc
	hosts      []domain.Host
	filtered   []domain.Host
	list       *widget.List
	selected   string
	tabs       *container.DocTabs
	status     *widget.Label
	search     *widget.Entry
	workspaces []*workspace
	taskID     string
	taskText   *readOnly
	taskStatus *widget.Label
	taskList   *widget.Select
	tunnels    []*remote.Tunnel
}

func New(app fyne.App, s *store.Store, m *remote.Manager, e *remote.Executor, a *agent.Service) *App {
	app.Settings().SetTheme(NewTheme(true))
	ctx, cancel := context.WithCancel(context.Background())
	u := &App{UI: app, Store: s, Manager: m, Agent: a, Executor: e, ctx: ctx, cancel: cancel}
	u.Window = app.NewWindow("NexShell")
	u.Window.Resize(fyne.NewSize(1380, 900))
	u.status = widget.NewLabel("就绪")
	u.tabs = container.NewDocTabs()
	u.tabs.SetTabLocation(container.TabLocationTop)
	u.tabs.Append(container.NewTabItem("工作空间", container.NewCenter(container.NewVBox(widget.NewLabelWithStyle("连接你的服务器", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), widget.NewLabel("终端 · 文件 · 监控 · 运维助手"), widget.NewButtonWithIcon("添加服务器", theme.ContentAddIcon(), func() { u.editHost(nil) })))))
	u.tabs.OnClosed = func(item *container.TabItem) {
		for _, ws := range u.workspaces {
			if ws.tab == item {
				ws.close()
			}
		}
	}
	u.search = widget.NewEntry()
	u.search.SetPlaceHolder("搜索名称、地址或标签")
	u.search.OnChanged = func(string) { u.filterHosts() }
	u.list = widget.NewList(func() int { return len(u.filtered) }, func() fyne.CanvasObject {
		return container.NewVBox(widget.NewLabelWithStyle("服务器", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewLabel("地址"))
	}, func(i widget.ListItemID, obj fyne.CanvasObject) {
		h := u.filtered[i]
		box := obj.(*fyne.Container)
		box.Objects[0].(*widget.Label).SetText(h.Name)
		box.Objects[1].(*widget.Label).SetText(h.User + "@" + h.Address + " · " + h.Group)
	})
	u.list.OnSelected = func(i widget.ListItemID) {
		if i >= 0 && i < len(u.filtered) {
			u.selected = u.filtered[i].ID
		}
	}
	side := container.NewBorder(container.NewVBox(widget.NewLabelWithStyle("服务器", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), u.search), container.NewGridWithColumns(2, widget.NewButton("连接", func() { u.connectSelected() }), widget.NewButton("编辑", func() {
		h, e := s.Host(u.selected)
		if e != nil {
			u.error(e)
			return
		}
		u.editHost(&h)
	})), nil, nil, u.list)
	center := container.NewHSplit(side, u.tabs)
	center.Offset = .19
	right := u.agentPanel()
	split := container.NewHSplit(center, right)
	split.Offset = .76
	toolbar := container.NewHBox(widget.NewLabelWithStyle("NexShell", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator(), widget.NewButtonWithIcon("添加主机", theme.ContentAddIcon(), func() { u.editHost(nil) }), widget.NewButton("导入", u.importHosts), widget.NewButton("导出", u.exportHosts), widget.NewButton("模型设置", u.modelDialog), widget.NewButton("批量执行", u.batchDialog), widget.NewButton("端口转发", u.tunnelDialog), widget.NewButton("快捷命令", u.snippetsDialog), widget.NewButton("删除主机", u.deleteHost), widget.NewButton("主题", func() {
		if app.Settings().ThemeVariant() == theme.VariantDark {
			app.Settings().SetTheme(NewTheme(false))
		} else {
			app.Settings().SetTheme(NewTheme(true))
		}
	}))
	u.Window.SetContent(container.NewBorder(toolbar, u.status, nil, nil, split))
	u.refreshHosts()
	u.refreshTasks()
	m.Prompt = u.prompt
	u.Window.SetCloseIntercept(func() {
		u.Window.SetCloseIntercept(nil)
		u.status.SetText("正在保存任务与关闭连接…")
		u.cancel()
		for _, ws := range u.workspaces {
			ws.close()
		}
		for _, t := range u.tunnels {
			t.Close()
		}
		go func() { u.Agent.Close(); u.Manager.Close(); _ = u.Store.Close(); fyne.Do(func() { u.Window.Close() }) }()
	})
	go u.watchTasks()
	return u
}
func (u *App) Show() { u.Window.ShowAndRun() }
func (u *App) error(err error) {
	if err != nil {
		dialog.ShowError(err, u.Window)
	}
}
func (u *App) work(label string, fn func() error) {
	u.status.SetText(label)
	go func() {
		err := fn()
		fyne.Do(func() {
			if err != nil {
				u.status.SetText("操作失败")
				u.error(err)
			} else {
				u.status.SetText("就绪")
			}
		})
	}()
}
func (u *App) refreshHosts() {
	hosts, err := store.All[domain.Host](u.Store, "hosts")
	if err != nil {
		u.error(err)
		return
	}
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].Group != hosts[j].Group {
			return hosts[i].Group < hosts[j].Group
		}
		return hosts[i].Name < hosts[j].Name
	})
	u.hosts = hosts
	u.filterHosts()
}
func (u *App) filterHosts() {
	q := strings.ToLower(u.search.Text)
	u.filtered = nil
	for _, h := range u.hosts {
		if strings.Contains(strings.ToLower(h.Name+" "+h.Address+" "+h.Group+" "+strings.Join(h.Tags, " ")), q) {
			u.filtered = append(u.filtered, h)
		}
	}
	u.list.Refresh()
}
func (u *App) connectSelected() {
	if u.selected == "" {
		u.error(errors.New("请选择服务器"))
		return
	}
	h, e := u.Store.Host(u.selected)
	if e != nil {
		u.error(e)
		return
	}
	u.connect(h)
}
func (u *App) connect(h domain.Host) {
	u.status.SetText("正在连接 " + h.Name)
	go func() {
		s, e := u.Manager.Terminal(u.ctx, h.ID, 24, 80)
		fyne.Do(func() {
			if e != nil {
				var key *remote.HostKeyError
				if errors.As(e, &key) && !key.Changed {
					dialog.ShowConfirm("核对服务器身份", key.Address+"\n"+key.Fingerprint+"\n确认与服务器管理员提供的指纹一致后连接。", func(ok bool) {
						if ok {
							if err := u.Manager.Trust(key); err != nil {
								u.error(err)
							} else {
								u.connect(h)
							}
						}
					}, u.Window)
				} else {
					u.error(e)
				}
				u.status.SetText("连接失败")
				return
			}
			ws := u.newWorkspace(h, s)
			u.workspaces = append(u.workspaces, ws)
			u.tabs.Append(ws.tab)
			u.tabs.Select(ws.tab)
			u.status.SetText("已连接 " + h.Name)
		})
	}()
}
func (u *App) editHost(existing *domain.Host) {
	h := domain.Host{ID: domain.ID(), Port: 22, Auth: "password", User: "root"}
	if existing != nil {
		h = *existing
	}
	entry := func(value string) *widget.Entry { e := widget.NewEntry(); e.SetText(value); return e }
	name, address, user, group, tags, key, proxy := entry(h.Name), entry(h.Address), entry(h.User), entry(h.Group), entry(strings.Join(h.Tags, ",")), entry(h.KeyPath), entry(h.Proxy)
	port := entry(strconv.Itoa(h.Port))
	secret := widget.NewPasswordEntry()
	secret.SetPlaceHolder("留空保留已保存凭据")
	auth := widget.NewSelect([]string{"password", "key", "agent", "interactive"}, nil)
	auth.SetSelected(h.Auth)
	jumpNames := []string{"直连"}
	jumps := map[string]string{"直连": ""}
	selectedJump := "直连"
	for _, v := range u.hosts {
		if v.ID == h.ID {
			continue
		}
		label := v.Name + " · " + v.ID[:8]
		jumpNames = append(jumpNames, label)
		jumps[label] = v.ID
		if v.ID == h.JumpID {
			selectedJump = label
		}
	}
	jump := widget.NewSelect(jumpNames, nil)
	jump.SetSelected(selectedJump)
	items := []*widget.FormItem{widget.NewFormItem("名称", name), widget.NewFormItem("地址", address), widget.NewFormItem("端口", port), widget.NewFormItem("用户名", user), widget.NewFormItem("分组", group), widget.NewFormItem("标签", tags), widget.NewFormItem("认证方式", auth), widget.NewFormItem("密码 / 私钥口令", secret), widget.NewFormItem("私钥文件", key), widget.NewFormItem("跳板机", jump), widget.NewFormItem("SOCKS5 代理", proxy)}
	d := dialog.NewForm("服务器设置", "保存", "取消", items, func(ok bool) {
		if !ok {
			return
		}
		p, err := strconv.Atoi(port.Text)
		if err != nil {
			u.error(err)
			return
		}
		h.Name = name.Text
		h.Address = address.Text
		h.Port = p
		h.User = user.Text
		h.Group = group.Text
		h.Tags = splitLines(strings.ReplaceAll(tags.Text, ",", "\n"))
		h.Auth = auth.Selected
		h.KeyPath = key.Text
		h.Proxy = proxy.Text
		h.JumpID = jumps[jump.Selected]
		if err = remote.ValidateHost(h); err != nil {
			u.error(err)
			return
		}
		password := secret.Text
		secret.SetText("")
		u.work("保存服务器", func() error {
			if password != "" {
				if err := (store.Credentials{}).Set("host:"+h.ID, password); err != nil {
					return err
				}
			}
			if err := u.Store.Put("hosts", h.ID, h); err != nil {
				return err
			}
			u.Manager.Disconnect(h.ID)
			fyne.Do(u.refreshHosts)
			return nil
		})
	}, u.Window)
	d.Resize(fyne.NewSize(570, 680))
	d.Show()
}
func splitLines(s string) []string {
	var out []string
	for _, x := range strings.Split(s, "\n") {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
func (u *App) importHosts() {
	dialog.ShowFileOpen(func(r fyne.URIReadCloser, e error) {
		if e != nil {
			u.error(e)
			return
		}
		if r == nil {
			return
		}
		defer r.Close()
		var hosts []domain.Host
		if e = json.NewDecoder(io.LimitReader(r, 4*1024*1024)).Decode(&hosts); e != nil {
			u.error(e)
			return
		}
		for _, h := range hosts {
			if e = remote.ValidateHost(h); e != nil {
				u.error(e)
				return
			}
		}
		for _, h := range hosts {
			if e = u.Store.Put("hosts", h.ID, h); e != nil {
				u.error(e)
				return
			}
		}
		u.refreshHosts()
	}, u.Window)
}
func (u *App) exportHosts() {
	dialog.ShowFileSave(func(w fyne.URIWriteCloser, e error) {
		if e != nil {
			u.error(e)
			return
		}
		if w == nil {
			return
		}
		defer w.Close()
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		u.error(enc.Encode(u.hosts))
	}, u.Window)
}
func (u *App) modelDialog() {
	profiles, e := store.All[domain.ModelProfile](u.Store, "models")
	if e != nil {
		u.error(e)
		return
	}
	p := domain.ModelProfile{ID: "default", Name: "默认模型", Provider: "openai", BaseURL: "https://api.openai.com/v1", ContextTokens: 32000}
	if len(profiles) > 0 {
		p = profiles[0]
	}
	provider := widget.NewSelect([]string{"openai", "ollama"}, nil)
	provider.SetSelected(p.Provider)
	base := widget.NewEntry()
	base.SetText(p.BaseURL)
	modelName := widget.NewEntry()
	modelName.SetText(p.Model)
	key := widget.NewPasswordEntry()
	key.SetPlaceHolder("留空保留已保存密钥")
	tokens := widget.NewEntry()
	tokens.SetText(strconv.Itoa(p.ContextTokens))
	provider.OnChanged = func(v string) {
		if v == "ollama" {
			base.SetText("http://localhost:11434")
		} else {
			base.SetText("https://api.openai.com/v1")
		}
	}
	dialog.ShowForm("模型设置", "保存", "取消", []*widget.FormItem{widget.NewFormItem("接口类型", provider), widget.NewFormItem("服务地址", base), widget.NewFormItem("模型名称", modelName), widget.NewFormItem("密钥", key), widget.NewFormItem("上下文容量", tokens)}, func(ok bool) {
		if !ok {
			return
		}
		p.Provider = provider.Selected
		p.BaseURL = base.Text
		p.Model = modelName.Text
		n, e := strconv.Atoi(tokens.Text)
		if e != nil || n < 4096 {
			u.error(errors.New("上下文容量至少为 4096"))
			return
		}
		p.ContextTokens = n
		secret := key.Text
		key.SetText("")
		u.work("保存模型设置", func() error {
			if secret != "" {
				if e := (store.Credentials{}).Set("model:"+p.ID, secret); e != nil {
					return e
				}
			}
			return u.Store.Put("models", p.ID, p)
		})
	}, u.Window)
}
func (u *App) prompt(ctx context.Context, user, instruction string, questions []string, echo []bool) ([]string, error) {
	type answer struct {
		v []string
		e error
	}
	result := make(chan answer, 1)
	fyne.Do(func() {
		fields := make([]*widget.Entry, len(questions))
		var items []*widget.FormItem
		for i, q := range questions {
			fields[i] = widget.NewEntry()
			if i >= len(echo) || !echo[i] {
				fields[i].Password = true
			}
			items = append(items, widget.NewFormItem(q, fields[i]))
		}
		dialog.ShowForm("身份验证 · "+user, "确认", "取消", items, func(ok bool) {
			if !ok {
				result <- answer{e: errors.New("身份验证已取消")}
				return
			}
			values := make([]string, len(fields))
			for i, e := range fields {
				values[i] = e.Text
				e.SetText("")
			}
			result <- answer{v: values}
		}, u.Window)
	})
	select {
	case a := <-result:
		return a.v, a.e
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (u *App) hostChoices() ([]string, map[string]string) {
	var names []string
	ids := map[string]string{}
	for _, h := range u.hosts {
		n := h.Name + " · " + h.User + "@" + h.Address + " · " + h.ID[:8]
		names = append(names, n)
		ids[n] = h.ID
	}
	return names, ids
}
func (u *App) tunnelDialog() {
	h, e := u.Store.Host(u.selected)
	if e != nil {
		u.error(errors.New("请先选择服务器"))
		return
	}
	kind := widget.NewSelect([]string{"local", "remote", "dynamic"}, nil)
	kind.SetSelected("local")
	bind := widget.NewEntry()
	bind.SetText("127.0.0.1:8080")
	dest := widget.NewEntry()
	dest.SetText("127.0.0.1:80")
	dialog.ShowForm("端口转发 · "+h.Name, "开启", "取消", []*widget.FormItem{widget.NewFormItem("类型", kind), widget.NewFormItem("监听地址", bind), widget.NewFormItem("目标地址", dest)}, func(ok bool) {
		if !ok {
			return
		}
		k, b, d := kind.Selected, bind.Text, dest.Text
		u.work("开启转发", func() error {
			t, e := u.Manager.Tunnel(u.ctx, h.ID, k, b, d)
			if e != nil {
				return e
			}
			fyne.Do(func() {
				u.tunnels = append(u.tunnels, t)
				dialog.ShowCustom("转发已开启", "关闭窗口", container.NewVBox(widget.NewLabel(t.Address+" → "+d), widget.NewButton("停止转发", func() { t.Close(); u.status.SetText("转发已停止") })), u.Window)
			})
			return nil
		})
	}, u.Window)
}
func (u *App) batchDialog() {
	names, ids := u.hostChoices()
	hosts := widget.NewCheckGroup(names, nil)
	cmd := widget.NewMultiLineEntry()
	cmd.SetMinRowsVisible(4)
	dialog.ShowForm("批量执行", "执行", "取消", []*widget.FormItem{widget.NewFormItem("目标服务器", hosts), widget.NewFormItem("命令", cmd)}, func(ok bool) {
		if !ok {
			return
		}
		if len(hosts.Selected) == 0 || strings.TrimSpace(cmd.Text) == "" {
			u.error(errors.New("请选择服务器并填写命令"))
			return
		}
		command := cmd.Text
		task := domain.ID()
		output := widget.NewMultiLineEntry()
		output.SetMinRowsVisible(18)
		dialog.ShowCustom("批量执行结果", "关闭", output, u.Window)
		for _, name := range hosts.Selected {
			id := ids[name]
			go func() {
				r, e := u.Executor.Execute(u.ctx, domain.Request{TaskID: task, CallID: domain.ID(), HostID: id, Operation: "shell", Command: command, TimeoutSeconds: 120})
				text := fmt.Sprintf("\n%s · %s\n%s\n", name, r.Status, r.Output)
				if e != nil {
					text += e.Error()
				}
				fyne.Do(func() { output.SetText(output.Text + text) })
			}()
		}
	}, u.Window)
}
func (u *App) snippetsDialog() {
	name := widget.NewEntry()
	command := widget.NewMultiLineEntry()
	command.SetMinRowsVisible(3)
	snippets, _ := store.All[domain.Snippet](u.Store, "snippets")
	var options []string
	byName := map[string]domain.Snippet{}
	for _, s := range snippets {
		options = append(options, s.Name)
		byName[s.Name] = s
	}
	pick := widget.NewSelect(options, func(n string) { s := byName[n]; name.SetText(s.Name); command.SetText(s.Command) })
	dialog.ShowCustom("快捷命令", "关闭", container.NewVBox(pick, widget.NewForm(widget.NewFormItem("名称", name), widget.NewFormItem("命令", command)), container.NewHBox(widget.NewButton("保存", func() {
		if name.Text == "" || command.Text == "" {
			return
		}
		u.error(u.Store.Put("snippets", name.Text, domain.Snippet{ID: name.Text, Name: name.Text, Command: command.Text}))
	}), widget.NewButton("插入当前终端", func() {
		u.insertSnippet(command.Text)
	}))), u.Window)
}
func (u *App) openLocal(path string) {
	uri, _ := url.Parse("file://" + filepath.ToSlash(path))
	u.error(u.UI.OpenURL(uri))
}
