package ui

import (
	"context"
	"errors"
	"fyne.io/fyne/v2/test"
	"github.com/GHOSTEDDIE/nexshell/internal/nativefiles"
	"reflect"
	"testing"
	"time"
)

func TestPickerCompletionAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		paths    []string
		err      error
		canceled bool
	}{
		{"multiple", []string{"/tmp/中文 空格 100%.txt", "/tmp/second.txt"}, nil, false},
		{"dismiss", nil, nativefiles.ErrCanceled, false},
		{"failure", nil, errors.New("native failure"), false},
		{"expired", []string{"/tmp/must-not-upload"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := test.NewApp()
			defer a.Quit()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			u := &App{Window: a.NewWindow("picker")}
			defer u.Window.Close()
			u.localFilePicker = func(context.Context, nativefiles.Request) ([]string, error) {
				if tc.canceled {
					cancel()
				}
				return tc.paths, tc.err
			}
			done := make(chan struct{})
			u.pickFiles(ctx, nativefiles.Request{Mode: nativefiles.OpenMultiple}, func(paths []string, err error) {
				defer close(done)
				if tc.canceled {
					if !errors.Is(err, context.Canceled) || len(paths) != 0 {
						t.Errorf("expired picker returned usable files: %v %v", paths, err)
					}
					return
				}
				wantErr := tc.err
				if errors.Is(wantErr, nativefiles.ErrCanceled) {
					wantErr = nil
				}
				if !errors.Is(err, wantErr) || !reflect.DeepEqual(paths, tc.paths) {
					t.Errorf("result: %v %v", paths, err)
				}
			})
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("picker completion stuck")
			}
			if u.filePickerOpen {
				t.Fatal("picker remained locked")
			}
		})
	}
}

func TestPickerPreventsDuplicateDialogs(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	u := &App{Window: a.NewWindow("picker")}
	defer u.Window.Close()
	release := make(chan struct{})
	done := make(chan struct{})
	u.localFilePicker = func(context.Context, nativefiles.Request) ([]string, error) {
		<-release
		return nil, nativefiles.ErrCanceled
	}
	u.pickFiles(context.Background(), nativefiles.Request{}, func([]string, error) { close(done) })
	blocked := false
	u.pickFiles(context.Background(), nativefiles.Request{}, func(_ []string, err error) { blocked = err != nil })
	close(release)
	<-done
	if !blocked {
		t.Fatal("second native dialog was allowed")
	}
}
