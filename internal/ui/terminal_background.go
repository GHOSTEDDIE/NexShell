package ui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/GHOSTEDDIE/nexshell/internal/terminal"
)

const backgroundFile = "terminal-background.image"

func decodeBackground(r io.Reader) (image.Image, []byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, 20*1024*1024+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > 20*1024*1024 {
		return nil, nil, fmt.Errorf("背景图片不能超过 20 MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("无法读取背景图片：%w", err)
	}
	if format != "png" && format != "jpeg" {
		return nil, nil, fmt.Errorf("请选择 PNG 或 JPEG 图片")
	}
	if int64(config.Width)*int64(config.Height) > 16000000 {
		return nil, nil, fmt.Errorf("背景图片不能超过 1600 万像素")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, data, err
}
func (u *App) restoreTerminalBackground() {
	if u.UI.Preferences().String("terminal.background.name") == "" {
		return
	}
	f, err := os.Open(filepath.Join(u.Store.Dir, backgroundFile))
	if err != nil {
		u.status.SetText("背景图片加载失败：" + err.Error())
		return
	}
	defer f.Close()
	img, _, err := decodeBackground(f)
	if err != nil {
		u.status.SetText(err.Error())
		return
	}
	u.terminalBackground = img
}
func (u *App) applyTerminalBackground(v *terminal.View) {
	v.SetBackground(u.terminalBackground, u.UI.Preferences().FloatWithFallback("terminal.background.opacity", .12))
}
func (u *App) refreshTerminalBackground() {
	for _, ws := range u.workspaces {
		for _, v := range ws.terminals {
			u.applyTerminalBackground(v)
		}
	}
}
func (u *App) backgroundSettings() fyne.CanvasObject {
	label := widget.NewLabel(u.UI.Preferences().StringWithFallback("terminal.background.name", "未设置背景图"))
	label.Truncation = fyne.TextTruncateEllipsis
	preview := canvas.NewImageFromImage(u.terminalBackground)
	preview.FillMode = canvas.ImageFillCover
	previewBox := sized(preview, 0, 100)
	if u.terminalBackground == nil {
		previewBox.Hide()
	}
	var choose, remove *widget.Button
	choose = widget.NewButton("选择图片", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				u.error(err)
				return
			}
			if r == nil {
				return
			}
			name := r.URI().Name()
			choose.Disable()
			remove.Disable()
			u.work("设置背景图", func() error {
				defer r.Close()
				defer fyne.Do(func() { choose.Enable(); remove.Enable() })
				img, data, err := decodeBackground(r)
				if err != nil {
					return err
				}
				target := filepath.Join(u.Store.Dir, backgroundFile)
				tmp, err := os.CreateTemp(u.Store.Dir, ".background-*")
				if err != nil {
					return err
				}
				defer os.Remove(tmp.Name())
				_, err = tmp.Write(data)
				closeErr := tmp.Close()
				if err != nil {
					return err
				}
				if closeErr != nil {
					return closeErr
				}
				if err = os.Rename(tmp.Name(), target); err != nil {
					return err
				}
				fyne.Do(func() {
					u.terminalBackground = img
					u.UI.Preferences().SetString("terminal.background.name", name)
					u.refreshTerminalBackground()
					label.SetText(name)
					preview.Image = img
					preview.Refresh()
					previewBox.Show()
				})
				return nil
			})
		}, u.Window)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg"}))
		d.Show()
	})
	remove = widget.NewButton("移除背景", func() {
		err := os.Remove(filepath.Join(u.Store.Dir, backgroundFile))
		if err != nil && !os.IsNotExist(err) {
			u.error(err)
			return
		}
		u.terminalBackground = nil
		u.UI.Preferences().RemoveValue("terminal.background.name")
		u.refreshTerminalBackground()
		label.SetText("未设置背景图")
		preview.Image = nil
		preview.Refresh()
		previewBox.Hide()
	})
	strength := widget.NewSlider(0, 15)
	strength.Step = 1
	strength.Value = u.UI.Preferences().FloatWithFallback("terminal.background.opacity", .12) * 100
	value := widget.NewLabel(fmt.Sprintf("%.0f%%", strength.Value))
	strength.OnChanged = func(n float64) {
		value.SetText(fmt.Sprintf("%.0f%%", n))
		u.UI.Preferences().SetFloat("terminal.background.opacity", n/100)
		u.refreshTerminalBackground()
	}
	return container.NewVBox(headingText("终端背景"), metaText("选择喜欢的图片，淡化显示以保持文字清晰。"), previewBox, label, container.NewHBox(choose, remove), container.NewBorder(nil, nil, bodyText("图片强度"), value, strength))
}
