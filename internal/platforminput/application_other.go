//go:build !darwin || ci

package platforminput

func InstallApplicationActions(reopen, quit func()) error { return nil }
