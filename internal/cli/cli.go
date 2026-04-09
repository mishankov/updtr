package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/core"
	"github.com/mishankov/updtr/internal/initgen"
	"github.com/mishankov/updtr/internal/orchestrator"
	"github.com/spf13/cobra"
)

type silentExit struct{}

func (silentExit) Error() string { return "" }

func IsSilentExit(err error) bool {
	_, ok := err.(silentExit)
	return ok
}

var newEngine = orchestrator.New

func New(version string, out io.Writer, errOut io.Writer) *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:           "updtr",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&configPath, "config", "updtr.yaml", "path to YAML config file")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the updtr version",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(out, "updtr %s\n", version)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create updtr.yaml from discovered Go modules",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			message, err := initgen.Run(cwd)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(out, message)
			return nil
		},
	})

	root.AddCommand(runCommand("detect", "Detect dependency updates", out, func(engine *orchestrator.Engine, cfg *config.Config, targets []string) (core.RunResult, error) {
		result, err := engine.Detect(root.Context(), cfg, targets)
		return result, err
	}, &configPath))

	root.AddCommand(runCommand("apply", "Apply eligible dependency updates", out, func(engine *orchestrator.Engine, cfg *config.Config, targets []string) (core.RunResult, error) {
		result, err := engine.Apply(root.Context(), cfg, targets)
		return result, err
	}, &configPath))

	return root
}

func runCommand(use string, short string, out io.Writer, run func(*orchestrator.Engine, *config.Config, []string) (core.RunResult, error), configPath *string) *cobra.Command {
	var targets []string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(loadPath(cmd, *configPath))
			if err != nil {
				return err
			}
			engine := newEngine()
			presenter := newRunPresenter(out)
			engine.Reporter = presenter
			result, err := run(engine, cfg, targets)
			if err != nil {
				return err
			}
			presenter.Render(result)
			if result.HasOperationalFailures() {
				return silentExit{}
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&targets, "target", nil, "target name to run; repeatable")
	return cmd
}

func loadPath(cmd *cobra.Command, configured string) string {
	if flag := cmd.Flag("config"); flag != nil && flag.Changed {
		return configured
	}
	return ""
}
