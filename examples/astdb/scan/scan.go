package scan

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/safedep/code/core"
	"github.com/safedep/code/examples/astdb/ent"
	fileent "github.com/safedep/code/examples/astdb/ent/file"
	"github.com/safedep/code/examples/astdb/storage"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
)

type Config struct {
	// Input/Output
	InputDirectory     string
	OutputDatabasePath string
	ProjectName        string
	GitHash            string

	// File filtering
	LanguageFilters []string
	MaxDepth        int
	IncludePatterns []string
	ExcludePatterns []string

	// Performance
	MaxWorkers int
	BatchSize  int

	// Database options
	SkipSchemaCreate  bool
	EnableForeignKeys bool
	EnableAutoMigrate bool

	// Reporting
	ShowProgress bool
	Verbose      bool
	OutputFormat string

	// AST extraction options
	PersistASTNodes bool
}

type scanner struct {
	config       Config
	fileSystem   core.ImportAwareFileSystem
	languages    []core.Language
	treeWalker   core.TreeWalker
	projectID    int
	progressChan chan string
}

func New(config Config) (*scanner, error) {
	// Create local file system
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{config.InputDirectory},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create local filesystem: %w", err)
	}

	// Get languages based on filters
	var languages []core.Language
	if len(config.LanguageFilters) == 0 {
		languages, err = lang.AllLanguages()
		if err != nil {
			return nil, fmt.Errorf("failed to get all languages: %w", err)
		}
	} else {
		for _, langName := range config.LanguageFilters {
			language, err := lang.GetLanguage(langName)
			if err != nil {
				return nil, fmt.Errorf("failed to get language %s: %w", langName, err)
			}

			languages = append(languages, language)
		}
	}

	// Create source walker (config fields are limited in CAF)
	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, languages)
	if err != nil {
		return nil, fmt.Errorf("failed to create source walker: %w", err)
	}

	// Create tree walker
	treeWalker, err := parser.NewWalkingParser(walker, languages)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree walker: %w", err)
	}

	progressChan := make(chan string, 100)
	return &scanner{
		config:       config,
		fileSystem:   fileSystem,
		languages:    languages,
		treeWalker:   treeWalker,
		progressChan: progressChan,
	}, nil
}

func (s *scanner) Run() error {
	// Configure storage based on CLI options
	storageConfig := storage.DefaultEntSqliteConfig(s.config.OutputDatabasePath)
	storageConfig.SkipSchemaCreation = s.config.SkipSchemaCreate
	storageConfig.EnableForeignKeys = s.config.EnableForeignKeys
	storageConfig.EnableAutoMigration = s.config.EnableAutoMigrate

	// Adjust connection pool settings based on worker count
	if s.config.MaxWorkers > 0 {
		storageConfig.MaxOpenConns = s.config.MaxWorkers * 2
		storageConfig.MaxIdleConns = s.config.MaxWorkers
	}

	storageClient, err := storage.NewEntSqliteStorage(storageConfig)
	if err != nil {
		return fmt.Errorf("failed to create database adapter: %w", err)
	}

	defer storageClient.Close()

	db, err := storageClient.Client()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	if s.config.ShowProgress {
		fmt.Printf("Starting scan of directory: %s\n", s.config.InputDirectory)
		fmt.Printf("Output database: %s\n", s.config.OutputDatabasePath)
		if len(s.config.LanguageFilters) > 0 {
			fmt.Printf("Language filters: %v\n", s.config.LanguageFilters)
		}
	}

	err = s.internalStartScan(db)
	if err != nil {
		return fmt.Errorf("failed to start scan: %w", err)
	}

	if s.config.ShowProgress {
		fmt.Println("Scan completed successfully!")
	}

	return nil
}

func (s *scanner) internalStartScan(db *ent.Client) error {
	ctx := context.Background()

	// Create project record
	project, err := s.createProject(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	s.projectID = project.ID

	if s.config.ShowProgress {
		fmt.Printf("Created project record: ID=%d, Name=%s\n", project.ID, project.Name)
	}

	// Create symbol registry for cross-file resolution
	symbolRegistry := NewSymbolRegistry(db, ctx, project.ID)
	err = symbolRegistry.LoadExistingSymbols()
	if err != nil {
		if s.config.Verbose {
			fmt.Printf("Warning: failed to load existing symbols: %v\n", err)
		}
	}

	// Create visitor for processing files
	visitor := &fileProcessor{
		scanner:        s,
		db:             db,
		ctx:            ctx,
		fileCount:      0,
		symbolRegistry: symbolRegistry,
	}

	// Start progress reporting if enabled
	if s.config.ShowProgress {
		go s.reportProgress()
	}

	// Walk and process files
	err = s.treeWalker.Walk(ctx, s.fileSystem, visitor)
	if err != nil {
		return fmt.Errorf("failed to walk and process files: %w", err)
	}

	// Close progress channel
	close(s.progressChan)

	if s.config.ShowProgress {
		fmt.Printf("Processed %d files successfully\n", visitor.fileCount)
	}

	// Project level inheritance analysis is possible only after all files are processed
	err = s.performProjectLevelAnalysis(ctx, db, project.ID, visitor.symbolRegistry)
	if err != nil {
		return fmt.Errorf("failed to perform project-level analysis: %w", err)
	}

	return nil
}

func (s *scanner) createProject(ctx context.Context, db *ent.Client) (*ent.Project, error) {
	projectBuilder := db.Project.Create().
		SetName(s.config.ProjectName).
		SetRootPath(s.config.InputDirectory).
		SetScannedAt(time.Now())

	if s.config.GitHash != "" {
		projectBuilder = projectBuilder.SetGitHash(s.config.GitHash)
	}

	// Add metadata
	metadata := map[string]any{
		"languages":        s.getLanguageNames(),
		"max_depth":        s.config.MaxDepth,
		"include_patterns": s.config.IncludePatterns,
		"exclude_patterns": s.config.ExcludePatterns,
	}

	projectBuilder = projectBuilder.SetMetadata(metadata)
	return projectBuilder.Save(ctx)
}

func (s *scanner) getLanguageNames() []string {
	names := make([]string, len(s.languages))
	for i, lang := range s.languages {
		names[i] = string(lang.Meta().Code)
	}

	return names
}

func (s *scanner) reportProgress() {
	for msg := range s.progressChan {
		fmt.Printf("  %s\n", msg)
	}
}

type fileProcessor struct {
	scanner        *scanner
	db             *ent.Client
	ctx            context.Context
	fileCount      int
	symbolRegistry *SymbolRegistry
}

func (fp *fileProcessor) VisitTree(tree core.ParseTree) error {
	fp.fileCount++

	// Get file information
	file, err := tree.File()
	if err != nil {
		return fmt.Errorf("failed to get file from tree: %w", err)
	}

	if fp.scanner.config.ShowProgress {
		select {
		case fp.scanner.progressChan <- fmt.Sprintf("Processing file %d: %s", fp.fileCount, file.Name()):
		default:
		}
	}

	// Create file record
	fileRecord, err := fp.createFileRecord(file)
	if err != nil {
		return fmt.Errorf("failed to create file record for %s: %w", file.Name(), err)
	}

	// Extract and persist AST nodes (only if flag is enabled)
	if fp.scanner.config.PersistASTNodes {
		err = fp.extractAndPersistASTNodes(tree, fileRecord)
		if err != nil {
			return fmt.Errorf("failed to extract AST nodes from %s: %w", file.Name(), err)
		}
	}

	// Extract and persist symbols (classes, functions)
	err = fp.extractAndPersistSymbols(tree, fileRecord)
	if err != nil {
		return fmt.Errorf("failed to extract symbols from %s: %w", file.Name(), err)
	}

	// Extract and persist import statements
	err = fp.extractAndPersistImports(tree, fileRecord)
	if err != nil {
		return fmt.Errorf("failed to extract imports from %s: %w", file.Name(), err)
	}

	return nil
}

func (fp *fileProcessor) createFileRecord(file core.File) (*ent.File, error) {
	// Calculate relative path from project root
	absPath := file.Name()
	relPath, err := filepath.Rel(fp.scanner.config.InputDirectory, absPath)
	if err != nil {
		// Fallback to absolute path
		relPath = absPath
	}

	// Get file content and calculate stats
	reader, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("failed to get file reader: %w", err)
	}

	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	// Get file info using os.Stat on the absolute path
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate content hash and line count
	contentHash := fmt.Sprintf("%x", content[:min(len(content), 32)]) // First 32 bytes as simple hash
	lineCount := fp.countLines(content)

	// Detect language
	languageCode := fp.detectLanguage(file)

	// Create file record
	return fp.db.File.Create().
		SetRelativePath(relPath).
		SetAbsolutePath(absPath).
		SetLanguage(fileent.Language(languageCode)).
		SetContentHash(contentHash).
		SetSizeBytes(int(fileInfo.Size())).
		SetLineCount(lineCount).
		SetCreatedAt(fileInfo.ModTime()).
		SetUpdatedAt(fileInfo.ModTime()).
		SetProjectID(fp.scanner.projectID).
		Save(fp.ctx)
}

func (fp *fileProcessor) countLines(content []byte) int {
	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}

	return lines
}

func (fp *fileProcessor) detectLanguage(file core.File) string {
	// Try to detect language from file extension
	for _, lang := range fp.scanner.languages {
		if slices.Contains(lang.Meta().SourceFileExtensions, filepath.Ext(file.Name())) {
			return string(lang.Meta().Code)
		}
	}

	return LanguageUnknown
}

// performProjectLevelAnalysis performs comprehensive project-level inheritance analysis
func (s *scanner) performProjectLevelAnalysis(ctx context.Context, db *ent.Client, projectID int, symbolRegistry *SymbolRegistry) error {
	if s.config.ShowProgress {
		fmt.Println("Performing project-level inheritance analysis...")
	}

	// Initialize project inheritance analyzer
	analyzer := NewProjectInheritanceAnalyzer(db, ctx, symbolRegistry, s.config)

	// 1. Resolve pending cross-file symbol links
	err := symbolRegistry.ResolvePendingInheritance()
	if err != nil {
		return fmt.Errorf("failed to resolve pending inheritance: %w", err)
	}

	// 2. Build global inheritance graph
	err = analyzer.BuildGlobalGraph(projectID)
	if err != nil {
		return fmt.Errorf("failed to build global inheritance graph: %w", err)
	}

	// 3. Store computed relationships
	computedProcessor := NewComputedInheritanceProcessor(db, ctx, symbolRegistry, s.config)
	err = computedProcessor.StoreComputedRelationships(projectID, analyzer.GetGlobalGraph())
	if err != nil {
		return fmt.Errorf("failed to store computed relationships: %w", err)
	}

	// 4. Update symbols with computed inheritance data
	err = computedProcessor.UpdateSymbolsWithComputedData(projectID, analyzer.GetGlobalGraph())
	if err != nil {
		return fmt.Errorf("failed to update symbols with computed data: %w", err)
	}

	// 5. Generate and store hierarchy statistics
	stats, err := analyzer.GenerateHierarchyStatistics()
	if err != nil {
		return fmt.Errorf("failed to generate hierarchy statistics: %w", err)
	}

	// Update project metadata with inheritance statistics
	symbolRegistryStats := symbolRegistry.GetStatistics()
	currentMetadata := map[string]any{
		"languages":              s.getLanguageNames(),
		"max_depth":              s.config.MaxDepth,
		"include_patterns":       s.config.IncludePatterns,
		"exclude_patterns":       s.config.ExcludePatterns,
		"inheritance_statistics": stats,
		"symbol_registry_stats": map[string]any{
			"total_symbols":        symbolRegistryStats.TotalSymbols,
			"pending_links":        symbolRegistryStats.PendingLinks,
			"cache_entries":        symbolRegistryStats.CacheEntries,
			"module_cache_entries": symbolRegistryStats.ModuleCacheEntries,
		},
	}

	_, err = db.Project.UpdateOneID(projectID).
		SetMetadata(currentMetadata).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update project with inheritance statistics: %w", err)
	}

	// 6. Detect and report inheritance issues
	if s.config.Verbose {
		issues, err := analyzer.DetectHierarchyIssues()
		if err == nil && len(issues) > 0 {
			fmt.Println("Inheritance Quality Issues Detected:")
			for _, issue := range issues {
				fmt.Printf("  [%s] %s: %s\n",
					strings.ToUpper(issue.Severity),
					issue.Type,
					issue.Description)
				if s.config.Verbose && issue.Suggestion != "" {
					fmt.Printf("    Suggestion: %s\n", issue.Suggestion)
				}
			}
		}

		// Show project statistics in verbose mode
		if s.config.ShowProgress {
			fmt.Printf("  Project Statistics:\n")
			if totalClasses, ok := stats["total_classes"].(int); ok {
				fmt.Printf("    Total Classes: %d\n", totalClasses)
			}
			if withInheritance, ok := stats["classes_with_inheritance"].(int); ok {
				fmt.Printf("    Classes with Inheritance: %d\n", withInheritance)
			}
			if multipleInheritance, ok := stats["multiple_inheritance_count"].(int); ok {
				fmt.Printf("    Multiple Inheritance: %d\n", multipleInheritance)
			}
			if maxDepth, ok := stats["max_inheritance_depth"].(int); ok {
				fmt.Printf("    Max Inheritance Depth: %d\n", maxDepth)
			}
			if circularErrors, ok := stats["circular_inheritance_errors"].(int); ok && circularErrors > 0 {
				fmt.Printf("    Circular Inheritance Errors: %d\n", circularErrors)
			}
		}
	}

	// 7. Generate quality processing results
	qualityIssues, err := computedProcessor.ProcessInheritanceQuality(projectID, analyzer.GetGlobalGraph())
	if err == nil && len(qualityIssues) > 0 && s.config.Verbose {
		fmt.Printf("Additional Quality Analysis: %d issues found\n", len(qualityIssues))
	}

	// 8. Validate global graph consistency
	if s.config.Verbose {
		warnings := analyzer.ValidateGlobalGraph()
		if len(warnings) > 0 {
			fmt.Println("Graph Validation Warnings:")
			for _, warning := range warnings {
				fmt.Printf("  Warning: %s\n", warning)
			}
		}
	}

	if s.config.ShowProgress {
		fmt.Println("Project-level analysis completed successfully!")
	}

	return nil
}
