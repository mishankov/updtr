package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mishankov/updtr/internal/config"
	"github.com/mishankov/updtr/internal/initgen"
	"github.com/mishankov/updtr/internal/orchestrator"
	"github.com/mishankov/updtr/internal/render"
	"github.com/spf13/cobra"
)

type silentExit struct{}

func (silentExit) Error() string { return "" }

func IsSilentExit(err error) bool {
	_, ok := err.(silentExit)
	return ok
}

func New(version string, out io.Writer, errOut io.Writer) *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:           "updtr",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&configPath, "config", "updtr.toml", "path to config file")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the updtr version",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(out, "updtr %s\n", version)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create updtr.toml from discovered Go modules",
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

	root.AddCommand(runCommand("detect", "Detect dependency updates", out, func(engine *orchestrator.Engine, cfg *config.Config, targets []string) (bool, error) {
		result, err := engine.Detect(root.Context(), cfg, targets)
		if err != nil {
			return false, err
		}
		render.Detect(out, result)
		return result.HasOperationalFailures(), nil
	}, &configPath))

	root.AddCommand(runCommand("apply", "Apply eligible dependency updates", out, func(engine *orchestrator.Engine, cfg *config.Config, targets []string) (bool, error) {
		result, err := engine.Apply(root.Context(), cfg, targets)
		if err != nil {
			return false, err
		}
		render.Apply(out, result)
		return result.HasOperationalFailures(), nil
	}, &configPath))

	return root
}

func runCommand(use string, short string, out io.Writer, run func(*orchestrator.Engine, *config.Config, []string) (bool, error), configPath *string) *cobra.Command {
	var targets []string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configPath)
			if err != nil {
				return err
			}
			hasFailures, err := run(orchestrator.New(), cfg, targets)
			if err != nil {
				return err
			}
			if hasFailures {
				return silentExit{}
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&targets, "target", nil, "target name to run; repeatable")
	return cmd
}
