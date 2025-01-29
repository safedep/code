package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"entgo.io/ent/dialect/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/safedep/code/core"
	"github.com/safedep/code/ent"
	"github.com/safedep/code/ent/codefile"
	"github.com/safedep/code/fs"
	"github.com/safedep/code/lang"
	"github.com/safedep/code/parser"
	"github.com/safedep/code/plugin"
	"github.com/safedep/code/plugin/callgraph"
	"github.com/safedep/code/plugin/depsusage"
	"github.com/safedep/code/storage"
	"github.com/safedep/dry/log"
)

var (
	dirToWalk string
	language  string
)

func init() {
	log.InitZapLogger("walker", "dev")

	flag.StringVar(&dirToWalk, "dir", "", "Directory to walk")
	flag.StringVar(&language, "lang", "python", "Language to use for parsing files")

	flag.Parse()
}

func main() {
	if dirToWalk == "" {
		flag.Usage()
		return
	}

	err := run()
	if err != nil {
		panic(err)
	}
}

func saveCallGraph(ctx context.Context, cg callgraph.CallGraph, codeAnalysisStorage core.CodeAnalysisStorage) error {
	nodeMap := make(map[string]*ent.CallgraphNode)

	client, err := codeAnalysisStorage.Client()
	if err != nil {
		return err
	}

	// Initialize nodes
	for _, node := range cg.Nodes {
		entNode, err := client.CallgraphNode.
			Create().
			SetNamespace(node.Namespace).
			Save(ctx)
		if err != nil {
			return err
		}
		nodeMap[node.Namespace] = entNode
	}

	// Populate edges
	for _, node := range cg.Nodes {
		caller := nodeMap[node.Namespace]
		for _, calleeNamespace := range node.CallsTo {
			callee := nodeMap[calleeNamespace]
			_, err := caller.Update().
				AddCallsTo(callee).
				Save(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func saveUsageEvidence(ctx context.Context, evidence *depsusage.UsageEvidence, codeAnalysisStorage core.CodeAnalysisStorage) error {
	fmt.Println("saving", evidence)
	client, err := codeAnalysisStorage.Client()
	if err != nil {
		return err
	}

	filePath := evidence.FilePath

	// Check if the CodeFile exists, if not create it
	cf, err := client.CodeFile.
		Query().
		Where(codefile.FilePath(filePath)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			cf, err = client.CodeFile.
				Create().
				SetFilePath(filePath).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create CodeFile: %w", err)
			}
		} else {
			return fmt.Errorf("failed to query CodeFile: %w", err)
		}
	}

	createdCf, err := client.UsageEvidence.
		Create().
		SetPackageHint(evidence.PackageHint).
		SetModuleName(evidence.ModuleName).
		SetModuleItem(evidence.ModuleItem).
		SetModuleAlias(evidence.ModuleAlias).
		SetIsWildCardUsage(evidence.IsWildCardUsage).
		SetIdentifier(evidence.Identifier).
		SetUsageFilePath(filePath).
		SetLine(evidence.Line).
		SetCodeFile(cf).
		Save(ctx)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to create UsageEvidence: %w", err)
	}
	fmt.Println("Created", createdCf.ID, createdCf.PackageHint)
	return nil
}

func printEvidencesForEachFile(ctx context.Context, codeAnalysisStorage core.CodeAnalysisStorage) error {
	client, err := codeAnalysisStorage.Client()
	if err != nil {
		return err
	}

	codeFiles, err := client.CodeFile.
		Query().
		WithUsageEvidences().
		All(ctx)
	if err != nil {
		return fmt.Errorf("failed querying code files: %w", err)
	}

	for _, cf := range codeFiles {
		fmt.Println()
		fmt.Printf("CodeFile: %s ----------------\n", cf.FilePath)

		evidences := cf.Edges.UsageEvidences
		if len(evidences) == 0 {
			fmt.Println("No usage evidences found.")
			continue
		}

		for _, evidence := range evidences {
			wildcardStr := ""
			if evidence.IsWildCardUsage {
				wildcardStr = " (Wildcard)"
			}
			fmt.Printf("UsageEvidence: %s/%s, (%s>%s)%s, %s//%s:L%d\n",
				*evidence.PackageHint, evidence.ModuleName, *evidence.ModuleItem, *evidence.ModuleAlias, wildcardStr, evidence.UsageFilePath, *evidence.Identifier, evidence.Line)
		}
	}

	return nil
}

func run() error {
	fileSystem, err := fs.NewLocalFileSystem(fs.LocalFileSystemConfig{
		AppDirectories: []string{dirToWalk},
	})

	if err != nil {
		return fmt.Errorf("failed to create local filesystem: %w", err)
	}

	var codeAnalysisStorage core.CodeAnalysisStorage = storage.NewSqliteStorage("caf")
	codeAnalysisStorage.Init(context.Background())
	defer codeAnalysisStorage.Close()

	language, err := lang.GetLanguage(language)
	if err != nil {
		return fmt.Errorf("failed to get language: %w", err)
	}

	walker, err := fs.NewSourceWalker(fs.SourceWalkerConfig{}, language)
	if err != nil {
		return fmt.Errorf("failed to create source walker: %w", err)
	}

	treeWalker, err := parser.NewWalkingParser(walker, language)
	if err != nil {
		return fmt.Errorf("failed to create tree walker: %w", err)
	}

	// consume callgraph
	var callgraphCallback callgraph.CallgraphCallback = func(cg *callgraph.CallGraph) error {
		cg.PrintCallGraph()

		fmt.Println("DFS Traversal:")
		for _, node := range cg.DFS() {
			fmt.Println(node)
		}
		fmt.Println()

		saveCallGraph(context.Background(), *cg, codeAnalysisStorage)
		fmt.Println("Graph saved successfully!")
		fmt.Println()
		return nil
	}

	// consume usage evidences
	var usageCallback depsusage.DependencyUsageCallback = func(ctx context.Context, evidence *depsusage.UsageEvidence) error {
		saveUsageEvidence(ctx, evidence, codeAnalysisStorage)
		return nil
	}

	pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
		callgraph.NewCallGraphPlugin(callgraphCallback),
		depsusage.NewDependencyUsagePlugin(usageCallback),
	})

	if err != nil {
		return fmt.Errorf("failed to create plugin executor: %w", err)
	}

	err = pluginExecutor.Execute(context.Background(), fileSystem)
	if err != nil {
		return fmt.Errorf("failed to execute plugin: %w", err)
	}

	fmt.Println()
	fmt.Println("Findings ----------------------------------------")
	fmt.Println()

	fmt.Println("DFS over sqlite DB")
	client, err := codeAnalysisStorage.Client()
	if err != nil {
		return err
	}
	codeFiles, err := client.CodeFile.
		Query().
		WithUsageEvidences().
		All(context.Background())
	if err != nil {
		return fmt.Errorf("failed querying code files: %w", err)
	}
	for _, cf := range codeFiles {
		namespace := cf.FilePath
		rootNode, err := getCallgraphNode(context.Background(), &codeAnalysisStorage, namespace)
		if err != nil {
			log.Errorf("Error getting root node for %s - %v", namespace, err)
			continue
		}
		dfs(context.Background(), &codeAnalysisStorage, rootNode)
		fmt.Println()
	}

	fmt.Println()

	fmt.Println("Evidences")
	printEvidencesForEachFile(context.Background(), codeAnalysisStorage)

	return nil
}

func getCallgraphNode(ctx context.Context, graphStorage *core.CodeAnalysisStorage, namespace string) (*ent.CallgraphNode, error) {
	client, err := (*graphStorage).Client()
	if err != nil {
		return nil, err
	}
	return client.CallgraphNode.
		Query().
		Where(func(s *sql.Selector) {
			s.Where(sql.EQ("namespace", namespace)) // Match the namespace field
		}).
		Only(ctx)
}

func dfs(ctx context.Context, graphStorage *core.CodeAnalysisStorage, node *ent.CallgraphNode) {
	visited := make(map[string]bool)
	client, err := (*graphStorage).Client()
	if err != nil {
		log.Errorf("Error resolving client:", err)
		return
	}
	dfsUtil(ctx, client, node, 0, visited)
}

func dfsUtil(ctx context.Context, client *ent.Client, node *ent.CallgraphNode, depth int, visited map[string]bool) {
	if visited[node.Namespace] {
		fmt.Printf("%s Stopped at %s\n", strings.Repeat("|", depth), node.Namespace)
		return
	}

	visited[node.Namespace] = true
	fmt.Printf("%s %s\n", strings.Repeat(">", depth), node.Namespace)

	nodes, err := node.QueryCallsTo().All(ctx)
	if err != nil {
		log.Debugf("Error querying callsTo: %v", err)
		return
	}

	for _, n := range nodes {
		dfsUtil(ctx, client, n, depth+1, visited)
	}
}
