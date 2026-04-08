package cli

import (
	"fmt"
	"io"

	"github.com/mishankov/updtr/internal/orchestrator"
)

type progressWriter struct {
	out io.Writer
}

func newProgressWriter(out io.Writer) orchestrator.ProgressReporter {
	return progressWriter{out: out}
}

func (w progressWriter) TargetStarted(event orchestrator.TargetProgress) {
	_, _ = fmt.Fprintf(w.out, "%s: target %s (%s) started\n", event.Mode, event.Target.Name, event.Target.NormalizedPath)
}

func (w progressWriter) TargetFinished(event orchestrator.TargetProgress) {
	_, _ = fmt.Fprintf(w.out, "%s: target %s (%s) finished outcome=%s elapsed=%s\n", event.Mode, event.Target.Name, event.Target.NormalizedPath, event.Outcome, event.Elapsed)
}
