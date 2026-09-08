# Local changes

Source: https://github.com/charmbracelet/x/tree/3986e9119cf98efcf5809969e11ad369fddb5522/vt

The original MIT license and upstream tests are retained. `utf8.go` extends the previous cell when a grapheme is split across transport writes. The pinned upstream flushed its grapheme buffer after each `Write`, so `Write("e")` followed by `Write("\u0301")` left a separate zero-width cell. Regression cases in the application cover chunked UTF-8, combining marks, scrollback and alternate screens. Run upstream tests as well when changing this patch.
