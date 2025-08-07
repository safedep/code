package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/safedep/code/examples/astdb/scan"
	"github.com/safedep/dry/log"
	"github.com/spf13/cobra"
)

var (
	// Required flags
	inputDir           string
	outputDatabasePath string

	// Optional flags
	projectName     string
	languageFilters []string
	maxDepth        int
	includePatterns []string
	excludePatterns []string
	gitHash         string

	// Performance options
	maxWorkers int
	batchSize  int

	// Database options
	skipSchemaCreate  bool
	enableForeignKeys bool
	enableAutoMigrate bool

	// Reporting options
	showProgress bool
	verbose      bool
	outputFormat string

	// AST extraction options
	persistASTNodes bool
)

func NewScanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a directory for source code to create AST database",
		Long: `Scan a directory recursively for source code files and create a comprehensive AST database.

The scanner will analyze source files in supported languages (Go, Python, Java, JavaScript, TypeScript)
and extract symbols, classes, functions, inheritance relationships, and call graphs.

Examples:
  # Basic scan
  astdb scan -D ./src -o ./analysis.db

  # Scan with language filters
  astdb scan -D ./src -o ./analysis.db --languages python,java

  # Scan with custom project name and git hash
  astdb scan -D ./src -o ./analysis.db --project myproject --git-hash abc123

  # Scan with progress reporting and verbose output
  astdb scan -D ./src -o ./analysis.db --progress --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate inputs
			if err := validateFlags(); err != nil {
				return fmt.Errorf("invalid flags: %w", err)
			}

			err := executeScan()
			if err != nil {
				log.Errorf("scan failed: %v", err)
				os.Exit(1)
			}

			return nil
		},
	}

	// Required flags
	cmd.Flags().StringVarP(&inputDir, "dir", "D", "", "Input directory to scan (required)")
	cmd.Flags().StringVarP(&outputDatabasePath, "output", "o", "", "Output database path (required)")

	// Project configuration
	cmd.Flags().StringVar(&projectName, "project", "", "Project name (defaults to directory name)")
	cmd.Flags().StringVar(&gitHash, "git-hash", "", "Git commit hash for this scan")

	// Language and file filtering
	cmd.Flags().StringSliceVar(&languageFilters, "languages", []string{},
		"Comma-separated list of languages to scan (go,python,java,javascript,typescript)")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 0, "Maximum directory depth to scan (0 = unlimited)")
	cmd.Flags().StringSliceVar(&includePatterns, "include", []string{},
		"Include file patterns (glob syntax)")
	cmd.Flags().StringSliceVar(&excludePatterns, "exclude",
		[]string{"node_modules", ".git", "vendor", "__pycache__"},
		"Exclude file patterns (glob syntax)")

	// Performance options
	cmd.Flags().IntVar(&maxWorkers, "workers", 4, "Maximum number of worker goroutines")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Batch size for database operations")

	// Database options
	cmd.Flags().BoolVar(&skipSchemaCreate, "skip-schema", false, "Skip database schema creation")
	cmd.Flags().BoolVar(&enableForeignKeys, "foreign-keys", true, "Enable foreign key constraints")
	cmd.Flags().BoolVar(&enableAutoMigrate, "auto-migrate", true, "Enable automatic schema migration")

	// Reporting options
	cmd.Flags().BoolVar(&showProgress, "progress", false, "Show progress during scanning")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	cmd.Flags().StringVar(&outputFormat, "format", scan.OutputFormatText, fmt.Sprintf("Output format (%s, %s)", scan.OutputFormatText, scan.OutputFormatJSON))

	// AST extraction options
	cmd.Flags().BoolVar(&persistASTNodes, "persist-ast-nodes", false, "Persist all AST nodes to database (disabled by default)")

	// Mark required flags
	_ = cmd.MarkFlagRequired("dir")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}

func validateFlags() error {
	// Check if input directory exists
	if info, err := os.Stat(inputDir); err != nil {
		return fmt.Errorf("input directory does not exist: %s", inputDir)
	} else if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}

	// Validate language filters
	supportedLanguages := scan.GetSupportedLanguages()
	for _, lang := range languageFilters {
		if !supportedLanguages[strings.ToLower(lang)] {
			return fmt.Errorf("unsupported language: %s", lang)
		}
	}

	// Validate output format
	if outputFormat != scan.OutputFormatText && outputFormat != scan.OutputFormatJSON {
		return fmt.Errorf("unsupported output format: %s (supported: %s, %s)",
			outputFormat, scan.OutputFormatText, scan.OutputFormatJSON)
	}

	// Validate performance settings
	if maxWorkers < 1 {
		return fmt.Errorf("max-workers must be at least 1")
	}

	if batchSize < 1 {
		return fmt.Errorf("batch-size must be at least 1")
	}

	return nil
}

func executeScan() error {
	// Determine project name
	finalProjectName := projectName
	if finalProjectName == "" {
		finalProjectName = filepath.Base(inputDir)
	}

	config := scan.Config{
		InputDirectory:     inputDir,
		OutputDatabasePath: outputDatabasePath,
		ProjectName:        finalProjectName,
		GitHash:            gitHash,
		LanguageFilters:    languageFilters,
		MaxDepth:           maxDepth,
		IncludePatterns:    includePatterns,
		ExcludePatterns:    excludePatterns,
		MaxWorkers:         maxWorkers,
		BatchSize:          batchSize,
		SkipSchemaCreate:   skipSchemaCreate,
		EnableForeignKeys:  enableForeignKeys,
		EnableAutoMigrate:  enableAutoMigrate,
		ShowProgress:       showProgress,
		Verbose:            verbose,
		OutputFormat:       outputFormat,
		PersistASTNodes:    persistASTNodes,
	}

	scanner, err := scan.New(config)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}

	err = scanner.Run()
	if err != nil {
		return fmt.Errorf("failed to run scanner: %w", err)
	}

	return nil
}
