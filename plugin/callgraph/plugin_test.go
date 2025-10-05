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

type expectedCallgraphRefs struct {
	Namespace string
	Args      [][]string
}

type callgraphTestcase struct {
	Language core.LanguageCode
	FilePath string

	// Namespaces representing assignment graph nodes (not exhaustive)
	ExpectedAssignmentGraph map[string][]string

	// Namespaces representing callgraph nodes (not exhaustive)
	ExpectedCallGraph map[string][]expectedCallgraphRefs

	// This is the list of minimum expected DFS results items needed to be present as result of cg.DFS()
	// This should not be exhaustive, but should cover most edge cases
	ExpectedDfsResults []dfsResultExpectation
}

var testcases = []callgraphTestcase{
	{
		Language: core.LanguageCodePython,
		FilePath: "fixtures/testClass.py",
		ExpectedAssignmentGraph: map[string][]string{
			"printxyz1":      {"xyz//printxyz1"},
			"xyz//printxyz1": {},
			"printxyz2":      {"xyz//printxyz2"},
			"xyz//printxyz2": {},
			"prt3":           {"xyz//printxyz3"},
			"xyz//printxyz3": {},
			"pprint":         {},
			"fixtures/testClass.py//TesterClass//__init__":    {},
			"fixtures/testClass.py//TesterClass//self//name":  {"\"TesterClass name\""},
			"fixtures/testClass.py//TesterClass//self//value": {"42", "fixtures/testClass.py//TesterClass//__init__//newValue", "100", "\"default value\""},
			"fixtures/testClass.py//alice":                    {"fixtures/testClass.py//TesterClass"},
			"fixtures/testClass.py//bannername":               {"fixtures/testClass.py//TesterClass//name"},
			"fixtures/testClass.py//x":                        {"fixtures/testClass.py//ClassA", "fixtures/testClass.py//ClassB"},
			"fixtures/testClass.py//y":                        {"fixtures/testClass.py//x"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testClass.py": {
				{"fixtures/testClass.py//TesterClass", [][]string{{"35"}}},
				{"fixtures/testClass.py//TesterClass//aboutme", [][]string{}},
				{"fixtures/testClass.py//ClassA", [][]string{}},
				{"fixtures/testClass.py//ClassB", [][]string{}},
				{"fixtures/testClass.py//ClassA//method1", [][]string{}},
				{"fixtures/testClass.py//ClassA//method1", [][]string{}},
				{"fixtures/testClass.py//ClassB//method1", [][]string{}},
				{"fixtures/testClass.py//ClassB//method1", [][]string{}},
				{"fixtures/testClass.py//ClassA//method2", [][]string{}},
				{"fixtures/testClass.py//ClassB//method2", [][]string{}},
				{"fixtures/testClass.py//ClassA//methodUnique", [][]string{}},
				{"fixtures/testClass.py//ClassB//methodUnique", [][]string{}},
			},
			"fixtures/testClass.py//TesterClass": {
				{"fixtures/testClass.py//TesterClass//__init__", [][]string{}},
			},
			"fixtures/testClass.py//TesterClass//self//__init__": {
				{"fixtures/testClass.py//TesterClass//__init__", [][]string{}},
			},
			"fixtures/testClass.py//TesterClass//__init__": {
				{"getenv", [][]string{{"\"USE_TAR\""}}},
			},
			"fixtures/testClass.py//TesterClass//self//helper_method": {
				{"fixtures/testClass.py//TesterClass//helper_method", [][]string{}},
			},
			"fixtures/testClass.py//TesterClass//helper_method": {
				{"print", [][]string{{"\"Called helper_method\""}}},
			},
			"fixtures/testClass.py//TesterClass//self//deepest_method": {
				{"fixtures/testClass.py//TesterClass//deepest_method", [][]string{}},
			},
			"fixtures/testClass.py//TesterClass//deepest_method": {
				{"fixtures/testClass.py//TesterClass//self//helper_method", [][]string{}},
				{"print", [][]string{{"\"Called deepest_method\""}}},
			},
			"fixtures/testClass.py//TesterClass//aboutme": {
				{"print", [][]string{{"f\"Name: {self.name}\""}}},
			},
			"fixtures/testClass.py//ClassA":       {},
			"fixtures/testClass.py//ClassA//self": {},
			"fixtures/testClass.py//ClassA//self//method1": {
				{"fixtures/testClass.py//ClassA//method1", [][]string{}},
			},
			"fixtures/testClass.py//ClassA//method1": {
				{"printxyz2", [][]string{{"\"GG\""}}},
			},
			"fixtures/testClass.py//ClassA//self//method2": {
				{"fixtures/testClass.py//ClassA//method2", [][]string{}},
			},
			"fixtures/testClass.py//ClassA//method2": {
				{"printxyz2", [][]string{{"\"GG\""}}},
			},
			"fixtures/testClass.py//ClassB//self//methodUnique": {
				{"fixtures/testClass.py//ClassB//methodUnique", [][]string{}},
			},
			"fixtures/testClass.py//ClassB//methodUnique": {
				{"prt3", [][]string{{"\"GG\""}}},
				{"pprint//pp", [][]string{{"\"GG\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testClass.py", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testClass.py//TesterClass//__init__", CallerNamespace: "fixtures/testClass.py//TesterClass", CallerIdentifierContent: ""},
			{Namespace: "getenv", CallerNamespace: "fixtures/testClass.py//TesterClass//__init__", CallerIdentifierContent: "getenv"},
			{Namespace: "os//getenv", CallerNamespace: "fixtures/testClass.py//TesterClass//__init__", CallerIdentifierContent: "getenv"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//aboutme", CallerIdentifierContent: "print"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//deepest_method", CallerIdentifierContent: "print"},
			{Namespace: "print", CallerNamespace: "fixtures/testClass.py//TesterClass//helper_method", CallerIdentifierContent: "print"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassA//method1", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassA//method2", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassB//method1", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "fixtures/testClass.py//ClassB//method2", CallerIdentifierContent: "printxyz2"},
			{Namespace: "xyz//printxyz3", CallerNamespace: "fixtures/testClass.py//ClassB//methodUnique", CallerIdentifierContent: "prt3"},
			{Namespace: "fixtures/testClass.py//TesterClass", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "TesterClass"},
			{Namespace: "fixtures/testClass.py//TesterClass//aboutme", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "alice.aboutme"},
			{Namespace: "fixtures/testClass.py//ClassA", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "ClassA"},
			{Namespace: "fixtures/testClass.py//ClassA//method1", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "x.method1"},
			{Namespace: "fixtures/testClass.py//ClassA//method1", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.method1"},
			{Namespace: "fixtures/testClass.py//ClassA//method2", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.method2"},
			{Namespace: "fixtures/testClass.py//ClassB//methodUnique", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: "y.methodUnique"},
			{Namespace: "fixtures/testClass.py//TesterClass//deepest_method", CallerNamespace: "fixtures/testClass.py", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testClass.py//TesterClass//self//helper_method", CallerNamespace: "fixtures/testClass.py//TesterClass//deepest_method", CallerIdentifierContent: "self.helper_method"},
			{Namespace: "fixtures/testClass.py//TesterClass//helper_method", CallerNamespace: "fixtures/testClass.py//TesterClass//self//helper_method", CallerIdentifierContent: ""},
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
			"SOME_CONSTANT":                  {"mypkg//SOME_CONSTANT"},
			"fixtures/testFunctions.py//baz": {"fixtures/testFunctions.py//bar"},
			"fixtures/testFunctions.py//xyz": {
				"\"abc\"",
				"25",
				"fixtures/testFunctions.py//foo",
				"fixtures/testFunctions.py//baz",
			},
			"fixtures/testFunctions.py//r1": {"95", "7.3", "2"},
			"fixtures/testFunctions.py//p1": {"599", "\"going good\"", "39.2"},
			"fixtures/testFunctions.py//p2": {"95", "True"},
			"fixtures/testFunctions.py//p3": {"\"gg\""},
			"fixtures/testFunctions.py//res": {
				"fixtures/testFunctions.py//r1",
				"fixtures/testFunctions.py//somenumber",
				"95",
				"7.3",
			},
			"fixtures/testFunctions.py//mul": {
				"fixtures/testFunctions.py//multiply",
			},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testFunctions.py": {
				{"fixtures/testFunctions.py//factorial", [][]string{{"5"}}},
				{"print", [][]string{{}}},
				{"fixtures/testFunctions.py//xyz", [][]string{}},
				{"fixtures/testFunctions.py//nestParent", [][]string{}},
				{"random//randint", [][]string{{"0"}, {"1"}}},
				{"fixtures/testFunctions.py//add", [][]string{{"599", "\"going good\"", "39.2"}, {"95", "True"}}},
				{"fixtures/testFunctions.py//sub", [][]string{{"\"gg\""}, {"6"}}},
				{"pstats//getsomestat", [][]string{}},
				{"fixtures/testFunctions.py//mul", [][]string{{"2"}, {"3"}}},
				{"fixtures/testFunctions.py//addProxy", [][]string{{"5"}, {"3"}}},
				{"fixtures/testFunctions.py//concat", [][]string{{"\"Hello, \""}, {"\"World!\""}}},
				{"fixtures/testFunctions.py//add", [][]string{{"599", "\"going good\"", "39.2"}, {"\"gg\""}}},
				{"fixtures/testFunctions.py//add", [][]string{{"95", "True"}, {}}},
				{"getenv", [][]string{{"\"SOME_ENV_VAR\""}}},
				{"print", [][]string{{"\"gg\""}, {"1"}, {"2.5"}, {"True"}, {"None"}, {"mypkg//SOME_CONSTANT"}, {}, {}, {}, {}, {}, {}, {}}},
			},
			"fixtures/testFunctions.py//factorial": {
				{
					"fixtures/testFunctions.py//factorial", [][]string{
						{"fixtures/testFunctions.py//factorial//x", "1"},
					},
				},
			},
			"fixtures/testFunctions.py//foo": {
				{"pprint//pprint", [][]string{{"\"foo\""}}},
			},
			"fixtures/testFunctions.py//bar": {
				{"print", [][]string{{"\"bar\""}}},
			},
			"fixtures/testFunctions.py//outerfn1": {
				{"chmod", [][]string{{"\"outerfn1\""}}},
			},
			"fixtures/testFunctions.py//outerfn2": {
				{"listdirfn", [][]string{{"\"outerfn2\""}}},
			},
			"fixtures/testFunctions.py//fn1": {
				{"printer4", [][]string{{"\"outer fn1\""}}},
			},
			"fixtures/testFunctions.py//nestParent": {
				{"fixtures/testFunctions.py//outerfn1", [][]string{}},
				{"fixtures/testFunctions.py//nestParent//nestChild", [][]string{}},
			},
			"fixtures/testFunctions.py//nestParent//parentScopedFn": {
				{"print", [][]string{{"\"parentScopedFn\""}}},
				{"fixtures/testFunctions.py//fn1", [][]string{}},
			},
			"fixtures/testFunctions.py//nestParent//nestChild": {
				{"printer1", [][]string{{"\"nestChild\""}}},
				{"fixtures/testFunctions.py//outerfn1", [][]string{}},
				{"fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", [][]string{}},
			},
			"fixtures/testFunctions.py//nestParent//nestChild//fn1": {
				{"printer6", [][]string{{"\"inner fn1\""}}},
			},
			"fixtures/testFunctions.py//nestParent//nestChild//childScopedFn": {
				{"printer2", [][]string{{"\"childScopedFn\""}}},
				{"fixtures/testFunctions.py//nestParent//nestChild//fn1", [][]string{}},
			},
			"fixtures/testFunctions.py//nestParent//nestChild//nestGrandChildUseless": {
				{"printer3", [][]string{{"\"nestGrandChildUseless\""}}},
			},
			"fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild": {
				{"pprint//pp", [][]string{{"\"nestGrandChild\""}}},
				{"fixtures/testFunctions.py//nestParent//parentScopedFn", [][]string{}},
				{"fixtures/testFunctions.py//outerfn2", [][]string{}},
				{"fixtures/testFunctions.py//nestParent//nestChild//childScopedFn", [][]string{}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testFunctions.py", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testFunctions.py//factorial", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "factorial"},
			{Namespace: "fixtures/testFunctions.py//factorial", CallerNamespace: "fixtures/testFunctions.py//factorial", CallerIdentifierContent: "factorial"},
			{Namespace: "fixtures/testFunctions.py//foo", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "xyz"},
			{Namespace: "fixtures/testFunctions.py//bar", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "xyz"},
			{Namespace: "os//chmod", CallerNamespace: "fixtures/testFunctions.py//outerfn1", CallerIdentifierContent: "chmod"},
			{Namespace: "os//listdir", CallerNamespace: "fixtures/testFunctions.py//outerfn2", CallerIdentifierContent: "listdirfn"},
			{Namespace: "xyzprintmodule//printer4", CallerNamespace: "fixtures/testFunctions.py//fn1", CallerIdentifierContent: "printer4"},
			{Namespace: "pprint//pp", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerIdentifierContent: "pprint.pp"},
			{Namespace: "pprint//pprint", CallerNamespace: "fixtures/testFunctions.py//foo", CallerIdentifierContent: "pprint.pprint"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild", CallerNamespace: "fixtures/testFunctions.py//nestParent", CallerIdentifierContent: "nestChild"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild//childScopedFn", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerIdentifierContent: "childScopedFn"},
			{Namespace: "fixtures/testFunctions.py//nestParent//nestChild//nestGrandChild", CallerNamespace: "fixtures/testFunctions.py//nestParent//nestChild", CallerIdentifierContent: "nestGrandChild"},
			{Namespace: "fixtures/testFunctions.py//add", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "add"},
			{Namespace: "fixtures/testFunctions.py//sub", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "sub"},
			{Namespace: "random//randint", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "random.randint"},
			{Namespace: "fixtures/testFunctions.py//mul", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "mul"},
			{Namespace: "fixtures/testFunctions.py//addProxy", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "addProxy"},
			{Namespace: "fixtures/testFunctions.py//add", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "add"},
			{Namespace: "fixtures/testFunctions.py//concat", CallerNamespace: "fixtures/testFunctions.py", CallerIdentifierContent: "concat"},
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
			"fixtures/CallgraphTestcases.java//UtilityClass//somelibfn//x": {"5"},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//i": {
				"5",
				"7",
				"1",
				"2",
				"59",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//s": {
				"\"GG\"",
				"\"HH\"",
				"\"ii\"",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases": {},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//dg": {
				"Dialog",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//cnv": {
				"java//awt//Canvas",
				"ScrollPane",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//width": {
				"32",
				"64",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//ht": {
				"99",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//lm": {
				"java//awt//BorderLayout",
				"java//awt//FlowLayout",
				"GridLayout",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//componentName": {
				"\"North\"",
				"\"South\"",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//worker": {
				"java//awt//SomeLayoutWorker",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//util": {
				"fixtures/CallgraphTestcases.java//UtilityClass",
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//result": {"938"},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main//obj": {
				"org//mycompany//mylib//SomeClass",
			},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/CallgraphTestcases.java//UtilityClass": {
				{"System//out//println", [][]string{
					{"fixtures/CallgraphTestcases.java//UtilityClass//x"},
				}},
			},
			"fixtures/CallgraphTestcases.java//UtilityClass//somelibfn": {
				{"Math//random", [][]string{}},
			},
			"fixtures/CallgraphTestcases.java": {
				{"fixtures/CallgraphTestcases.java//CallgraphTestcases//main", [][]string{}},
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases": {
				{"com//custompkg//SomeClass//defaultConstructor", [][]string{}},
				{"com//custompkg//SomeClass//someMethod", [][]string{{"5", "7"}}},
				{"com//custompkg//SomeClass//someOtherMethod", [][]string{
					{"5", "7", "1", "2", "59"},
					{"\"GG\"", "\"HH\"", "\"ii\""},
				}},
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//this//myfunc": {
				{"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", [][]string{}},
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc": {
				{"String//valueOf", [][]string{{"'c'"}}},
				{"String//valueOf", [][]string{{"'c'", "59"}}},
			},
			"fixtures/CallgraphTestcases.java//CallgraphTestcases//main": {
				{"Dialog", [][]string{{"java//awt//Window"}}},
				{"Window", [][]string{{"java//awt//Frame"}}},
				{"Frame", [][]string{{"true"}}},
				{"java//awt//Dialog//setTitle", [][]string{{"\"Test Dialog\""}}},
				{"java//awt//Dialog//prop//getSomething", [][]string{}},
				{"java//awt//Canvas", [][]string{}},
				{"ScrollPane", [][]string{}},
				{"java//awt//Canvas//getWidth", [][]string{}},
				{"java//awt//ScrollPane//getWidth", [][]string{}},
				{"Math//random", [][]string{}},
				{"Math//random", [][]string{}},
				{"java//awt//Canvas//setSize", [][]string{
					{"32", "64"},
					{"55", "99"},
				}},
				{"java//awt//ScrollPane//setSize", [][]string{
					{"32", "64"},
					{"55", "99"},
				}},
				{"java//awt//Canvas//prop//subprop//subsubprop//getSomething", [][]string{}},
				{"java//awt//ScrollPane//prop//subprop//subsubprop//getSomething", [][]string{}},
				{"java//awt//BorderLayout", [][]string{}},
				{"Math//random", [][]string{}},
				{"java//awt//Button", [][]string{{"\"North Button\""}}},
				{"java//awt//BorderLayout//addLayoutComponent", [][]string{
					{"\"North\"", "\"South\""},
					{"java//awt//Button"},
				}},
				{"java//awt//FlowLayout", [][]string{}},
				{"java//awt//Container", [][]string{}},
				{"java//awt//BorderLayout//minimumLayoutSize", [][]string{{"java//awt//Container"}}},
				{"java//awt//FlowLayout//minimumLayoutSize", [][]string{{"java//awt//Container"}}},
				{"GridLayout", [][]string{}},

				// Ensures that variable lm is resolved to different objects (BorderLayout, FlowLayout, GridLayout)
				{"java//awt//BorderLayout//toString", [][]string{}},
				{"java//awt//FlowLayout//toString", [][]string{}},
				{"java//awt//GridLayout//toString", [][]string{}},
				{"java//awt//BorderLayout//prop//getSomething", [][]string{}},
				{"java//awt//FlowLayout//prop//getSomething", [][]string{}},
				{"java//awt//GridLayout//prop//getSomething", [][]string{}},
				{"java//awt//SomeLayoutWorker", [][]string{
					{"java//awt//BorderLayout", "java//awt//FlowLayout", "java//awt//GridLayout"},
				}},

				{"fixtures/CallgraphTestcases.java//UtilityClass", [][]string{{"10"}}},
				{"fixtures/CallgraphTestcases.java//UtilityClass//somelibfn", [][]string{{"5"}, {"10"}}},
				{"somelibfn", [][]string{}},
				{"fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", [][]string{}},
				{"System//out//println", [][]string{{"\"GG\""}}},
				{"System//out//xyz//println", [][]string{{"\"GG\""}}},
				{"System//console", [][]string{}},
				{"com//companyX//fn1", [][]string{}},
				{"System//getenv", [][]string{}},
				{"Math//atan", [][]string{{"1.0"}}},
				{"com//somecompany//customlib//datatransfer//DataTransferer//getInstance", [][]string{}},
				{"org//mycompany//mylib//SomeClass", [][]string{}},
				{"org//mycompany//mylib//SomeClass//prop//someMethod", [][]string{{"\"GG\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/CallgraphTestcases.java", CallerNamespace: "fixtures/CallgraphTestcases.java", CallerIdentifierContent: ""},
			{Namespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerNamespace: "fixtures/CallgraphTestcases.java", CallerIdentifierContent: ""},

			{Namespace: "Frame", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Frame(true)"},
			{Namespace: "java//awt//Frame", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Frame(true)"},
			{Namespace: "Window", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Window(new Frame(true))"},
			{Namespace: "java//awt//Window", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Window(new Frame(true))"},
			{Namespace: "Dialog", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Dialog(new Window(new Frame(true)))"},
			{Namespace: "java//awt//Dialog", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new Dialog(new Window(new Frame(true)))"},
			{Namespace: "java//awt//Dialog//setTitle", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "dg.setTitle(\"Test Dialog\")"},
			{Namespace: "java//awt//Dialog//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "dg.prop.getSomething()"},
			{Namespace: "java//awt//Canvas", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Canvas()"},
			{Namespace: "java//awt//ScrollPane", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new ScrollPane()"},
			{Namespace: "java//awt//Canvas//getWidth", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.getWidth()"},
			{Namespace: "java//awt//ScrollPane//getWidth", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.getWidth()"},
			{Namespace: "Math//random", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "Math.random()"},
			{Namespace: "java//awt//Canvas//setSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.setSize(width, (Math.random() < 0.5) ? 55 : ht)"},
			{Namespace: "java//awt//ScrollPane//setSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.setSize(width, (Math.random() < 0.5) ? 55 : ht)"},
			{Namespace: "java//awt//Canvas//prop//subprop//subsubprop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.prop.subprop.subsubprop.getSomething()"},
			{Namespace: "java//awt//ScrollPane//prop//subprop//subsubprop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "cnv.prop.subprop.subsubprop.getSomething()"},

			{Namespace: "java//awt//BorderLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.BorderLayout()"},
			{Namespace: "java//awt//Button", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Button(\"North Button\")"},
			{Namespace: "java//awt//BorderLayout//addLayoutComponent", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.addLayoutComponent(componentName, new java.awt.Button(\"North Button\"))"},
			{Namespace: "java//awt//FlowLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.FlowLayout()"},
			{Namespace: "java//awt//Container", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.Container()"},
			{Namespace: "java//awt//BorderLayout//minimumLayoutSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.minimumLayoutSize(new java.awt.Container())"},
			{Namespace: "java//awt//FlowLayout//minimumLayoutSize", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.minimumLayoutSize(new java.awt.Container())"},
			{Namespace: "java//awt//GridLayout", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new GridLayout()"},
			{Namespace: "java//awt//BorderLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//FlowLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//GridLayout//toString", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.toString()"},
			{Namespace: "java//awt//BorderLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},
			{Namespace: "java//awt//FlowLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},
			{Namespace: "java//awt//GridLayout//prop//getSomething", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "lm.prop.getSomething()"},

			{Namespace: "java//awt//SomeLayoutWorker", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new java.awt.SomeLayoutWorker(lm)"},

			{Namespace: "fixtures/CallgraphTestcases.java//UtilityClass", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new UtilityClass(10)"},
			{Namespace: "System//out//println", CallerNamespace: "fixtures/CallgraphTestcases.java//UtilityClass", CallerIdentifierContent: "System.out.println(x)"},
			{Namespace: "fixtures/CallgraphTestcases.java//UtilityClass//somelibfn", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "util.somelibfn(5, 10)"},

			{Namespace: "somelibfn", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "somelibfn()"},
			{Namespace: "somelib//xyz//somelibfn", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "somelibfn()"},
			{Namespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "myfunc()"},
			{Namespace: "String//valueOf", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerIdentifierContent: "String.valueOf('c')"},
			{Namespace: "String//valueOf", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerIdentifierContent: "String.valueOf(false ? 'c' : 59)"},
			{Namespace: "System//out//println", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.out.println(\"GG\")"},
			{Namespace: "System//out//xyz//println", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.out.xyz.println(\"GG\")"},

			{Namespace: "System//console", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.console().readPassword()"},
			{Namespace: "com//companyX//fn1", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "com.companyX.fn1()"},
			{Namespace: "System//getenv", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "System.getenv().keySet().iterator(com.companyX.fn1()).hasNext()"},
			{Namespace: "Math//atan", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "Math.atan(1.0)"},

			{Namespace: "com//somecompany//customlib//datatransfer//DataTransferer//getInstance", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "com.somecompany.customlib.datatransfer.DataTransferer.getInstance()"},

			{Namespace: "org//mycompany//mylib//SomeClass", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "new org.mycompany.mylib.SomeClass()"},
			{Namespace: "org//mycompany//mylib//SomeClass//prop//someMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases//main", CallerIdentifierContent: "obj.prop.someMethod(\"GG\")"},

			{Namespace: "com//custompkg//SomeClass//defaultConstructor", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.defaultConstructor()"},
			{Namespace: "com//custompkg//SomeClass//someMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.someMethod(i)"},
			{Namespace: "com//custompkg//SomeClass//someOtherMethod", CallerNamespace: "fixtures/CallgraphTestcases.java//CallgraphTestcases", CallerIdentifierContent: "com.custompkg.SomeClass.someOtherMethod(i, s)"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "fixtures/testCallGraph.go",
		ExpectedAssignmentGraph: map[string][]string{
			"fmt": {},
			"os":  {},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testCallGraph.go": {
				{"fixtures/testCallGraph.go//helper", [][]string{}},
				{"fixtures/testCallGraph.go//processFile", [][]string{}},
				{"fixtures/testCallGraph.go//main", [][]string{}},
			},
			"fixtures/testCallGraph.go//main": {
				{"fixtures/testCallGraph.go//helper", [][]string{{"10"}}},
				{"fixtures/testCallGraph.go//processFile", [][]string{{"\"test.txt\""}}},
				{"os//Getenv", [][]string{{"\"HOME\""}}},
				{"fmt//Sprintf", [][]string{{"\"formatted: %v\""}, {"123"}}},
				{"fmt//Println", [][]string{{"fixtures/testCallGraph.go//main//result"}}},
			},
			"fixtures/testCallGraph.go//helper": {
				{"fmt//Println", [][]string{{"\"helper called\""}}},
			},
			"fixtures/testCallGraph.go//processFile": {
				{"os//WriteFile", [][]string{
					{"fixtures/testCallGraph.go//processFile//filename"},
					{"fixtures/testCallGraph.go//processFile//data"},
					{"0644"},
				}},
				{"fmt//Printf", [][]string{
					{"\"Wrote file: %s\\n\""},
					{"fixtures/testCallGraph.go//processFile//filename"},
				}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testCallGraph.go", CallerNamespace: "fixtures/testCallGraph.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testCallGraph.go//helper", CallerNamespace: "fixtures/testCallGraph.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testCallGraph.go//processFile", CallerNamespace: "fixtures/testCallGraph.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testCallGraph.go//main", CallerNamespace: "fixtures/testCallGraph.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testCallGraph.go//helper", CallerNamespace: "fixtures/testCallGraph.go//main", CallerIdentifierContent: "helper"},
			{Namespace: "fixtures/testCallGraph.go//processFile", CallerNamespace: "fixtures/testCallGraph.go//main", CallerIdentifierContent: "processFile"},
			{Namespace: "fmt//Println", CallerNamespace: "fixtures/testCallGraph.go//helper", CallerIdentifierContent: "fmt.Println"},
			{Namespace: "os//WriteFile", CallerNamespace: "fixtures/testCallGraph.go//processFile", CallerIdentifierContent: "os.WriteFile"},
			{Namespace: "fmt//Printf", CallerNamespace: "fixtures/testCallGraph.go//processFile", CallerIdentifierContent: "fmt.Printf"},
			{Namespace: "os//Getenv", CallerNamespace: "fixtures/testCallGraph.go//main", CallerIdentifierContent: "os.Getenv"},
			{Namespace: "fmt//Sprintf", CallerNamespace: "fixtures/testCallGraph.go//main", CallerIdentifierContent: "fmt.Sprintf"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "fixtures/testGoLibrary.go",
		ExpectedAssignmentGraph: map[string][]string{
			"fmt":     {},
			"strings": {},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testGoLibrary.go": {
				{"fixtures/testGoLibrary.go//ProcessData", [][]string{}},
				{"fixtures/testGoLibrary.go//ValidateInput", [][]string{}},
				{"fixtures/testGoLibrary.go//logError", [][]string{}},
				{"fixtures/testGoLibrary.go//Transform", [][]string{}},
			},
			"fixtures/testGoLibrary.go//ProcessData": {
				{"strings//ToUpper", [][]string{{"fixtures/testGoLibrary.go//ProcessData//data"}}},
				{"fmt//Printf", [][]string{{"\"Processed: %s\\n\""}, {"fixtures/testGoLibrary.go//ProcessData//result"}}},
			},
			"fixtures/testGoLibrary.go//ValidateInput": {
				{"fixtures/testGoLibrary.go//logError", [][]string{{"\"Empty input\""}}},
			},
			"fixtures/testGoLibrary.go//logError": {
				{"fmt//Println", [][]string{{"\"Error:\""}, {"fixtures/testGoLibrary.go//logError//msg"}}},
			},
			"fixtures/testGoLibrary.go//Transform": {
				{"fixtures/testGoLibrary.go//ValidateInput", [][]string{{"fixtures/testGoLibrary.go//Transform//s"}}},
				{"fixtures/testGoLibrary.go//ProcessData", [][]string{{"fixtures/testGoLibrary.go//Transform//s"}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testGoLibrary.go", CallerNamespace: "fixtures/testGoLibrary.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testGoLibrary.go//ProcessData", CallerNamespace: "fixtures/testGoLibrary.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testGoLibrary.go//ValidateInput", CallerNamespace: "fixtures/testGoLibrary.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testGoLibrary.go//logError", CallerNamespace: "fixtures/testGoLibrary.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testGoLibrary.go//Transform", CallerNamespace: "fixtures/testGoLibrary.go", CallerIdentifierContent: ""},
			{Namespace: "strings//ToUpper", CallerNamespace: "fixtures/testGoLibrary.go//ProcessData", CallerIdentifierContent: "strings.ToUpper"},
			{Namespace: "fmt//Printf", CallerNamespace: "fixtures/testGoLibrary.go//ProcessData", CallerIdentifierContent: "fmt.Printf"},
			{Namespace: "fixtures/testGoLibrary.go//logError", CallerNamespace: "fixtures/testGoLibrary.go//ValidateInput", CallerIdentifierContent: "logError"},
			{Namespace: "fmt//Println", CallerNamespace: "fixtures/testGoLibrary.go//logError", CallerIdentifierContent: "fmt.Println"},
			{Namespace: "fixtures/testGoLibrary.go//ValidateInput", CallerNamespace: "fixtures/testGoLibrary.go//Transform", CallerIdentifierContent: "ValidateInput"},
			{Namespace: "fixtures/testGoLibrary.go//ProcessData", CallerNamespace: "fixtures/testGoLibrary.go//Transform", CallerIdentifierContent: "ProcessData"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "fixtures/testGoNestedImports.go",
		ExpectedAssignmentGraph: map[string][]string{
			"json":     {"encoding//json"},
			"http":     {"net//http"},
			"filepath": {"path//filepath"},
			"ioutil":   {"io//ioutil"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testGoNestedImports.go": {
				{"fixtures/testGoNestedImports.go//makeHTTPRequest", [][]string{}},
				{"fixtures/testGoNestedImports.go//parseJSON", [][]string{}},
				{"fixtures/testGoNestedImports.go//processPath", [][]string{}},
				{"fixtures/testGoNestedImports.go//readConfig", [][]string{}},
				{"fixtures/testGoNestedImports.go//fetchAndParse", [][]string{}},
				{"fixtures/testGoNestedImports.go//main", [][]string{}},
			},
			"fixtures/testGoNestedImports.go//makeHTTPRequest": {
				{"net//http//Get", [][]string{{"\"https://api.example.com/data\""}}},
			},
			"fixtures/testGoNestedImports.go//parseJSON": {
				{"encoding//json//Unmarshal", [][]string{
					{"fixtures/testGoNestedImports.go//parseJSON//data"},
					{"fixtures/testGoNestedImports.go//parseJSON//result"},
				}},
			},
			"fixtures/testGoNestedImports.go//processPath": {
				{"path//filepath//Join", [][]string{
					{"fixtures/testGoNestedImports.go//processPath//dir"},
					{"fixtures/testGoNestedImports.go//processPath//file"},
				}},
			},
			"fixtures/testGoNestedImports.go//readConfig": {
				{"io//ioutil//ReadFile", [][]string{{"fixtures/testGoNestedImports.go//readConfig//filename"}}},
			},
			"fixtures/testGoNestedImports.go//fetchAndParse": {
				{"net//http//Get", [][]string{{"\"https://api.example.com/config\""}}},
				{"io//ioutil//ReadAll", [][]string{{}}},
				{"fixtures/testGoNestedImports.go//parseJSON", [][]string{{"fixtures/testGoNestedImports.go//fetchAndParse//data"}}},
				{"path//filepath//Join", [][]string{{"\"/etc\""}, {"\"app.conf\""}}},
				{"encoding//json//Marshal", [][]string{{"fixtures/testGoNestedImports.go//fetchAndParse//config"}}},
				{"net//http//NewRequest", [][]string{{"\"POST\""}, {"\"/api\""}, {}}},
			},
			"fixtures/testGoNestedImports.go//main": {
				{"fixtures/testGoNestedImports.go//makeHTTPRequest", [][]string{}},
				{"fixtures/testGoNestedImports.go//readConfig", [][]string{{"\"config.json\""}}},
				{"fixtures/testGoNestedImports.go//parseJSON", [][]string{{"fixtures/testGoNestedImports.go//main//data"}}},
				{"fixtures/testGoNestedImports.go//processPath", [][]string{{"\"/tmp\""}, {"\"test.txt\""}}},
				{"fixtures/testGoNestedImports.go//fetchAndParse", [][]string{}},
				{"net//http//Head", [][]string{{"\"https://example.com\""}}},
				{"encoding//json//Valid", [][]string{{"fixtures/testGoNestedImports.go//main//result"}}},
				{"path//filepath//Abs", [][]string{{"\"/tmp\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testGoNestedImports.go", CallerNamespace: "fixtures/testGoNestedImports.go", CallerIdentifierContent: ""},
			{Namespace: "fixtures/testGoNestedImports.go//main", CallerNamespace: "fixtures/testGoNestedImports.go", CallerIdentifierContent: ""},
			{Namespace: "net//http//Get", CallerNamespace: "fixtures/testGoNestedImports.go//makeHTTPRequest", CallerIdentifierContent: "http.Get"},
			{Namespace: "encoding//json//Unmarshal", CallerNamespace: "fixtures/testGoNestedImports.go//parseJSON", CallerIdentifierContent: "json.Unmarshal"},
			{Namespace: "path//filepath//Join", CallerNamespace: "fixtures/testGoNestedImports.go//processPath", CallerIdentifierContent: "filepath.Join"},
			{Namespace: "io//ioutil//ReadFile", CallerNamespace: "fixtures/testGoNestedImports.go//readConfig", CallerIdentifierContent: "ioutil.ReadFile"},
			{Namespace: "fixtures/testGoNestedImports.go//makeHTTPRequest", CallerNamespace: "fixtures/testGoNestedImports.go//main", CallerIdentifierContent: "makeHTTPRequest"},
			{Namespace: "fixtures/testGoNestedImports.go//readConfig", CallerNamespace: "fixtures/testGoNestedImports.go//main", CallerIdentifierContent: "readConfig"},
			{Namespace: "fixtures/testGoNestedImports.go//parseJSON", CallerNamespace: "fixtures/testGoNestedImports.go//main", CallerIdentifierContent: "parseJSON"},
		},
	},
	{
		Language: core.LanguageCodeJavascript,
		FilePath: "fixtures/testJavascript.js",
		ExpectedAssignmentGraph: map[string][]string{
			"fs":       {},
			"axios":    {},
			"readFile": {"fs//promises//readFile"},
			"writeFile": {"fs//promises//writeFile"},
			"log":  {"console//log"},
			"warn": {"console//warn"},
			"fixtures/testJavascript.js//simpleFunction": {},
			"fixtures/testJavascript.js//TestClass":      {},
			"fixtures/testJavascript.js//instance":       {"fixtures/testJavascript.js//TestClass"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"fixtures/testJavascript.js": {
				{"fixtures/testJavascript.js//require", [][]string{}},
				{"fixtures/testJavascript.js//require", [][]string{}},
				{"fixtures/testJavascript.js//TestClass", [][]string{}},
				{"fixtures/testJavascript.js//TestClass//helperMethod", [][]string{}},
				{"fixtures/testJavascript.js//TestClass//deepMethod", [][]string{}},
				{"fixtures/testJavascript.js//simpleFunction", [][]string{}},
				{"fixtures/testJavascript.js//arrowFunc", [][]string{}},
				{"log", [][]string{}},
				{"fs//readFileSync", [][]string{}},
				{"axios//get", [][]string{}},
				{"instance.helperMethod()//toString", [][]string{}},
				{"fixtures/testJavascript.js//TestClass//helperMethod", [][]string{}},
				{"fixtures/testJavascript.js//ClassA", [][]string{}},
				{"fixtures/testJavascript.js//ClassB", [][]string{}},
				{"fixtures/testJavascript.js//ClassA//method1", [][]string{}},
				{"fixtures/testJavascript.js//ClassA//method1", [][]string{}},
				{"fixtures/testJavascript.js//ClassA//method2", [][]string{}},
				{"fixtures/testJavascript.js//ClassA//methodUnique", [][]string{}},
			},
			"fixtures/testJavascript.js//simpleFunction": {
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//arrowFunc": {
				{"warn", [][]string{}},
			},
			"fixtures/testJavascript.js//TestClass//constructor": {
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//TestClass//helperMethod": {
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//TestClass//deepMethod": {
				{"fixtures/testJavascript.js//TestClass//this//helperMethod", [][]string{}},
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//ClassA//method1": {
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//ClassA//method2": {
				{"warn", [][]string{}},
			},
			"fixtures/testJavascript.js//ClassB//method1": {
				{"log", [][]string{}},
			},
			"fixtures/testJavascript.js//ClassB//method2": {
				{"warn", [][]string{}},
			},
			"fixtures/testJavascript.js//ClassB//methodUnique": {
				{"log", [][]string{}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "fixtures/testJavascript.js//simpleFunction", CallerNamespace: "fixtures/testJavascript.js", CallerIdentifierContent: "simpleFunction"},
			{Namespace: "fixtures/testJavascript.js//arrowFunc", CallerNamespace: "fixtures/testJavascript.js", CallerIdentifierContent: "arrowFunc"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js", CallerIdentifierContent: "log"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js//simpleFunction", CallerIdentifierContent: "log"},
			{Namespace: "console//warn", CallerNamespace: "fixtures/testJavascript.js//arrowFunc", CallerIdentifierContent: "warn"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js//TestClass//constructor", CallerIdentifierContent: "log"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js//TestClass//helperMethod", CallerIdentifierContent: "log"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js//TestClass//deepMethod", CallerIdentifierContent: "log"},
			{Namespace: "console//log", CallerNamespace: "fixtures/testJavascript.js//ClassA//method1", CallerIdentifierContent: "log"},
			{Namespace: "console//warn", CallerNamespace: "fixtures/testJavascript.js//ClassA//method2", CallerIdentifierContent: "warn"},
			{Namespace: "fs//readFileSync", CallerNamespace: "fixtures/testJavascript.js", CallerIdentifierContent: "fs.readFileSync"},
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

func assertCallGraph(t *testing.T, cg *CallGraph, expectedCallGraph map[string][]expectedCallgraphRefs) {
	for sourceNamespace, expectedCallRefs := range expectedCallGraph {
		sourceNode, exists := cg.Nodes[sourceNamespace]
		assert.True(t, exists, "Expected source node %s to exist in call graph", sourceNamespace)
		assert.NotNil(t, sourceNode, "Expected source node %s to be non-nil", sourceNamespace)
		if sourceNode == nil {
			continue
		}

		assert.Equal(t, sourceNamespace, sourceNode.Namespace)

		actualCallgraphRefs := []expectedCallgraphRefs{}
		for _, call := range sourceNode.CallsTo {
			arguments := [][]string{}
			for _, arg := range call.Arguments {
				argResolutions := []string{}
				for _, argResolution := range arg.Nodes {
					argResolutions = append(argResolutions, argResolution.Namespace)
				}
				arguments = append(arguments, argResolutions)
			}
			actualCallgraphRefs = append(actualCallgraphRefs, expectedCallgraphRefs{
				Namespace: call.CalleeNamespace,
				Args:      arguments,
			})
		}

		assert.ElementsMatch(t, expectedCallRefs, actualCallgraphRefs, "Expected callgraph refs for source node %s to match", sourceNamespace)
	}
}

func assertDfs(t *testing.T, cg *CallGraph, expectedDfsResults []dfsResultExpectation, treeData *[]byte) {
	dfsResults := cg.DFS()

	actualDfsItems := make(map[dfsResultExpectation]int)
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

		key := dfsResultExpectation{
			Namespace:               namespace,
			CallerNamespace:         callerNamespace,
			CallerIdentifierContent: callerIdentifierContent,
		}
		actualDfsItems[key]++
	}

	expectedDfsItems := make(map[dfsResultExpectation]int)
	for _, expectedItem := range expectedDfsResults {
		expectedDfsItems[expectedItem]++
	}

	// Ensure expectedDfsItems are present in actualDfsItems
	for expectedItem, expectedItemCount := range expectedDfsItems {
		actualCount, found := actualDfsItems[expectedItem]
		assert.True(t, found, "Expected DFS result item %v to be present in results", expectedItem)
		assert.GreaterOrEqual(t, actualCount, expectedItemCount, "Expected DFS result item %v to have at least %d occurrences, found %d", expectedItem, expectedItemCount, actualCount)
	}
}
