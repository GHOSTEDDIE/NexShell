package ui

import (
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

var designIcons = map[string]fyne.Resource{
	"server":     newLineIcon(fyne.NewStaticResource("server.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M3 3h18v7H3zM3 14h18v7H3zM6 6h1M6 17h1"/></svg>`))),
	"terminal":   newLineIcon(fyne.NewStaticResource("terminal.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m5 5 5 6-5 6m8 0h7"/></svg>`))),
	"chevron":    newLineIcon(fyne.NewStaticResource("chevron.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m7 9 5 5 5-5"/></svg>`))),
	"search":     newLineIcon(fyne.NewStaticResource("search.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" cx="10" cy="10" r="7"/><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m15 15 6 6"/></svg>`))),
	"plus":       newLineIcon(fyne.NewStaticResource("plus.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M12 4v16M4 12h16"/></svg>`))),
	"folder":     newLineIcon(fyne.NewStaticResource("folder.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M2 5h7l3 3h10v13H2Z"/></svg>`))),
	"file":       newLineIcon(fyne.NewStaticResource("file.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M5 2h9l5 5v15H5ZM14 2v6h5"/></svg>`))),
	"spark":      newLineIcon(fyne.NewStaticResource("spark.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m12 2 3 7 7 3-7 3-3 7-3-7-7-3 7-3Z"/></svg>`))),
	"split":      newLineIcon(fyne.NewStaticResource("split.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M3 4h18v16H3ZM12 4v16"/></svg>`))),
	"more":       newLineIcon(fyne.NewStaticResource("more.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" cx="4" cy="12" r="1"/><circle fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" cx="12" cy="12" r="1"/><circle fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" cx="20" cy="12" r="1"/></svg>`))),
	"refresh":    newLineIcon(fyne.NewStaticResource("refresh.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M20 7v5h-5M4 17v-5h5M6.1 7a7 7 0 0 1 11.6-2L20 8M4 16l2.3 3A7 7 0 0 0 17.9 17"/></svg>`))),
	"upload":     newLineIcon(fyne.NewStaticResource("upload.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M3 16v6h18v-6M12 16V2m-6 6 6-6 6 6"/></svg>`))),
	"send":       newLineIcon(fyne.NewStaticResource("send.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M12 21V3m-8 8 8-8 8 8"/></svg>`))),
	"close":      newLineIcon(fyne.NewStaticResource("close.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m6 6 12 12M6 18 18 6"/></svg>`))),
	"sun":        newLineIcon(fyne.NewStaticResource("sun.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" cx="12" cy="12" r="4"/><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5"/></svg>`))),
	"moon":       newLineIcon(fyne.NewStaticResource("moon.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M20.5 14A9 9 0 0 1 10 3.5 9 9 0 1 0 20.5 14Z"/></svg>`))),
	"monitor":    newLineIcon(fyne.NewStaticResource("monitor.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><rect fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" x="2" y="3" width="20" height="14" rx="2"/><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M8 21h8m-4-4v4"/></svg>`))),
	"settings":   newLineIcon(fyne.NewStaticResource("settings.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M19.65 9.66 L21.85 10.26 L21.85 13.74 L19.65 14.34 L19.06 15.76 L20.19 17.74 L17.74 20.19 L15.76 19.06 L14.34 19.65 L13.74 21.85 L10.26 21.85 L9.66 19.65 L8.24 19.06 L6.26 20.19 L3.81 17.74 L4.94 15.76 L4.35 14.34 L2.15 13.74 L2.15 10.26 L4.35 9.66 L4.94 8.24 L3.81 6.26 L6.26 3.81 L8.24 4.94 L9.66 4.35 L10.26 2.15 L13.74 2.15 L14.34 4.35 L15.76 4.94 L17.74 3.81 L20.19 6.26 L19.06 8.24 Z"/><circle fill="none" stroke="currentColor" stroke-width="2" cx="12" cy="12" r="3"/></svg>`))),
	"arrow-left": newLineIcon(fyne.NewStaticResource("arrow-left.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m12 5-7 7 7 7M5 12h14"/></svg>`))),
	"expand":     newLineIcon(fyne.NewStaticResource("expand.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="M15 3h6v6m0-6-7 7M9 21H3v-6m0 6 7-7"/></svg>`))),
	"home":       newLineIcon(fyne.NewStaticResource("home.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m3 10 9-7 9 7v10H3zM9 20v-7h6v7"/></svg>`))),
}

func designIcon(name string) fyne.Resource { return designIcons[name] }

// Fyne's fill recolouring does not recolour SVG strokes. Resolve the stroke
// explicitly and include its colour in the cache key for OS theme changes.
type lineIcon struct {
	source fyne.Resource
	tone   fyne.ThemeColorName
}

func newLineIcon(r fyne.Resource) *lineIcon { return &lineIcon{source: r, tone: colorMuted} }
func (r *lineIcon) colorHex() string {
	c := theme.Color(r.tone)
	red, green, blue, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", red>>8, green>>8, blue>>8)
}
func (r *lineIcon) Name() string { return r.source.Name() + r.colorHex() }
func (r *lineIcon) Content() []byte {
	return bytes.ReplaceAll(r.source.Content(), []byte("currentColor"), []byte(r.colorHex()))
}
