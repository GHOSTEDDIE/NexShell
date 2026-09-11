// Native file-dialog and drop-coordinate smoke harness; selected files are never modified.
package main

import (
	"context"
	"encoding/json"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/nativefiles"
	"github.com/GHOSTEDDIE/nexshell/internal/platforminput"
	"os"
	"time"
)

func main() {
	a := app.NewWithID("io.nexshell.filecheck")
	w := a.NewWindow("NexShell 文件交互验证")
	label := widget.NewLabel("将测试文件拖到此窗口下半部")
	log := func(value any) {
		f, err := os.OpenFile("/tmp/nexshell-native-results.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		json.NewEncoder(f).Encode(value)
	}
	choose := func(mode nativefiles.Mode, timed bool) {
		request := nativefiles.Request{Mode: mode, Title: "NexShell 文件选择验证", Filename: "/tmp/nexshell-native-input/"}
		if mode == nativefiles.Save {
			request.Filename = "/tmp/nexshell-native-input/保存测试.txt"
		}
		if native, ok := w.(driver.NativeWindow); ok {
			native.RunNative(func(c any) {
				switch c := c.(type) {
				case driver.MacWindowContext:
					request.Parent = c.NSWindow
				case driver.WindowsWindowContext:
					request.Parent = c.HWND
				}
			})
		}
		go func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if timed {
				ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
			}
			paths, err := nativefiles.Choose(ctx, request)
			message := ""
			if err != nil {
				message = err.Error()
			}
			log(map[string]any{"mode": mode, "paths": paths, "error": message, "timed": timed})
			fyne.Do(func() { label.SetText("选择完成：" + message) })
		}()
	}
	w.SetContent(container.NewBorder(container.NewHBox(widget.NewButton("多选文件", func() { choose(nativefiles.OpenMultiple, false) }), widget.NewButton("保存文件", func() { choose(nativefiles.Save, false) }), widget.NewButton("选择目录", func() { choose(nativefiles.Directory, false) }), widget.NewButton("取消测试", func() { choose(nativefiles.Open, false) }), widget.NewButton("超时测试", func() { choose(nativefiles.Open, true) })), nil, nil, nil, label))
	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		var handle uintptr
		w.(driver.NativeWindow).RunNative(func(c any) {
			switch c := c.(type) {
			case driver.MacWindowContext:
				handle = c.NSWindow
			case driver.WindowsWindowContext:
				handle = c.HWND
			}
		})
		x, y, ok := platforminput.DropPosition(handle)
		var paths []string
		for _, uri := range uris {
			paths = append(paths, uri.Path())
		}
		log(map[string]any{"drop": paths, "x": x, "y": y, "ok": ok, "fyneX": pos.X, "fyneY": pos.Y})
		fyne.Do(func() { label.SetText("已接收拖拽文件") })
	})
	w.Resize(fyne.NewSize(820, 420))
	w.ShowAndRun()
}
