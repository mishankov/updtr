package cli

import (
	"io"

	"github.com/mishankov/updtr/internal/presenter"
)

type runPresenter = presenter.RunPresenter

func newRunPresenter(out io.Writer) runPresenter {
	policy := resolveTerminalPolicy(out)
	if policy.LiveUpdates {
		return presenter.NewLive(out, policy.Color)
	}
	return presenter.NewAppendOnly(out, policy.Color)
}
