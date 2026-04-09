package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

type terminalPolicy struct {
	LiveUpdates bool
	Color       bool
}

var (
	resolveTerminalPolicy = detectTerminalPolicy
	lookupEnv             = os.LookupEnv
	terminalWriterCheck   = isTerminalWriter
)

func detectTerminalPolicy(out io.Writer) terminalPolicy {
	isTTY := terminalWriterCheck(out)
	_, noColor := lookupEnv("NO_COLOR")
	return terminalPolicy{
		LiveUpdates: isTTY,
		Color:       isTTY && !noColor,
	}
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}
