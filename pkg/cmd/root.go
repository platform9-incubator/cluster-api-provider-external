package cmd

import (
	"context"
	"os"

	"github.com/erwinvaneyk/cobras"
	"github.com/erwinvaneyk/goversion/pkg/extensions"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type RootOptions struct {
	Debug bool
}

func NewCmdRoot() *cobra.Command {
	opts := &RootOptions{
		Debug: os.Getenv("PF9_DEBUG") != "",
	}

	cmd := &cobra.Command{
		Use:              "cape",
		Short:            "CLI for running and interacting with external clusters in CAPI",
		PersistentPreRun: cobras.Run(opts),
	}

	cmd.PersistentFlags().BoolVar(&opts.Debug, "debug", opts.Debug, "More logs. [PF9_DEBUG]")

	cmd.AddCommand(NewCmdImport(opts))
	cmd.AddCommand(NewCmdRun(opts))
	cmd.AddCommand(extensions.NewCobraCmdWithDefaults())

	return cmd
}

func (o *RootOptions) Complete(cmd *cobra.Command, args []string) error {
	return nil
}

func (o *RootOptions) Validate() error {
	return nil
}

func (o *RootOptions) Run(ctx context.Context) error {
	// Configure the logging and its verbosity
	setupLogging(o.Debug)

	return nil
}

func Execute() {
	if err := NewCmdRoot().Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging(debug bool) {
	zapCfg := zap.NewDevelopmentConfig()
	if !debug {
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logger, err := zapCfg.Build()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}
