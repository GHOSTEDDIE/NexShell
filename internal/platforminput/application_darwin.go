//go:build darwin && !ci

package platforminput

/*
#cgo LDFLAGS: -framework Cocoa
int nexInstallApplicationActions(void);
*/
import "C"

import (
	"fmt"
	"fyne.io/fyne/v2"
	"sync"
)

var applicationActions struct {
	sync.RWMutex
	reopen, quit func()
}

func InstallApplicationActions(reopen, quit func()) error {
	applicationActions.Lock()
	applicationActions.reopen = reopen
	applicationActions.quit = quit
	applicationActions.Unlock()
	if C.nexInstallApplicationActions() == 0 {
		return fmt.Errorf("无法配置 macOS 窗口恢复与退出操作")
	}
	return nil
}

//export nexApplicationAction
func nexApplicationAction(terminate C.int) {
	applicationActions.RLock()
	action := applicationActions.reopen
	if terminate != 0 {
		action = applicationActions.quit
	}
	applicationActions.RUnlock()
	if action != nil {
		fyne.Do(action)
	}
}
