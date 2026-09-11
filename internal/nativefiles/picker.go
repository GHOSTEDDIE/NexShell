// Package nativefiles selects local paths using operating-system dialogs.
// Selection never opens or truncates the selected file.
package nativefiles

import (
	"context"
	"github.com/ncruces/zenity"
)

type Mode int

const (
	Open Mode = iota
	OpenMultiple
	Save
	Directory
)

type Request struct {
	Mode            Mode
	Title, Filename string
	Patterns        []string
	Parent          any
}

var ErrCanceled = zenity.ErrCanceled

func chooseZenity(ctx context.Context, r Request) ([]string, error) {
	options := []zenity.Option{zenity.Context(ctx), zenity.Title(r.Title), zenity.Filename(r.Filename)}
	if r.Parent != nil {
		options = append(options, zenity.Attach(r.Parent), zenity.Modal())
	}
	if len(r.Patterns) > 0 {
		options = append(options, zenity.FileFilter{Name: r.Title, Patterns: r.Patterns, CaseFold: true})
	}
	if r.Mode == OpenMultiple {
		return zenity.SelectFileMultiple(options...)
	}
	var p string
	var err error
	switch r.Mode {
	case Save:
		p, err = zenity.SelectFileSave(append(options, zenity.ConfirmOverwrite())...)
	case Directory:
		p, err = zenity.SelectFile(append(options, zenity.Directory())...)
	default:
		p, err = zenity.SelectFile(options...)
	}
	if err != nil || p == "" {
		return nil, err
	}
	return []string{p}, nil
}
