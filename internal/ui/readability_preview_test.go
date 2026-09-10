package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadabilityPreview(t *testing.T) {
	dir := os.Getenv("NEXSHELL_VISUAL_EVIDENCE")
	if dir == "" {
		t.Skip("optional visual capture")
	}
	a := test.NewApp()
	defer a.Quit()
	restoreTheme(a)
	w := a.NewWindow("NexShell")
	defer w.Close()
	v := terminal.NewView(io.Discard, strings.NewReader(""))
	v.Close()
	v.Core.Write([]byte("root@app-01:~# ls -la\r\n\x1b[34mprojects/  .local/  .ssh/\x1b[0m\r\n\x1b[31marchive.tar.gz  backup.zip\x1b[0m\r\n\x1b[32m服务运行正常\x1b[0m\r\n\r\nroot@app-01:~# "))
	img := image.NewNRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / 640), uint8(y * 255 / 480), 200, 255})
		}
	}
	v.SetBackground(img, .12)
	chat := newConversationView()
	chat.update("preview", []chatMessage{{"assistant", "## 检查结果\n\n**服务正常**，已根据最新只读证据重新验证。\n\n- CPU 使用率：12%\n- 内存使用率：38%\n\n```sh\nfree -m\nsystemctl status nginx\n```\n\n| 项目 | 状态 |\n| --- | --- |\n| Nginx | 运行中 |\n| 磁盘 | 空间充足 |"}, {"execution", "验证证据不匹配，已读取新证据后重新验证。"}})
	split := container.NewHSplit(v, chat.scroll)
	split.Offset = .6
	w.SetContent(split)
	w.Resize(fyne.NewSize(1200, 720))
	w.Show()
	v.Refresh()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "terminal-markdown.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
	u := &App{UI: a, Window: w, terminalBackground: img}
	a.Preferences().SetString("terminal.background.name", "自定义背景.png")
	w.SetContent(container.NewVScroll(inset(u.backgroundSettings(), 24, 24, 24, 24)))
	w.Resize(fyne.NewSize(660, 540))
	settings, err := os.Create(filepath.Join(dir, "background-settings.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer settings.Close()
	if err = png.Encode(settings, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
