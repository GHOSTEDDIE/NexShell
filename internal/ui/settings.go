package ui

import (
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/domain"
	"github.com/GHOSTEDDIE/nexshell/internal/store"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"image/color"
	"strconv"
	"strings"
)

type componentTheme struct{ body, clearInput bool }

func (t componentTheme) Font(s fyne.TextStyle) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Font(s)
}
func (t componentTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Icon(n)
}
func (t componentTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if t.clearInput && (n == theme.ColorNameInputBackground || n == theme.ColorNameInputBorder) {
		return color.Transparent
	}
	return fyne.CurrentApp().Settings().Theme().Color(n, v)
}
func (t componentTheme) Size(n fyne.ThemeSizeName) float32 {
	if t.body && n == theme.SizeNameText {
		n = sizeBody
	}
	return fyne.CurrentApp().Settings().Theme().Size(n)
}
func formField(name string, obj fyne.CanvasObject, hint string) fyne.CanvasObject {
	items := []fyne.CanvasObject{textUI(name, sizeControl, theme.ColorNameForeground, false), container.NewThemeOverride(obj, componentTheme{body: true})}
	if hint != "" {
		l := widget.NewRichText(&widget.TextSegment{Text: hint, Style: widget.RichTextStyle{Inline: true, SizeName: sizeMeta, ColorName: colorMuted}})
		l.Wrapping = fyne.TextWrapWord
		items = append(items, l)
	}
	return container.NewVBox(items...)
}
func (u *App) settingsDialog(page string) {
	if u.settingsPopup != nil {
		u.settingsPopup.hideImmediately()
	}
	profiles, e := store.All[domain.ModelProfile](u.Store, "models")
	if e != nil {
		u.error(e)
		return
	}
	p := domain.ModelProfile{ID: "default", Name: "默认模型", Provider: "openai", BaseURL: "https://api.openai.com/v1", ContextTokens: 32000}
	if len(profiles) > 0 {
		p = profiles[0]
	}
	provider := widget.NewSelect([]string{"OpenAI 兼容接口", "Ollama"}, nil)
	provider.SetSelected("OpenAI 兼容接口")
	if p.Provider == "ollama" {
		provider.SetSelected("Ollama")
	}
	base := widget.NewEntry()
	base.SetText(p.BaseURL)
	modelName := widget.NewEntry()
	modelName.SetText(p.Model)
	modelName.SetPlaceHolder("输入服务提供的模型名称")
	key := widget.NewPasswordEntry()
	key.SetPlaceHolder("留空保留已保存密钥")
	tokens := widget.NewEntry()
	tokens.SetText(strconv.Itoa(p.ContextTokens))
	provider.OnChanged = func(v string) {
		if base.Text == "https://api.openai.com/v1" || base.Text == "http://localhost:11434" || base.Text == "" {
			if v == "Ollama" {
				base.SetText("http://localhost:11434")
			} else {
				base.SetText("https://api.openai.com/v1")
			}
		}
	}
	modelSummary := p.Model
	if modelSummary == "" {
		modelSummary = "尚未配置模型"
	}
	summary := panel(padded(container.NewBorder(nil, nil, inset(widget.NewIcon(designIcon("spark")), 0, 10, 0, 0), textUI("默认", sizeMeta, theme.ColorNameSuccess, false), container.NewVBox(headingText(p.Name), metaText(provider.Selected+" · "+modelSummary))), 14), colorPanel, true, 6)
	modelPane := container.NewVScroll(inset(container.NewVBox(headingText("模型配置"), metaText("配置助手使用的模型服务。"), gap(12), summary, gap(14), container.NewGridWithColumns(2, formField("接口类型", provider, ""), formField("模型名称", modelName, "")), gap(8), formField("服务地址", base, "填写模型服务的接口地址。"), gap(8), formField("API 密钥", key, "留空保留已保存密钥。"), gap(8), formField("上下文容量", tokens, "单位：Token，至少 4,096。")), 24, 30, 24, 30))
	var popup *motionPopup
	closingSettings := false
	closePopup := func() {
		if closingSettings {
			return
		}
		closingSettings = true
		key.SetText("")
		popup.Hide()
		if u.settingsPopup == popup {
			u.settingsThemeStatus = nil
		}
	}
	save := action("保存", nil, func() {
		if closingSettings {
			return
		}
		n, err := strconv.Atoi(tokens.Text)
		if err != nil || n < 4096 {
			u.error(errors.New("上下文容量至少为 4096"))
			return
		}
		if strings.TrimSpace(modelName.Text) == "" || strings.TrimSpace(base.Text) == "" {
			u.error(errors.New("请填写服务地址和模型名称"))
			return
		}
		p.Provider = "openai"
		if provider.Selected == "Ollama" {
			p.Provider = "ollama"
		}
		p.BaseURL = strings.TrimSpace(base.Text)
		p.Model = strings.TrimSpace(modelName.Text)
		p.ContextTokens = n
		secret := key.Text
		key.SetText("")
		closePopup()
		u.work("保存模型设置", func() error {
			if secret != "" {
				if err := (store.Credentials{}).Set("model:"+p.ID, secret); err != nil {
					return err
				}
			}
			if err := u.Store.Put("models", p.ID, p); err != nil {
				return err
			}
			fyne.Do(func() { u.refreshModelChoices() })
			return nil
		})
	})
	save.primary = true
	save.minWidth = 72
	done := action("完成", nil, closePopup)
	done.primary = true
	done.minWidth = 72
	cancelButton := outlineAction("取消", nil, closePopup)
	cancelButton.minWidth = 72
	settingsActions := container.NewCenter(container.NewHBox(cancelButton, save))
	note := metaText("模型配置保存在此设备上")
	footer := container.NewStack(container.NewBorder(nil, nil, note, settingsActions, layout.NewSpacer()))
	mode := u.UI.Preferences().StringWithFallback(themePreference, "system")
	var choices []*appearanceTile
	choose := func(value string) {
		setAppearanceMode(u.UI, value)
		for i, b := range choices {
			b.selected = []string{"light", "dark", "system"}[i] == value
			b.Refresh()
		}
		u.refreshAppearanceStatus()
	}
	cards := container.NewGridWithColumns(3)
	for i, title := range []string{"亮色", "暗色", "跟随系统"} {
		value := []string{"light", "dark", "system"}[i]
		tile := newAppearanceTile(title, value, func() { choose(value) })
		tile.selected = mode == value
		choices = append(choices, tile)
		cards.Add(tile)
	}
	u.settingsThemeStatus = metaText("")
	u.refreshAppearanceStatus()
	fontSize := widget.NewSelect([]string{"标准 · 14 px", "较大 · 15 px", "大 · 16 px"}, nil)
	scale := u.UI.Preferences().FloatWithFallback("appearance.textScale", 1)
	index := 0
	if scale > 1.1 {
		index = 2
	} else if scale > 1.02 {
		index = 1
	}
	fontSize.SetSelectedIndex(index)
	fontSize.OnChanged = func(v string) {
		size := map[string]float64{"标准 · 14 px": 1, "较大 · 15 px": 15.0 / 14, "大 · 16 px": 16.0 / 14}[v]
		u.UI.Preferences().SetFloat("appearance.textScale", size)
		restoreTheme(u.UI)
	}
	terminalSize := widget.NewSelect(terminalFontOptions(), nil)
	terminalSize.SetSelected(fmt.Sprintf("%.0f px", u.UI.Preferences().FloatWithFallback("terminal.size", float64(terminal.DefaultFontSize))))
	terminalSize.OnChanged = func(v string) { n, _ := strconv.Atoi(strings.Fields(v)[0]); u.setTerminalSize(float32(n)) }
	motionToggle := widget.NewCheck("启用窗口动效", func(enabled bool) { u.UI.Preferences().SetBool("appearance.motion", enabled) })
	motionToggle.SetChecked(u.UI.Preferences().BoolWithFallback("appearance.motion", true))
	appearancePane := container.NewVScroll(inset(container.NewVBox(headingText("应用外观"), metaText("选择你习惯的工作台配色。"), gap(14), cards, gap(10), motionToggle, gap(14), panel(padded(container.NewHBox(widget.NewIcon(designIcon("monitor")), u.settingsThemeStatus), 12), colorPanel, true, 5), gap(20), widget.NewSeparator(), gap(12), headingText("文字显示"), gap(10), container.NewBorder(nil, nil, container.NewVBox(bodyText("界面文字"), metaText("标题、正文和辅助信息同步缩放")), sized(fontSize, 155, 36)), gap(14), container.NewBorder(nil, nil, container.NewVBox(bodyText("终端文字"), metaText("使用等宽字体，保持输出对齐")), sized(terminalSize, 155, 36)), gap(20), widget.NewSeparator(), gap(12), u.backgroundSettings()), 24, 30, 24, 30))
	content := container.NewStack(modelPane)
	var appearanceButton, modelsButton *actionButton
	switchPage := func(name string) {
		if name == "appearance" {
			content.Objects = []fyne.CanvasObject{appearancePane}
			footer.Objects = []fyne.CanvasObject{container.NewBorder(nil, nil, metaText("外观调整即时生效"), container.NewCenter(done), layout.NewSpacer())}
		} else {
			content.Objects = []fyne.CanvasObject{modelPane}
			footer.Objects = []fyne.CanvasObject{container.NewBorder(nil, nil, note, settingsActions, layout.NewSpacer())}
		}
		appearanceButton.selected = name == "appearance"
		modelsButton.selected = name == "models"
		appearanceButton.Refresh()
		modelsButton.Refresh()
		content.Refresh()
		footer.Refresh()
	}
	appearanceButton = action("外观与显示", designIcon("sun"), func() { switchPage("appearance") })
	modelsButton = action("模型配置", designIcon("spark"), func() { switchPage("models") })
	appearanceButton.leading = true
	modelsButton.leading = true
	nav := sized(panel(inset(container.NewVBox(appearanceButton, modelsButton), 18, 10, 18, 10), colorPanel, false, 0), 170, 0)
	head := container.NewBorder(nil, nil, container.NewHBox(widget.NewIcon(designIcon("settings")), textUI("设置", sizeTitle, theme.ColorNameForeground, true), metaText("NexShell")), action("", designIcon("close"), closePopup))
	whole := edge(sized(panel(inset(head, 10, 24, 10, 24), theme.ColorNameBackground, false, 0), 0, 58), sized(panel(inset(footer, 10, 24, 10, 24), colorSoft, false, 0), 0, 62), nav, nil, content)
	popup = newMotionPopup(whole, u.Window.Canvas())
	u.settingsPopup = popup
	u.settingsPopup.Resize(fyne.NewSize(min(860, u.Window.Canvas().Size().Width-40), min(665, u.Window.Canvas().Size().Height-40)))
	switchPage(page)
	u.settingsPopup.Show()
}
func (u *App) refreshAppearanceStatus() {
	if u.settingsThemeStatus == nil {
		return
	}
	mode := u.UI.Preferences().StringWithFallback(themePreference, "system")
	name := "亮色"
	if mode == "dark" || (mode == "system" && u.UI.Settings().ThemeVariant() == theme.VariantDark) {
		name = "暗色"
	}
	prefix := "当前使用"
	if mode == "system" {
		prefix = "跟随系统，当前使用"
	}
	u.settingsThemeStatus.SetText(prefix + name + "外观。")
}
func (u *App) setTerminalSize(size float32) {
	if size < 8 || size > 40 {
		return
	}
	u.UI.Preferences().SetFloat("terminal.size", float64(size))
	for _, ws := range u.workspaces {
		for _, v := range ws.terminals {
			v.SetFontSize(size)
		}
	}
}
func themePreview(mode string) fyne.Resource {
	bg, bar, lines := "#fff", "#f5f5f7", "#d1d1d6"
	if mode == "dark" {
		bg, bar, lines = "#1c1c1e", "#2c2c2e", "#636366"
	}
	extra := ""
	if mode == "system" {
		extra = `<path d="M80 0h80v75H80z" fill="#1c1c1e"/>`
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="160" height="75"><rect width="160" height="75" fill="%s"/><rect width="160" height="12" fill="%s"/>%s<path d="M8 25h22M8 34h22M8 43h22" stroke="%s" stroke-width="4"/><rect x="45" y="22" width="106" height="44" rx="2" fill="#171e29"/></svg>`, bg, bar, extra, lines)
	return fyne.NewStaticResource("preview-"+mode+".svg", []byte(svg))
}

func terminalFontOptions() []string {
	values := []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 20, 24, 28, 32, 36, 40}
	out := make([]string, len(values))
	for i, n := range values {
		out[i] = fmt.Sprintf("%d px", n)
	}
	return out
}
