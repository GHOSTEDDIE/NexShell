# Local macOS input-method fixes

Source: github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1.0.20260707082822-2a407d02d01a. Original sources and licenses are retained.

Only glfw/src/cocoa_window.m is modified:

- Resolve Cocoa text composition before dispatching physical keys. Keys that start, edit or commit marked text are consumed by the input method.
- Return the complete marked UTF-16 range and selected range.
- Display pending marked text in a native overlay; clear it after commit/cancellation.
- Convert the focused widget's caret rectangle to screen coordinates for candidate positioning. NexShell supplies the rectangle via Fyne NativeWindow and CursorPosition.

Tests in tests/nativeime exercise an isolated hidden window on the AppKit main thread. They check the native protocol, committed Chinese text and composing versus ordinary Return, without posting events to the OS or the user's window.
