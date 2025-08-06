package scan

import (
	"fmt"
	"os"

	"github.com/safedep/code/examples/astdb/scan"
	"github.com/safedep/dry/log"
	"github.com/spf13/cobra"
)

var (
	inputDir           string
	outputDatabasePath string
)

func NewScanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a directory for source code to create AST database",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeScan()
			if err != nil {
				log.Errorf("scan failed: %v", err)

				// If we return err here then cobra just prints usage which misleading
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputDir, "dir", "D", "", "Input directory to scan")
	cmd.Flags().StringVarP(&outputDatabasePath, "output", "o", "", "Output database path")

	_ = cmd.MarkFlagRequired("dir")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func executeScan() error {
	scanner, err := scan.New(scan.Config{
		InputDirectory:     inputDir,
		OutputDatabasePath: outputDatabasePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}

	err = scanner.Run()
	if err != nil {
		return fmt.Errorf("failed to run scanner: %w", err)
	}

	return nil
}
