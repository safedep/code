package callgraph

import (
	"context"
	"fmt"
	"testing"

	"github.com/safedep/code/core"
	"github.com/safedep/code/pkg/test"
	"github.com/safedep/code/plugin"
	"github.com/stretchr/testify/assert"
)

type callgraphTestcase struct {
	Language core.LanguageCode
	FilePath string

	// Namezpaces representing assignment graph nodes (not exhaustive)
	ExpectedAssignmentGraph map[string][]string

	// Namezpaces representing callgraph nodes (not exhaustive)
	ExpectedCallGraph map[string][]string
}

var testcases = []callgraphTestcase{
	{
		Language: core.LanguageCodePython,
		FilePath: "fixtures/testClass.py",
		ExpectedAssignmentGraph: map[string][]string{
			"printxyz2":                {"xyz//printxyz2"},
			"xyz//printxyz2":           {},
			"printxyz1":                {"xyz//printxyz1"},
			"xyz//printxyz1":           {},
			"fixtures/testClass.py//x": {"fixtures/testClass.py//ClassA", "fixtures/testClass.py//ClassB"},
			"fixtures/testClass.py//TesterClass//__init__":    {},
			"fixtures/testClass.py//alice":                    {"fixtures/testClass.py//TesterClass"},
			"fixtures/testClass.py//bannername":               {"fixtures/testClass.py//TesterClass//name"},
			"fixtures/testClass.py//y":                        {"fixtures/testClass.py//x"},
			"fixtures/testClass.py//TesterClass//self//name":  {"fixtures/testClass.py//TesterClass//__init__//\"TesterClass name\""},
			"fixtures/testClass.py//TesterClass//self//value": {"fixtures/testClass.py//TesterClass//__init__//42", "fixtures/testClass.py//TesterClass//__init__//100"},
		},
		ExpectedCallGraph: map[string][]string{
			"fixtures/testClass.py": {
				"fixtures/testClass.py//TesterClass",
				"fixtures/testClass.py//TesterClass//aboutme",
				"fixtures/testClass.py//ClassA",
				"fixtures/testClass.py//ClassB",
				"fixtures/testClass.py//ClassA//method1",
				"fixtures/testClass.py//ClassB//method1",
				"fixtures/testClass.py//ClassA//method2",
				"fixtures/testClass.py//ClassB//method2",
				"fixtures/testClass.py//ClassA//methodUnique",
				"fixtures/testClass.py//ClassB//methodUnique",
			},
			"fixtures/testClass.py//TesterClass":                 {"fixtures/testClass.py//TesterClass//__init__"},
			"fixtures/testClass.py//TesterClass//__init__":       {"getenv"},
			"fixtures/testClass.py//TesterClass//self//__init__": {"fixtures/testClass.py//TesterClass//__init__"},
			"fixtures/testClass.py//TesterClass//self//aboutme":  {"fixtures/testClass.py//TesterClass//aboutme"},
			"fixtures/testClass.py//TesterClass//deepest_method": {"fixtures/testClass.py//TesterClass//self//helper_method", "print"},
			"fixtures/testClass.py//TesterClass//helper_method":  {"print"},
			"fixtures/testClass.py//TesterClass//aboutme":        {"print"},
			"fixtures/testClass.py//ClassA":                      {},
			"fixtures/testClass.py//ClassA//self":                {},
			"fixtures/testClass.py//ClassA//self//method1":       {"fixtures/testClass.py//ClassA//method1"},
			"fixtures/testClass.py//ClassA//method1":             {"printxyz2"},
			"fixtures/testClass.py//ClassA//method2":             {"printxyz2"},
			"fixtures/testClass.py//ClassB":                      {},
			"fixtures/testClass.py//ClassB//method1":             {"printxyz2"},
			"fixtures/testClass.py//ClassB//self//method2":       {"fixtures/testClass.py//ClassB//method2"},
			"fixtures/testClass.py//ClassB//methodUnique":        {"printxyz3", "pprint//pp"},
			"fixtures/testClass.py//ClassB//self//methodUnique":  {"fixtures/testClass.py//ClassB//methodUnique"},
		},
	},
	{
		Language: core.LanguageCodePython,
		FilePath: "fixtures/testFunctions.py",
		ExpectedAssignmentGraph: map[string][]string{
			"listdirfn":                      {"os//listdir"},
			"printer2":                       {"xyzprintmodule//printer2"},
			"printer3":                       {"xyzprintmodule//printer3"},
			"printer4":                       {"xyzprintmodule//printer4"},
			"fixtures/testFunctions.py//baz": {"fixtures/testFunctions.py//bar"},
			"fixtures/testFunctions.py//xyz": {"fixtures/testFunctions.py//\"abc\"", "fixtures/testFunctions.py//25", "fixtures/testFunctions.py//foo", "fixtures/testFunctions.py//baz"},
			"fixtures/testFunctions.py//r1":  {"fixtures/testFunctions.py//95", "fixtures/testFunctions.py//7.3", "fixtures/testFunctions.py//2"},
			"fixtures/testFunctions.py//res": {"fixtures/testFunctions.py//r1", "fixtures/testFunctions.py//somenumber", "fixtures/testFunctions.py//95", "fixtures/testFunctions.py//7.3"},
		},
		ExpectedCallGraph: map[string][]string{
			"fixtures/testFunctions.py": {
				"fixtures/testFunctions.py//factorial",
				"print",
				"fixtures/testFunctions.py//xyz",
				"fixtures/testFunctions.py//nestParent",
				"fixtures/testFunctions.py//add",
				"fixtures/testFunctions.py//sub",
				"pstats//getsomestat",
			},
			"fixtures/testFunctions.py//factorial":                                    {"fixtures/testFunctions.py//factorial"},
			"fixtures/testFunctions.py//foo":                                          {"pprint//pprint"},
			"fixtures/testFunctions.py//bar":                                          {"print"},
			"fixtures/testFunctions.py//outerfn1":                                     {"chmod"},
			"fixtures/testFunctions.py//nestParent":                                   {"fixtures/testFunctions.py//outerfn1", "fixtures/testFunctions.py//nestParent//nestChild"},
			"fixtures/testFunctions.py//outerfn2":                                     {"listdirfn"},
			"fixtures/testFunctions.py//nestParent//nestChild//fn1":                   {"printer6"},
			"fixtures/testFunctions.py//nestParent//nestChild//childScopedFn":         {"printer2", "fixtures/testFunctions.py//nestParent//nestChild//fn1"},
			"fixtures/testFunctions.py//nestParent//nestChild":                        {"printer1", "fixtures/testFunctions.py//outerfn1", "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild"},
			"fixtures/testFunctions.py//fn1":                                          {"printer4"},
			"fixtures/testFunctions.py//nestParent//nestChild//nestGrandChildUseless": {"printer3"},
			"fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild":        {"pprint//pp", "fixtures/testFunctions.py//nestParent//parentScopedFn", "fixtures/testFunctions.py//outerfn2", "fixtures/testFunctions.py//nestParent//nestChild//childScopedFn"},
		},
	},
}

func TestCallgraphPlugin(t *testing.T) {
	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("%s__%s", testcase.FilePath, testcase.Language), func(t *testing.T) {
			filePaths := []string{testcase.FilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{testcase.Language})
			assert.NoError(t, err)

			var callgraphCallback CallgraphCallback = func(ctx context.Context, cg *CallGraph) error {
				for assigneeNamespace, expectedAssignmentsNamespaces := range testcase.ExpectedAssignmentGraph {
					assigneeNode, exists := cg.assignmentGraph.Assignments[assigneeNamespace]
					assert.True(t, exists, "Expected assignee node %s to exist in assignment graph", assigneeNamespace)
					assert.NotNil(t, assigneeNode, "Expected assignee node %s to be non-nil", assigneeNamespace)
					if assigneeNode == nil {
						continue
					}

					assert.Equal(t, assigneeNamespace, assigneeNode.Namespace)
					assert.ElementsMatch(t, expectedAssignmentsNamespaces, assigneeNode.AssignedTo)
				}

				for sourceNamespace, expectedTargetNamespaces := range testcase.ExpectedCallGraph {
					sourceNode, exists := cg.Nodes[sourceNamespace]
					assert.True(t, exists, "Expected source node %s to exist in call graph", sourceNamespace)
					assert.NotNil(t, sourceNode, "Expected source node %s to be non-nil", sourceNamespace)
					if sourceNode == nil {
						continue
					}

					assert.Equal(t, sourceNamespace, sourceNode.Namespace)
					assert.ElementsMatch(t, expectedTargetNamespaces, sourceNode.CallsTo)
				}
				return nil
			}

			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewCallGraphPlugin(callgraphCallback),
			})
			assert.NoError(t, err)

			err = pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err)
		})
	}
}
