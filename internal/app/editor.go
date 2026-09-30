package app

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// editorDoneMsg arrives when $EDITOR exits; policies are reloaded then.
type editorDoneMsg struct{ err error }

// editorCommand opens file at line in $VISUAL / $EDITOR (vi if unset,
// notepad on Windows). The editor may have arguments ("code -w").
func editorCommand(visual, editor, file string, line int) (*exec.Cmd, error) {
	value := visual
	if value == "" {
		value = editor
	}
	if value == "" {
		value = "vi"
		if runtime.GOOS == "windows" {
			value = "notepad"
		}
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return nil, errors.New("$EDITOR is empty")
	}
	name := strings.TrimSuffix(filepath.Base(parts[0]), ".exe")
	args := parts[1:]
	switch name {
	case "code", "code-insiders", "codium", "cursor":
		args = append(args, "-g", file+":"+strconv.Itoa(line))
	case "subl", "zed", "hx", "helix":
		args = append(args, file+":"+strconv.Itoa(line))
	case "notepad":
		args = append(args, file)
	default: // vi, vim, nvim, nano, emacs, micro, kak... all take +N
		args = append(args, "+"+strconv.Itoa(line), file)
	}
	return exec.Command(parts[0], args...), nil
}
