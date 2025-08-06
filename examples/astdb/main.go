package main

import (
	"fmt"
	"os"

	"github.com/safedep/code/examples/astdb/cmd/scan"
	"github.com/safedep/dry/log"
	"github.com/safedep/dry/obs"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:              "astdb [OPTIONS] COMMAND [ARG...]",
		Short:            "SafeDep Code Analysis AST Database CLI",
		TraverseChildren: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			return fmt.Errorf("astdb: %s is not a valid command", args[0])
		},
	}

	cmd.AddCommand(scan.NewScanCommand())

	cobra.OnInitialize(func() {
		log.InitZapLogger(obs.AppServiceName("astdb"), obs.AppServiceEnv("dev"))
	})

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
