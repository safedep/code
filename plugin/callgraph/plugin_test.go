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

type dfsResultExpectation struct {
	Namespace               string
	CallerNamespace         string
	CallerIdentifierContent string
}

type callgraphTestcase struct {
	Language core.LanguageCode
	FilePath string

	// Namezpaces representing assignment graph nodes (not exhaustive)
	ExpectedAssignmentGraph map[string][]string

	// Namezpaces representing callgraph nodes (not exhaustive)
	ExpectedCallGraph map[string][]string

	// This is the list of minimum expected DFS results items needed to be present as result of cg.DFS()
	// This should not be exhaustive, but should cover most edge cases
	ExpectedDfsResults []dfsResultExpectation
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
				"fixtures/testClass.py//ClassA//method1",
				"fixtures/testClass.py//ClassB//method1",
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
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testClass.py", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: ""},
			{Namespace: "getenv", CallerNamespace: "fixtures/testClass.py//TesterClass//__init__", CallerIdentifierContent: "getenv"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//aboutme", CallerIdentifierContent: "print"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//deepest_method", CallerIdentifierContent: "print"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//helper_method", CallerIdentifierContent: "print"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassA//method1", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassA//method2", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassB//method1", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassB//method2", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz3", CallerNamespace: "fixtures/testClass.py//ClassB//methodUnique", CallerIdentifierContent: "printxyz3"},
			{Namespace: "fixtures/testClass.py//ClassA", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "ClassA"},
			{Namespace: "fixtures/testClass.py//ClassA//method1", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "x.method1"},
			{Namespace: "fixtures/testClass.py//ClassA//method1", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.method1"},
			{Namespace: "fixtures/testClass.py//ClassA//method2", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.method2"},
			{Namespace: "fixtures/testClass.py//ClassB//methodUnique", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.methodUnique"},
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
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testFunctions.py", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: ""},
			{Namespace: "os//chmod", CallerNamespace: "fixtures/testFunctions.py//outerfn1", CallerIdentifierContent: "chmod"},
			{Namespace: "os//listdir", CallerNamespace: "fixtures/testFunctions.py//outerfn2", CallerIdentifierContent: "listdirfn"},
			{Namespace: "pprint//pp", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerIdentifierContent: "pprint.pp"},
			{Namespace: "pprint//pprint", CallerNamespace: "fixtures/testFunctions.py//foo", CallerIdentifierContent: "pprint.pprint"},
			{Namespace: "fixtures/testFunctions.py//factorial", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "factorial"},
			{Namespace: "fixtures/testFunctions.py//factorial", CallerNamespace: "fixtures/testFunctions.py//factorial", CallerIdentifierContent: "factorial"},
			{Namespace: "fixtures/testFunctions.py//foo", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "xyz"},
			{Namespace: "fixtures/testFunctions.py//bar", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "xyz"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild", CallerNamespace: "fixtures/testFunctions.py//nestParent", CallerIdentifierContent: "nestChild"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild//childScopedFn", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerIdentifierContent: "childScopedFn"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild", CallerIdentifierContent: "nestGrandChild"},
			{Namespace: "fixtures/testFunctions.py//add", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "add"},
			{Namespace: "fixtures/testFunctions.py//sub", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "sub"},
		},
	},
	{
		Language: core.LanguageCodeJava,
		FilePath: "fixtures/CallgraphTestcases.java",
		ExpectedAssignmentGraph: map[string][]string{
			"Dialog":        {"java//awt//Dialog"},
			"Frame":         {"java//awt//Frame"},
			"GridLayout":    {"java//awt//GridLayout"},
			"ScrollPane":    {"java//awt//ScrollPane"},
			"LayoutManager": {"java//awt//LayoutManager"},
			"Window":        {"java//awt//Window"},
			"somelibfn":     {"somelib//xyz//somelibfn"},
			"MailcapFile":   {"com//sun//activation//registries//MailcapFile"},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases": {},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//dg": {
				"Dialog",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//cnv": {
				"java//awt//Canvas",
				"ScrollPane",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//lm": {
				"java//awt//BorderLayout",
				"java//awt//FlowLayout",
				"GridLayout",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//obj": {
				"org//mycompany//mylib//SomeClass",
			},
		},
		ExpectedCallGraph: map[string][]string{
			"fixtures/CallgraphTestcases.java": {
				"fixtures/CallgraphTestcases.java//CallgraphTestcases//main",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases": {
				"com//custompkg//SomeClass//defaultConstructor",
				"com//custompkg//SomeClass//someMethod",
				"com//custompkg//SomeClass//someOtherMethod",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc": {
				"String//valueOf",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//this//myfunc": {
				"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main": {
				"Dialog",
				"Window",
				"Frame",
				"java//awt//Dialog//setTitle",
				"java//awt//Dialog//prop//getSomething",
				"java//awt//Canvas",
				"ScrollPane",
				"java//awt//Canvas//setSize",
				"java//awt//ScrollPane//setSize",
				"java//awt//Canvas//prop//subprop//subsubprop//getSomething",
				"java//awt//ScrollPane//prop//subprop//subsubprop//getSomething",
				"java//awt//BorderLayout",
				"java//awt//Button",
				"java//awt//BorderLayout//addLayoutComponent",
				"java//awt//FlowLayout",
				"java//awt//Container",
				"java//awt//BorderLayout//minimumLayoutSize",
				"java//awt//FlowLayout//minimumLayoutSize",
				"GridLayout",
				"java//awt//BorderLayout//toString",
				"java//awt//FlowLayout//toString",
				"java//awt//GridLayout//toString",
				"java//awt//BorderLayout//prop//getSomething",
				"java//awt//FlowLayout//prop//getSomething",
				"java//awt//GridLayout//prop//getSomething",
				"somelibfn",
				"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc",
				"System//out//println",
				"System//out//xyz//println",
				"System//console",
				"com//companyX//fn1",
				"System//getenv",
				"Math//atan",
				"com//somecompany//customlib//datatransfer//DataTransferer//getInstance",
				"org//mycompany//mylib//SomeClass",
				"org//mycompany//mylib//SomeClass//prop//someMethod",
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/CallgraphTestcases.java", CallerNamespace: "fixtures/CallgraphTestcases.java", CallerIdentifierContent: ""},
			{Namespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerNamespace: "fixtures/CallgraphTestcases.java", CallerIdentifierContent: ""},
			{Namespace: "com//custompkg//SomeClass//defaultConstructor", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.defaultConstructor()"},
			{Namespace: "com//custompkg//SomeClass//someMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.someMethod(i)"},
			{Namespace: "com//custompkg//SomeClass//someOtherMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.someOtherMethod(i, s)"},
			{Namespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerNamespace: "fixtures/CallgraphTestcases.java", CallerIdentifierContent: ""},
			{Namespace: "java//awt//Dialog", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Dialog(new Window(new Frame()))"},
			{Namespace: "java//awt//Window", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Window(new Frame())"},
			{Namespace: "java//awt//Frame", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Frame()"},
			{Namespace: "java//awt//Dialog//setTitle", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "dg.setTitle(\"Test Dialog\")"},
			{Namespace: "java//awt//Dialog//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "dg.prop.getSomething()"},
			{Namespace: "java//awt//Canvas", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Canvas()"},
			{Namespace: "ScrollPane", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new ScrollPane()"},
			{Namespace: "java//awt//ScrollPane", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new ScrollPane()"},
			{Namespace: "java//awt//Canvas//setSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.setSize(100, 100)"},
			{Namespace: "java//awt//ScrollPane//setSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.setSize(100, 100)"},
			{Namespace: "java//awt//ScrollPane//prop//subprop//subsubprop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.prop.subprop.subsubprop.getSomething()"},
			{Namespace: "java//awt//BorderLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.BorderLayout()"},
			{Namespace: "java//awt//Button", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Button(\"North Button\")"},
			{Namespace: "java//awt//BorderLayout//addLayoutComponent", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.addLayoutComponent(\"North\", new java.awt.Button(\"North Button\"))"},
			{Namespace: "java//awt//FlowLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.FlowLayout()"},
			{Namespace: "java//awt//Container", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Container()"},
			{Namespace: "java//awt//BorderLayout//minimumLayoutSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.minimumLayoutSize(new java.awt.Container())"},
			{Namespace: "java//awt//FlowLayout//minimumLayoutSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.minimumLayoutSize(new java.awt.Container())"},
			{Namespace: "GridLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new GridLayout()"},
			{Namespace: "java//awt//GridLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new GridLayout()"},
			{Namespace: "java//awt//BorderLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//FlowLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//GridLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//BorderLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},
			{Namespace: "java//awt//FlowLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},
			{Namespace: "java//awt//GridLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},
			{Namespace: "somelibfn", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "somelibfn()"},
			{Namespace: "somelib//xyz//somelibfn", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "somelibfn()"},
			{Namespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "myfunc()"},
			{Namespace: "String//valueOf", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerIdentifierContent: "String.valueOf('c')"},
			{Namespace: "System//out//println", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.out.println(\"GG\")"},
			{Namespace: "System//out//xyz//println", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.out.xyz.println(\"GG\")"},
			{Namespace: "System//console", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.console().readPassword()"},
			{Namespace: "com//companyX//fn1", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "com.companyX.fn1()"},
			{Namespace: "System//getenv", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.getenv().keySet().iterator(com.companyX.fn1()).hasNext()"},
			{Namespace: "Math//atan", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "Math.atan(1.0)"},
			{Namespace: "com//somecompany//customlib//datatransfer//DataTransferer//getInstance", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "com.somecompany.customlib.datatransfer.DataTransferer.getInstance()"},
			{Namespace: "org//mycompany//mylib//SomeClass", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new org.mycompany.mylib.SomeClass()"},
			{Namespace: "org//mycompany//mylib//SomeClass//prop//someMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "obj.prop.someMethod(\"GG\")"},
		},
	},
}

func TestCallgraphPlugin(t *testing.T) {
	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("%s__%s", testcase.FilePath, testcase.Language), func(t *testing.T) {
			t.Parallel()

			filePaths := []string{testcase.FilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext(filePaths, []core.LanguageCode{testcase.Language})
			assert.NoError(t, err)

			var callgraphCallback CallgraphCallback = func(ctx context.Context, cg *CallGraph) error {
				assert.NotNil(t, cg, "Expected call graph to be non-nil")
				assert.NotNil(t, cg.assignmentGraph, "Expected assignment graph to be non-nil")

				treeData, err := cg.Tree.Data()
				assert.NoError(t, err)
				assert.NotNil(t, treeData, "Expected tree data to be non-nil")

				assertAssignmentGraph(t, cg, testcase.ExpectedAssignmentGraph)

				assertCallGraph(t, cg, testcase.ExpectedCallGraph)

				assertDfs(t, cg, testcase.ExpectedDfsResults, treeData)

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

func assertAssignmentGraph(t *testing.T, cg *CallGraph, expectedAssignmentGraph map[string][]string) {
	for assigneeNamespace, expectedAssignmentsNamespaces := range expectedAssignmentGraph {
		assigneeNode, exists := cg.assignmentGraph.Assignments[assigneeNamespace]
		assert.True(t, exists, "Expected assignee node %s to exist in assignment graph", assigneeNamespace)
		assert.NotNil(t, assigneeNode, "Expected assignee node %s to be non-nil", assigneeNamespace)
		if assigneeNode == nil {
			continue
		}

		assert.Equal(t, assigneeNamespace, assigneeNode.Namespace)
		assert.ElementsMatch(t, expectedAssignmentsNamespaces, assigneeNode.AssignedTo)
	}
}

func assertCallGraph(t *testing.T, cg *CallGraph, expectedCallGraph map[string][]string) {
	for sourceNamespace, expectedTargetNamespaces := range expectedCallGraph {
		sourceNode, exists := cg.Nodes[sourceNamespace]
		assert.True(t, exists, "Expected source node %s to exist in call graph", sourceNamespace)
		assert.NotNil(t, sourceNode, "Expected source node %s to be non-nil", sourceNamespace)
		if sourceNode == nil {
			continue
		}

		assert.Equal(t, sourceNamespace, sourceNode.Namespace)

		targetNamespaces := []string{}
		for _, call := range sourceNode.CallsTo {
			targetNamespaces = append(targetNamespaces, call.CalleeNamespace)
		}

		assert.ElementsMatch(t, expectedTargetNamespaces, targetNamespaces, "Expected target namespaces for source node %s to match", sourceNamespace)
	}

}

func assertDfs(t *testing.T, cg *CallGraph, expectedDfsResults []dfsResultExpectation, treeData *[]byte) {
	type dfsItemKey struct {
		Namespace, CallerNamespace, CallerIdentifierContent string
	}

	dfsResults := cg.DFS()

	actualDfsItems := make(map[dfsItemKey]int)
	for _, dfsResultItem := range dfsResults {
		namespace := dfsResultItem.Namespace

		callerNamespace := ""
		if dfsResultItem.Caller != nil {
			callerNamespace = dfsResultItem.Caller.Namespace
		}

		callerIdentifierContent := ""
		if dfsResultItem.CallerIdentifier != nil {
			callerIdentifierContent = dfsResultItem.CallerIdentifier.Content(*treeData)
		}

		key := dfsItemKey{
			Namespace:               namespace,
			CallerNamespace:         callerNamespace,
			CallerIdentifierContent: callerIdentifierContent,
		}
		actualDfsItems[key]++
	}

	expectedDfsItems := make(map[dfsItemKey]int)
	for _, expectedItem := range expectedDfsResults {
		key := dfsItemKey{
			Namespace:               expectedItem.Namespace,
			CallerNamespace:         expectedItem.CallerNamespace,
			CallerIdentifierContent: expectedItem.CallerIdentifierContent,
		}
		expectedDfsItems[key]++
	}

	// Ensure expectedDfsItems are present in actualDfsItems
	for expectedItem, expectedItemCount := range expectedDfsItems {
		actualCount, found := actualDfsItems[expectedItem]
		assert.True(t, found, "Expected DFS result item %v to be present in results", expectedItem)
		assert.GreaterOrEqual(t, actualCount, expectedItemCount, "Expected DFS result item %v to have at least %d occurrences, found %d", expectedItem, expectedItemCount, actualCount)
	}

}
