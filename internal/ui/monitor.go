package ui

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/remote"
	"sort"
	"strconv"
	"strings"
	"time"
)

func emptyServerStatus() fyne.CanvasObject {
	l := widget.NewLabel("连接服务器后，在此查看运行状态")
	l.Wrapping = fyne.TextWrapWord
	l.SizeName = sizeBody
	return padded(l, 18)
}
func (u *App) selectWorkspace(item *container.TabItem) {
	u.terminalsDesktop.Select("")
	content := emptyServerStatus()
	contextName := "尚未连接服务器"
	for _, ws := range u.workspaces {
		if ws.tab != item || ws.ctx.Err() != nil {
			continue
		}
		u.selected = ws.host.ID
		contextName = "当前连接 · " + ws.host.Name
		if len(ws.terminalIDs) > 0 {
			u.terminalsDesktop.Select(ws.terminalIDs[0])
		}
		header := inset(container.NewBorder(nil, nil, inset(widget.NewIcon(designIcon("server")), 0, 10, 0, 0), nil, container.NewVBox(headingText(ws.host.Name), metaText(ws.host.User+"@"+ws.host.Address))), 18, 18, 12, 18)
		content = container.NewBorder(header, nil, nil, nil, ws.monitor)
		break
	}
	u.serverStatus.Objects = []fyne.CanvasObject{content}
	u.serverStatus.Refresh()
	if u.assistantContext != nil {
		u.assistantContext.SetText(contextName)
	}
	u.refreshDesktopState()
}
func metricState(m remote.Metric) string {
	switch m.State {
	case "warming_up":
		return "—"
	case "unsupported":
		return "不支持采集"
	case "error":
		return "采集失败"
	}
	return "暂无数据"
}
func metricText(m remote.Metric) string {
	if m.State != "ok" {
		return metricState(m)
	}
	if v, ok := m.Value.(string); ok {
		return v
	}
	b, _ := json.MarshalIndent(m.Value, "", "  ")
	return string(b)
}

type meterView struct {
	widget.BaseWidget
	value float64
}

func newMeter() *meterView              { m := &meterView{}; m.ExtendBaseWidget(m); return m }
func (m *meterView) SetValue(v float64) { m.value = max(0, min(1, v)); m.Refresh() }
func (m *meterView) CreateRenderer() fyne.WidgetRenderer {
	r := &meterRenderer{m: m, track: canvas.NewRectangle(themedColor(colorTrack)), fill: canvas.NewRectangle(themedColor(colorMeter))}
	r.track.CornerRadius = 2.5
	r.fill.CornerRadius = 2.5
	return r
}

type meterRenderer struct {
	m           *meterView
	track, fill *canvas.Rectangle
}

func (r *meterRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.track, r.fill} }
func (r *meterRenderer) Destroy()                     {}
func (r *meterRenderer) MinSize() fyne.Size           { return fyne.NewSize(100, 5) }
func (r *meterRenderer) Layout(s fyne.Size) {
	r.track.Resize(s)
	r.fill.Resize(fyne.NewSize(s.Width*float32(r.m.value), s.Height))
}
func (r *meterRenderer) Refresh() {
	r.track.FillColor = themedColor(colorTrack)
	r.fill.FillColor = themedColor(colorMeter)
	r.Layout(r.m.Size())
	r.track.Refresh()
	r.fill.Refresh()
}

type metricCard struct {
	value, detail *textView
	bar           *meterView
	content       fyne.CanvasObject
}

func newMetricCard(name string) *metricCard {
	v := headingText("—")
	v.Alignment = fyne.TextAlignTrailing
	m := &metricCard{value: v, detail: metaText("—"), bar: newMeter()}
	m.content = container.NewVBox(container.NewBorder(nil, nil, textUI(name, sizeControl, colorMuted, false), v), gap(3), m.bar, gap(2), m.detail)
	return m
}
func (m *metricCard) unavailable(s string) {
	m.value.SetText("—")
	m.detail.SetText(s)
	m.bar.SetValue(0)
}

type serverMetrics struct {
	details          *widget.Accordion
	content          fyne.CanvasObject
	stamp, state     *textView
	cpu, memory      *metricCard
	disks, network   *fyne.Container
	system, uptime   *textView
	processes, ports *monitorTable
	lastDisk         string
}

func newServerMetrics(parents ...fyne.Window) *serverMetrics {
	var parent fyne.Window
	if len(parents) > 0 {
		parent = parents[0]
	}
	m := &serverMetrics{stamp: metaText("等待采集"), state: textUI("● 已连接", sizeMeta, theme.ColorNameSuccess, false), cpu: newMetricCard("CPU 使用率"), memory: newMetricCard("内存使用率"), disks: container.NewVBox(metaText("等待采集")), network: container.NewVBox(metaText("等待采集")), system: metaText("—"), uptime: metaText("—"), processes: newMonitorTable("进程", parent), ports: newMonitorTable("端口连接", parent)}
	section := func(title string, body fyne.CanvasObject) fyne.CanvasObject {
		return container.NewVBox(gap(10), widget.NewSeparator(), gap(10), headingText(title), gap(6), body)
	}
	details := widget.NewAccordion(widget.NewAccordionItem("进程", m.processes.content), widget.NewAccordionItem("端口连接", m.ports.content))
	m.details = details
	all := container.NewVBox(container.NewBorder(nil, nil, m.state, m.stamp), gap(10), widget.NewSeparator(), gap(14), m.cpu.content, gap(16), m.memory.content, section("磁盘", m.disks), section("网络", m.network), section("系统信息", container.NewVBox(m.system, m.uptime)), gap(12), details)
	m.content = container.NewVScroll(inset(all, 0, 18, 18, 18))
	return m
}
func (m *serverMetrics) apply(s remote.Snapshot, err error) {
	if err != nil {
		m.state.SetText("采集失败")
		m.stamp.SetText("数据已过期")
		m.cpu.unavailable("数据已过期")
		m.memory.unavailable("数据已过期")
		m.disks.Objects = []fyne.CanvasObject{metaText("数据已过期")}
		m.network.Objects = []fyne.CanvasObject{metaText("数据已过期")}
		m.lastDisk = ""
		m.system.SetText("数据已过期")
		m.uptime.SetText("")
		m.processes.unavailable("数据已过期")
		m.ports.unavailable("数据已过期")
		m.content.Refresh()
		return
	}
	m.state.SetText("● 已连接")
	m.stamp.SetText("更新于 " + s.At.Format("15:04:05"))
	if v, ok := s.CPU.Value.(float64); ok && s.CPU.State == "ok" {
		m.cpu.value.SetText(fmt.Sprintf("%.1f%%", v))
		m.cpu.bar.SetValue(v / 100)
		f := strings.Fields(metricText(s.Load))
		if len(f) > 3 {
			f = f[:3]
		}
		m.cpu.detail.SetText("负载 " + strings.Join(f, " / "))
	} else {
		m.cpu.unavailable(metricState(s.CPU))
	}
	if mem, ok := s.Memory.Value.(map[string]uint64); ok && s.Memory.State == "ok" && mem["total"] > 0 {
		ratio := float64(mem["used"]) / float64(mem["total"])
		m.memory.value.SetText(fmt.Sprintf("%.1f%%", ratio*100))
		m.memory.bar.SetValue(ratio)
		m.memory.detail.SetText(fmt.Sprintf("已用 %.1f / 共 %.1f GiB", float64(mem["used"])/(1<<30), float64(mem["total"])/(1<<30)))
	} else {
		m.memory.unavailable(metricState(s.Memory))
	}
	disks := metricText(s.Disks)
	if disks != m.lastDisk {
		m.lastDisk = disks
		m.disks.Objects = nil
		rows, e := remote.ParseDiskUsage(disks)
		if e != nil || s.Disks.State != "ok" {
			m.disks.Add(metaText(metricState(s.Disks)))
			if s.Disks.State == "ok" {
				m.disks.Objects = []fyne.CanvasObject{metaText("磁盘数据格式错误")}
			}
		} else {
			for _, d := range rows {
				card := newMetricCard(d.Mount)
				card.value.SetText(fmt.Sprintf("%d%%", d.Percent))
				card.bar.SetValue(float64(d.Percent) / 100)
				card.detail.SetText(diskSize(float64(d.UsedKiB)) + " / " + diskSize(float64(d.TotalKiB)))
				m.disks.Add(card.content)
				m.disks.Add(gap(8))
			}
			if len(rows) == 0 {
				m.disks.Add(metaText("暂无磁盘"))
			}
		}
		m.disks.Refresh()
	}
	m.network.Objects = nil
	if rates, ok := s.Network.Value.(map[string]remote.NetworkRate); ok && s.Network.State == "ok" {
		names := make([]string, 0, len(rates))
		for name := range rates {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			r := rates[name]
			m.network.Add(textUI(name, sizeMeta, colorMuted, false))
			m.network.Add(container.NewBorder(nil, nil, metaText("↓ 接收"), bodyText(rateSize(r.ReceiveBytesPerSecond))))
			m.network.Add(container.NewBorder(nil, nil, metaText("↑ 发送"), bodyText(rateSize(r.SendBytesPerSecond))))
		}
		if len(names) == 0 {
			m.network.Add(metaText("暂无网卡"))
		}
	} else {
		m.network.Add(metaText(metricState(s.Network)))
	}
	m.network.Refresh()
	m.system.SetText(metricText(s.System))
	m.uptime.SetText("运行时间 " + formatUptime(s.Uptime))
	m.processes.update(s.Processes)
	m.ports.update(s.Ports)
}
func rateSize(v float64) string { return strings.ReplaceAll(diskSize(v/1024), "i", "") + "/s" }
func formatUptime(m remote.Metric) string {
	f := strings.Fields(metricText(m))
	if m.State != "ok" || len(f) == 0 {
		return metricState(m)
	}
	v, e := strconv.ParseFloat(f[0], 64)
	if e != nil || v < 0 {
		return "采集失败"
	}
	return fmt.Sprintf("%d 天 %d 时", int(v)/86400, int(v)%86400/3600)
}
func (w *workspace) monitorPane() fyne.CanvasObject {
	m := newServerMetrics(w.u.Window)
	go func() {
		var prev *remote.Snapshot
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for {
			s, e := w.u.Manager.Monitor(w.ctx, w.host.ID, prev)
			if e == nil {
				prev = &s
			}
			if w.ctx.Err() != nil {
				return
			}
			fyne.DoAndWait(func() {
				if w.ctx.Err() == nil {
					m.apply(s, e)
				}
			})
			select {
			case <-t.C:
			case <-w.ctx.Done():
				return
			}
		}
	}()
	return m.content
}
