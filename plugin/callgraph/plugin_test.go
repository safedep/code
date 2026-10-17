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
	Namespace       string
	CallerNamespace string
	Text            string
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
		FilePath: "testClass.py",
		ExpectedAssignmentGraph: map[string][]string{
			"printxyz1":                              {"xyz//printxyz1"},
			"xyz//printxyz1":                         {},
			"printxyz2":                              {"xyz//printxyz2"},
			"xyz//printxyz2":                         {},
			"prt3":                                   {"xyz//printxyz3"},
			"xyz//printxyz3":                         {},
			"pprint":                                 {},
			"testClass.py//TesterClass//__init__":    {},
			"testClass.py//TesterClass//self//name":  {"\"TesterClass name\""},
			"testClass.py//TesterClass//self//value": {"42", "testClass.py//TesterClass//__init__//newValue", "100", "\"default value\""},
			"testClass.py//alice":                    {"testClass.py//TesterClass"},
			"testClass.py//bannername":               {"testClass.py//TesterClass//name"},
			"testClass.py//x":                        {"testClass.py//ClassA", "testClass.py//ClassB"},
			"testClass.py//y":                        {"testClass.py//x"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testClass.py": {
				{"testClass.py//TesterClass", [][]string{{"35"}}},
				{"testClass.py//TesterClass//aboutme", [][]string{}},
				{"testClass.py//ClassA", [][]string{}},
				{"testClass.py//ClassB", [][]string{}},
				{"testClass.py//ClassA//method1", [][]string{}},
				{"testClass.py//ClassA//method1", [][]string{}},
				{"testClass.py//ClassB//method1", [][]string{}},
				{"testClass.py//ClassB//method1", [][]string{}},
				{"testClass.py//ClassA//method2", [][]string{}},
				{"testClass.py//ClassB//method2", [][]string{}},
				{"testClass.py//ClassA//methodUnique", [][]string{}},
				{"testClass.py//ClassB//methodUnique", [][]string{}},
			},
			"testClass.py//TesterClass": {
				{"testClass.py//TesterClass//__init__", [][]string{}},
			},
			"testClass.py//TesterClass//self//__init__": {
				{"testClass.py//TesterClass//__init__", [][]string{}},
			},
			"testClass.py//TesterClass//__init__": {
				{"getenv", [][]string{{"\"USE_TAR\""}}},
			},
			"testClass.py//TesterClass//self//helper_method": {
				{"testClass.py//TesterClass//helper_method", [][]string{}},
			},
			"testClass.py//TesterClass//helper_method": {
				{"print", [][]string{{"\"Called helper_method\""}}},
			},
			"testClass.py//TesterClass//self//deepest_method": {
				{"testClass.py//TesterClass//deepest_method", [][]string{}},
			},
			"testClass.py//TesterClass//deepest_method": {
				{"testClass.py//TesterClass//self//helper_method", [][]string{}},
				{"print", [][]string{{"\"Called deepest_method\""}}},
			},
			"testClass.py//TesterClass//aboutme": {
				{"print", [][]string{{"f\"Name: {self.name}\""}}},
			},
			"testClass.py//ClassA":       {},
			"testClass.py//ClassA//self": {},
			"testClass.py//ClassA//self//method1": {
				{"testClass.py//ClassA//method1", [][]string{}},
			},
			"testClass.py//ClassA//method1": {
				{"printxyz2", [][]string{{"\"GG\""}}},
			},
			"testClass.py//ClassA//self//method2": {
				{"testClass.py//ClassA//method2", [][]string{}},
			},
			"testClass.py//ClassA//method2": {
				{"printxyz2", [][]string{{"\"GG\""}}},
			},
			"testClass.py//ClassB//self//methodUnique": {
				{"testClass.py//ClassB//methodUnique", [][]string{}},
			},
			"testClass.py//ClassB//methodUnique": {
				{"prt3", [][]string{{"\"GG\""}}},
				{"pprint//pp", [][]string{{"\"GG\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testClass.py", CallerNamespace: "testClass.py", Text: ""},
			{Namespace: "testClass.py//TesterClass//__init__", CallerNamespace: "testClass.py//TesterClass", Text: ""},
			{Namespace: "getenv", CallerNamespace: "testClass.py//TesterClass//__init__", Text: "getenv"},
			{Namespace: "os//getenv", CallerNamespace: "testClass.py//TesterClass//__init__", Text: "getenv"},
			{Namespace: "print", CallerNamespace: "testClass.py//TesterClass//aboutme", Text: "print"},
			{Namespace: "print", CallerNamespace: "testClass.py//TesterClass//deepest_method", Text: "print"},
			{Namespace: "print", CallerNamespace: "testClass.py//TesterClass//helper_method", Text: "print"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "testClass.py//ClassA//method1", Text: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "testClass.py//ClassA//method2", Text: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "testClass.py//ClassB//method1", Text: "printxyz2"},
			{Namespace: "xyz//printxyz2", CallerNamespace: "testClass.py//ClassB//method2", Text: "printxyz2"},
			{Namespace: "xyz//printxyz3", CallerNamespace: "testClass.py//ClassB//methodUnique", Text: "prt3"},
			{Namespace: "testClass.py//TesterClass", CallerNamespace: "testClass.py", Text: "TesterClass"},
			{Namespace: "testClass.py//TesterClass//aboutme", CallerNamespace: "testClass.py", Text: "alice.aboutme"},
			{Namespace: "testClass.py//ClassA", CallerNamespace: "testClass.py", Text: "ClassA"},
			{Namespace: "testClass.py//ClassA//method1", CallerNamespace: "testClass.py", Text: "x.method1"},
			{Namespace: "testClass.py//ClassA//method1", CallerNamespace: "testClass.py", Text: "y.method1"},
			{Namespace: "testClass.py//ClassA//method2", CallerNamespace: "testClass.py", Text: "y.method2"},
			{Namespace: "testClass.py//ClassB//methodUnique", CallerNamespace: "testClass.py", Text: "y.methodUnique"},
			{Namespace: "testClass.py//TesterClass//deepest_method", CallerNamespace: "testClass.py", Text: ""},
			{Namespace: "testClass.py//TesterClass//self//helper_method", CallerNamespace: "testClass.py//TesterClass//deepest_method", Text: "self.helper_method"},
			{Namespace: "testClass.py//TesterClass//helper_method", CallerNamespace: "testClass.py//TesterClass//self//helper_method", Text: ""},
		},
	},
	{
		Language: core.LanguageCodePython,
		FilePath: "testFunctions.py",
		ExpectedAssignmentGraph: map[string][]string{
			"listdirfn":             {"os//listdir"},
			"printer2":              {"xyzprintmodule//printer2"},
			"printer3":              {"xyzprintmodule//printer3"},
			"printer4":              {"xyzprintmodule//printer4"},
			"SOME_CONSTANT":         {"mypkg//SOME_CONSTANT"},
			"testFunctions.py//baz": {"testFunctions.py//bar"},
			"testFunctions.py//xyz": {
				"\"abc\"",
				"25",
				"testFunctions.py//foo",
				"testFunctions.py//baz",
			},
			"testFunctions.py//r1": {"95", "7.3", "2"},
			"testFunctions.py//p1": {"599", "\"going good\"", "39.2"},
			"testFunctions.py//p2": {"95", "True"},
			"testFunctions.py//p3": {"\"gg\""},
			"testFunctions.py//res": {
				"testFunctions.py//r1",
				"testFunctions.py//somenumber",
				"95",
				"7.3",
				"pstats//getsomestat",
			},
			"testFunctions.py//mul": {
				"testFunctions.py//multiply",
			},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testFunctions.py": {
				{"testFunctions.py//factorial", [][]string{{"5"}}},
				{"print", [][]string{{}}},
				{"testFunctions.py//xyz", [][]string{}},
				{"testFunctions.py//nestParent", [][]string{}},
				{"random//randint", [][]string{{"0"}, {"1"}}},
				{"testFunctions.py//add", [][]string{{"599", "\"going good\"", "39.2"}, {"95", "True"}}},
				{"testFunctions.py//sub", [][]string{{"\"gg\""}, {"6"}}},
				{"pstats//getsomestat", [][]string{}},
				{"testFunctions.py//mul", [][]string{{"2"}, {"3"}}},
				{"testFunctions.py//addProxy", [][]string{{"5"}, {"3"}}},
				{"testFunctions.py//concat", [][]string{{"\"Hello, \""}, {"\"World!\""}}},
				{"testFunctions.py//add", [][]string{{"599", "\"going good\"", "39.2"}, {"\"gg\""}}},
				{"testFunctions.py//add", [][]string{{"95", "True"}, {}}},
				{"getenv", [][]string{{"\"SOME_ENV_VAR\""}}},
				{"print", [][]string{{"\"gg\""}, {"1"}, {"2.5"}, {"True"}, {"None"}, {"mypkg//SOME_CONSTANT"}, {}, {}, {}, {}, {}, {}, {"os//getenv"}}},
			},
			"testFunctions.py//factorial": {
				{
					"testFunctions.py//factorial", [][]string{
						{"testFunctions.py//factorial//x", "1"},
					},
				},
			},
			"testFunctions.py//foo": {
				{"pprint//pprint", [][]string{{"\"foo\""}}},
			},
			"testFunctions.py//bar": {
				{"print", [][]string{{"\"bar\""}}},
			},
			"testFunctions.py//outerfn1": {
				{"chmod", [][]string{{"\"outerfn1\""}}},
			},
			"testFunctions.py//outerfn2": {
				{"listdirfn", [][]string{{"\"outerfn2\""}}},
			},
			"testFunctions.py//fn1": {
				{"printer4", [][]string{{"\"outer fn1\""}}},
			},
			"testFunctions.py//nestParent": {
				{"testFunctions.py//outerfn1", [][]string{}},
				{"testFunctions.py//nestParent//nestChild", [][]string{}},
			},
			"testFunctions.py//nestParent//parentScopedFn": {
				{"print", [][]string{{"\"parentScopedFn\""}}},
				{"testFunctions.py//fn1", [][]string{}},
			},
			"testFunctions.py//nestParent//nestChild": {
				{"printer1", [][]string{{"\"nestChild\""}}},
				{"testFunctions.py//outerfn1", [][]string{}},
				{"testFunctions.py//nestParent//nestChild//nestGrandChild", [][]string{}},
			},
			"testFunctions.py//nestParent//nestChild//fn1": {
				{"printer6", [][]string{{"\"inner fn1\""}}},
			},
			"testFunctions.py//nestParent//nestChild//childScopedFn": {
				{"printer2", [][]string{{"\"childScopedFn\""}}},
				{"testFunctions.py//nestParent//nestChild//fn1", [][]string{}},
			},
			"testFunctions.py//nestParent//nestChild//nestGrandChildUseless": {
				{"printer3", [][]string{{"\"nestGrandChildUseless\""}}},
			},
			"testFunctions.py//nestParent//nestChild//nestGrandChild": {
				{"pprint//pp", [][]string{{"\"nestGrandChild\""}}},
				{"testFunctions.py//nestParent//parentScopedFn", [][]string{}},
				{"testFunctions.py//outerfn2", [][]string{}},
				{"testFunctions.py//nestParent//nestChild//childScopedFn", [][]string{}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testFunctions.py", CallerNamespace: "testFunctions.py", Text: ""},
			{Namespace: "testFunctions.py//factorial", CallerNamespace: "testFunctions.py", Text: "factorial"},
			{Namespace: "testFunctions.py//factorial", CallerNamespace: "testFunctions.py//factorial", Text: "factorial"},
			{Namespace: "testFunctions.py//foo", CallerNamespace: "testFunctions.py", Text: "xyz"},
			{Namespace: "testFunctions.py//bar", CallerNamespace: "testFunctions.py", Text: "xyz"},
			{Namespace: "os//chmod", CallerNamespace: "testFunctions.py//outerfn1", Text: "chmod"},
			{Namespace: "os//listdir", CallerNamespace: "testFunctions.py//outerfn2", Text: "listdirfn"},
			{Namespace: "xyzprintmodule//printer4", CallerNamespace: "testFunctions.py//fn1", Text: "printer4"},
			{Namespace: "pprint//pp", CallerNamespace: "testFunctions.py//nestParent//nestChild//nestGrandChild", Text: "pprint.pp"},
			{Namespace: "pprint//pprint", CallerNamespace: "testFunctions.py//foo", Text: "pprint.pprint"},
			{Namespace: "testFunctions.py//nestParent//nestChild", CallerNamespace: "testFunctions.py//nestParent", Text: "nestChild"},
			{Namespace: "testFunctions.py//nestParent//nestChild//childScopedFn", CallerNamespace: "testFunctions.py//nestParent//nestChild//nestGrandChild", Text: "childScopedFn"},
			{Namespace: "testFunctions.py//nestParent//nestChild//nestGrandChild", CallerNamespace: "testFunctions.py//nestParent//nestChild", Text: "nestGrandChild"},
			{Namespace: "testFunctions.py//add", CallerNamespace: "testFunctions.py", Text: "add"},
			{Namespace: "testFunctions.py//sub", CallerNamespace: "testFunctions.py", Text: "sub"},
			{Namespace: "random//randint", CallerNamespace: "testFunctions.py", Text: "random.randint"},
			{Namespace: "testFunctions.py//mul", CallerNamespace: "testFunctions.py", Text: "mul"},
			{Namespace: "testFunctions.py//addProxy", CallerNamespace: "testFunctions.py", Text: "addProxy"},
			{Namespace: "testFunctions.py//add", CallerNamespace: "testFunctions.py", Text: "add"},
			{Namespace: "testFunctions.py//concat", CallerNamespace: "testFunctions.py", Text: "concat"},
		},
	},
	{
		Language: core.LanguageCodeJava,
		FilePath: "CallgraphTestcases.java",
		ExpectedAssignmentGraph: map[string][]string{
			"Dialog":        {"java//awt//Dialog"},
			"Frame":         {"java//awt//Frame"},
			"GridLayout":    {"java//awt//GridLayout"},
			"ScrollPane":    {"java//awt//ScrollPane"},
			"LayoutManager": {"java//awt//LayoutManager"},
			"Window":        {"java//awt//Window"},
			"somelibfn":     {"somelib//xyz//somelibfn"},
			"MailcapFile":   {"com//sun//activation//registries//MailcapFile"},
			"CallgraphTestcases.java//UtilityClass//somelibfn//x": {"5"},
			"CallgraphTestcases.java//CallgraphTestcases//i": {
				"5",
				"7",
				"1",
				"2",
				"59",
			},
			"CallgraphTestcases.java//CallgraphTestcases//s": {
				"\"GG\"",
				"\"HH\"",
				"\"ii\"",
			},
			"CallgraphTestcases.java//CallgraphTestcases": {},
			"CallgraphTestcases.java//CallgraphTestcases//main//dg": {
				"Dialog",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//cnv": {
				"java//awt//Canvas",
				"ScrollPane",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//width": {
				"32",
				"64",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//ht": {
				"99",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//lm": {
				"java//awt//BorderLayout",
				"java//awt//FlowLayout",
				"GridLayout",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//componentName": {
				"\"North\"",
				"\"South\"",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//worker": {
				"java//awt//SomeLayoutWorker",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//util": {
				"CallgraphTestcases.java//UtilityClass",
			},
			"CallgraphTestcases.java//CallgraphTestcases//main//result": {"938"},
			"CallgraphTestcases.java//CallgraphTestcases//main//obj": {
				"org//mycompany//mylib//SomeClass",
			},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"CallgraphTestcases.java//UtilityClass": {
				{"System//out//println", [][]string{
					{"CallgraphTestcases.java//UtilityClass//x"},
				}},
			},
			"CallgraphTestcases.java//UtilityClass//somelibfn": {
				{"Math//random", [][]string{}},
			},
			"CallgraphTestcases.java": {
				{"CallgraphTestcases.java//CallgraphTestcases//main", [][]string{}},
			},
			"CallgraphTestcases.java//CallgraphTestcases": {
				{"com//custompkg//SomeClass//defaultConstructor", [][]string{}},
				{"com//custompkg//SomeClass//someMethod", [][]string{{"5", "7"}}},
				{"com//custompkg//SomeClass//someOtherMethod", [][]string{
					{"5", "7", "1", "2", "59"},
					{"\"GG\"", "\"HH\"", "\"ii\""},
				}},
			},
			"CallgraphTestcases.java//CallgraphTestcases//this//myfunc": {
				{"CallgraphTestcases.java//CallgraphTestcases//myfunc", [][]string{}},
			},
			"CallgraphTestcases.java//CallgraphTestcases//myfunc": {
				{"String//valueOf", [][]string{{"'c'"}}},
				{"String//valueOf", [][]string{{"'c'", "59"}}},
			},
			"CallgraphTestcases.java//CallgraphTestcases//main": {
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

				{"CallgraphTestcases.java//UtilityClass", [][]string{{"10"}}},
				{"CallgraphTestcases.java//UtilityClass//somelibfn", [][]string{{"5"}, {"10"}}},
				{"somelibfn", [][]string{}},
				{"CallgraphTestcases.java//CallgraphTestcases//myfunc", [][]string{}},
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
			{Namespace: "CallgraphTestcases.java", CallerNamespace: "CallgraphTestcases.java", Text: ""},
			{Namespace: "CallgraphTestcases.java//CallgraphTestcases//main", CallerNamespace: "CallgraphTestcases.java", Text: ""},

			{Namespace: "Frame", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Frame"},
			{Namespace: "java//awt//Frame", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Frame"},
			{Namespace: "Window", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Window"},
			{Namespace: "java//awt//Window", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Window"},
			{Namespace: "Dialog", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Dialog"},
			{Namespace: "java//awt//Dialog", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Dialog"},
			{Namespace: "java//awt//Dialog//setTitle", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "dg.setTitle"},
			{Namespace: "java//awt//Dialog//prop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "dg.prop.getSomething"},
			{Namespace: "java//awt//Canvas", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.Canvas"},
			{Namespace: "java//awt//ScrollPane", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "ScrollPane"},
			{Namespace: "java//awt//Canvas//getWidth", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.getWidth"},
			{Namespace: "java//awt//ScrollPane//getWidth", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.getWidth"},
			{Namespace: "Math//random", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Math.random"},
			{Namespace: "java//awt//Canvas//setSize", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.setSize"},
			{Namespace: "java//awt//ScrollPane//setSize", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.setSize"},
			{Namespace: "java//awt//Canvas//prop//subprop//subsubprop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.prop.subprop.subsubprop.getSomething"},
			{Namespace: "java//awt//ScrollPane//prop//subprop//subsubprop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "cnv.prop.subprop.subsubprop.getSomething"},

			{Namespace: "java//awt//BorderLayout", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.BorderLayout"},
			{Namespace: "java//awt//Button", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.Button"},
			{Namespace: "java//awt//BorderLayout//addLayoutComponent", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.addLayoutComponent"},
			{Namespace: "java//awt//FlowLayout", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.FlowLayout"},
			{Namespace: "java//awt//Container", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.Container"},
			{Namespace: "java//awt//BorderLayout//minimumLayoutSize", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.minimumLayoutSize"},
			{Namespace: "java//awt//FlowLayout//minimumLayoutSize", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.minimumLayoutSize"},
			{Namespace: "java//awt//GridLayout", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "GridLayout"},
			{Namespace: "java//awt//BorderLayout//toString", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.toString"},
			{Namespace: "java//awt//FlowLayout//toString", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.toString"},
			{Namespace: "java//awt//GridLayout//toString", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.toString"},
			{Namespace: "java//awt//BorderLayout//prop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.prop.getSomething"},
			{Namespace: "java//awt//FlowLayout//prop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.prop.getSomething"},
			{Namespace: "java//awt//GridLayout//prop//getSomething", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "lm.prop.getSomething"},

			{Namespace: "java//awt//SomeLayoutWorker", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "java.awt.SomeLayoutWorker"},

			{Namespace: "CallgraphTestcases.java//UtilityClass", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "UtilityClass"},
			{Namespace: "System//out//println", CallerNamespace: "CallgraphTestcases.java//UtilityClass", Text: "System.out.println"},
			{Namespace: "CallgraphTestcases.java//UtilityClass//somelibfn", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "util.somelibfn"},

			{Namespace: "somelibfn", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "somelibfn"},
			{Namespace: "somelib//xyz//somelibfn", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "somelibfn"},
			{Namespace: "CallgraphTestcases.java//CallgraphTestcases//myfunc", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "myfunc"},
			{Namespace: "String//valueOf", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//myfunc", Text: "String.valueOf"},
			{Namespace: "String//valueOf", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//myfunc", Text: "String.valueOf"},
			{Namespace: "System//out//println", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "System.out.println"},
			{Namespace: "System//out//xyz//println", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "System.out.xyz.println"},

			{Namespace: "System//console", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "System.console().readPassword"},
			{Namespace: "com//companyX//fn1", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "com.companyX.fn1"},
			{Namespace: "System//getenv", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "System.getenv().keySet().iterator(com.companyX.fn1()).hasNext"},
			{Namespace: "Math//atan", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "Math.atan"},

			{Namespace: "com//somecompany//customlib//datatransfer//DataTransferer//getInstance", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "com.somecompany.customlib.datatransfer.DataTransferer.getInstance"},

			{Namespace: "org//mycompany//mylib//SomeClass", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "org.mycompany.mylib.SomeClass"},
			{Namespace: "org//mycompany//mylib//SomeClass//prop//someMethod", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases//main", Text: "obj.prop.someMethod"},

			{Namespace: "com//custompkg//SomeClass//defaultConstructor", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases", Text: "com.custompkg.SomeClass.defaultConstructor"},
			{Namespace: "com//custompkg//SomeClass//someMethod", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases", Text: "com.custompkg.SomeClass.someMethod"},
			{Namespace: "com//custompkg//SomeClass//someOtherMethod", CallerNamespace: "CallgraphTestcases.java//CallgraphTestcases", Text: "com.custompkg.SomeClass.someOtherMethod"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "testCallGraph.go",
		ExpectedAssignmentGraph: map[string][]string{
			"fmt": {},
			"os":  {},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testCallGraph.go": {
				{"testCallGraph.go//helper", [][]string{}},
				{"testCallGraph.go//processFile", [][]string{}},
				{"testCallGraph.go//main", [][]string{}},
			},
			"testCallGraph.go//main": {
				{"testCallGraph.go//helper", [][]string{{"10"}}},
				{"testCallGraph.go//processFile", [][]string{{"\"test.txt\""}}},
				{"os//Getenv", [][]string{{"\"HOME\""}}},
				{"fmt//Sprintf", [][]string{{"\"formatted: %v\""}, {"123"}}},
				{"fmt//Println", [][]string{{"testCallGraph.go//main//result"}}},
			},
			"testCallGraph.go//helper": {
				{"fmt//Println", [][]string{{"\"helper called\""}}},
			},
			"testCallGraph.go//processFile": {
				{"os//WriteFile", [][]string{
					{"testCallGraph.go//processFile//filename"},
					{"testCallGraph.go//processFile//data"},
					{"0644"},
				}},
				{"fmt//Printf", [][]string{
					{"\"Wrote file: %s\\n\""},
					{"testCallGraph.go//processFile//filename"},
				}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testCallGraph.go", CallerNamespace: "testCallGraph.go", Text: ""},
			{Namespace: "testCallGraph.go//helper", CallerNamespace: "testCallGraph.go", Text: ""},
			{Namespace: "testCallGraph.go//processFile", CallerNamespace: "testCallGraph.go", Text: ""},
			{Namespace: "testCallGraph.go//main", CallerNamespace: "testCallGraph.go", Text: ""},
			{Namespace: "testCallGraph.go//helper", CallerNamespace: "testCallGraph.go//main", Text: "helper"},
			{Namespace: "testCallGraph.go//processFile", CallerNamespace: "testCallGraph.go//main", Text: "processFile"},
			{Namespace: "fmt//Println", CallerNamespace: "testCallGraph.go//helper", Text: "fmt.Println"},
			{Namespace: "os//WriteFile", CallerNamespace: "testCallGraph.go//processFile", Text: "os.WriteFile"},
			{Namespace: "fmt//Printf", CallerNamespace: "testCallGraph.go//processFile", Text: "fmt.Printf"},
			{Namespace: "os//Getenv", CallerNamespace: "testCallGraph.go//main", Text: "os.Getenv"},
			{Namespace: "fmt//Sprintf", CallerNamespace: "testCallGraph.go//main", Text: "fmt.Sprintf"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "testGoLibrary.go",
		ExpectedAssignmentGraph: map[string][]string{
			"fmt":     {},
			"strings": {},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testGoLibrary.go": {
				{"testGoLibrary.go//ProcessData", [][]string{}},
				{"testGoLibrary.go//ValidateInput", [][]string{}},
				{"testGoLibrary.go//logError", [][]string{}},
				{"testGoLibrary.go//Transform", [][]string{}},
			},
			"testGoLibrary.go//ProcessData": {
				{"strings//ToUpper", [][]string{{"testGoLibrary.go//ProcessData//data"}}},
				{"fmt//Printf", [][]string{{"\"Processed: %s\\n\""}, {"testGoLibrary.go//ProcessData//result"}}},
			},
			"testGoLibrary.go//ValidateInput": {
				{"testGoLibrary.go//logError", [][]string{{"\"Empty input\""}}},
			},
			"testGoLibrary.go//logError": {
				{"fmt//Println", [][]string{{"\"Error:\""}, {"testGoLibrary.go//logError//msg"}}},
			},
			"testGoLibrary.go//Transform": {
				{"testGoLibrary.go//ValidateInput", [][]string{{"testGoLibrary.go//Transform//s"}}},
				{"testGoLibrary.go//ProcessData", [][]string{{"testGoLibrary.go//Transform//s"}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testGoLibrary.go", CallerNamespace: "testGoLibrary.go", Text: ""},
			{Namespace: "testGoLibrary.go//ProcessData", CallerNamespace: "testGoLibrary.go", Text: ""},
			{Namespace: "testGoLibrary.go//ValidateInput", CallerNamespace: "testGoLibrary.go", Text: ""},
			{Namespace: "testGoLibrary.go//logError", CallerNamespace: "testGoLibrary.go", Text: ""},
			{Namespace: "testGoLibrary.go//Transform", CallerNamespace: "testGoLibrary.go", Text: ""},
			{Namespace: "strings//ToUpper", CallerNamespace: "testGoLibrary.go//ProcessData", Text: "strings.ToUpper"},
			{Namespace: "fmt//Printf", CallerNamespace: "testGoLibrary.go//ProcessData", Text: "fmt.Printf"},
			{Namespace: "testGoLibrary.go//logError", CallerNamespace: "testGoLibrary.go//ValidateInput", Text: "logError"},
			{Namespace: "fmt//Println", CallerNamespace: "testGoLibrary.go//logError", Text: "fmt.Println"},
			{Namespace: "testGoLibrary.go//ValidateInput", CallerNamespace: "testGoLibrary.go//Transform", Text: "ValidateInput"},
			{Namespace: "testGoLibrary.go//ProcessData", CallerNamespace: "testGoLibrary.go//Transform", Text: "ProcessData"},
		},
	},
	{
		Language: core.LanguageCodeGo,
		FilePath: "testGoNestedImports.go",
		ExpectedAssignmentGraph: map[string][]string{
			"json":     {"encoding//json"},
			"http":     {"net//http"},
			"filepath": {"path//filepath"},
			"ioutil":   {"io//ioutil"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testGoNestedImports.go": {
				{"testGoNestedImports.go//makeHTTPRequest", [][]string{}},
				{"testGoNestedImports.go//parseJSON", [][]string{}},
				{"testGoNestedImports.go//processPath", [][]string{}},
				{"testGoNestedImports.go//readConfig", [][]string{}},
				{"testGoNestedImports.go//fetchAndParse", [][]string{}},
				{"testGoNestedImports.go//main", [][]string{}},
			},
			"testGoNestedImports.go//makeHTTPRequest": {
				{"net//http//Get", [][]string{{"\"https://api.example.com/data\""}}},
			},
			"testGoNestedImports.go//parseJSON": {
				{"encoding//json//Unmarshal", [][]string{
					{"testGoNestedImports.go//parseJSON//data"},
					{"testGoNestedImports.go//parseJSON//result"},
				}},
			},
			"testGoNestedImports.go//processPath": {
				{"path//filepath//Join", [][]string{
					{"testGoNestedImports.go//processPath//dir"},
					{"testGoNestedImports.go//processPath//file"},
				}},
			},
			"testGoNestedImports.go//readConfig": {
				{"io//ioutil//ReadFile", [][]string{{"testGoNestedImports.go//readConfig//filename"}}},
			},
			"testGoNestedImports.go//fetchAndParse": {
				{"net//http//Get", [][]string{{"\"https://api.example.com/config\""}}},
				{"io//ioutil//ReadAll", [][]string{{}}},
				{"testGoNestedImports.go//parseJSON", [][]string{{"testGoNestedImports.go//fetchAndParse//data"}}},
				{"path//filepath//Join", [][]string{{"\"/etc\""}, {"\"app.conf\""}}},
				{"encoding//json//Marshal", [][]string{{"testGoNestedImports.go//fetchAndParse//config"}}},
				{"net//http//NewRequest", [][]string{{"\"POST\""}, {"\"/api\""}, {}}},
			},
			"testGoNestedImports.go//main": {
				{"testGoNestedImports.go//makeHTTPRequest", [][]string{}},
				{"testGoNestedImports.go//readConfig", [][]string{{"\"config.json\""}}},
				{"testGoNestedImports.go//parseJSON", [][]string{{"testGoNestedImports.go//main//data"}}},
				{"testGoNestedImports.go//processPath", [][]string{{"\"/tmp\""}, {"\"test.txt\""}}},
				{"testGoNestedImports.go//fetchAndParse", [][]string{}},
				{"net//http//Head", [][]string{{"\"https://example.com\""}}},
				{"encoding//json//Valid", [][]string{{"testGoNestedImports.go//main//result"}}},
				{"path//filepath//Abs", [][]string{{"\"/tmp\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testGoNestedImports.go", CallerNamespace: "testGoNestedImports.go", Text: ""},
			{Namespace: "testGoNestedImports.go//main", CallerNamespace: "testGoNestedImports.go", Text: ""},
			{Namespace: "net//http//Get", CallerNamespace: "testGoNestedImports.go//makeHTTPRequest", Text: "http.Get"},
			{Namespace: "encoding//json//Unmarshal", CallerNamespace: "testGoNestedImports.go//parseJSON", Text: "json.Unmarshal"},
			{Namespace: "path//filepath//Join", CallerNamespace: "testGoNestedImports.go//processPath", Text: "filepath.Join"},
			{Namespace: "io//ioutil//ReadFile", CallerNamespace: "testGoNestedImports.go//readConfig", Text: "ioutil.ReadFile"},
			{Namespace: "testGoNestedImports.go//makeHTTPRequest", CallerNamespace: "testGoNestedImports.go//main", Text: "makeHTTPRequest"},
			{Namespace: "testGoNestedImports.go//readConfig", CallerNamespace: "testGoNestedImports.go//main", Text: "readConfig"},
			{Namespace: "testGoNestedImports.go//parseJSON", CallerNamespace: "testGoNestedImports.go//main", Text: "parseJSON"},
		},
	},
	{
		Language: core.LanguageCodeJavascript,
		FilePath: "testJavascript.js",
		ExpectedAssignmentGraph: map[string][]string{
			"fs":                                {},
			"axios":                             {},
			"sqlite3":                           {},
			"readFile":                          {"fs//promises//readFile"},
			"writeFile":                         {"fs//promises//writeFile"},
			"log":                               {"console//log"},
			"warn":                              {"console//warn"},
			"testJavascript.js//simpleFunction": {},
			"testJavascript.js//TestClass":      {},
			"testJavascript.js//instance":       {"testJavascript.js//TestClass"},
			"testJavascript.js//db":             {"sqlite3//Database"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			"testJavascript.js": {
				{"require", [][]string{{"'fs'"}}},
				{"require", [][]string{{"'fs/promises'"}}},
				{"require", [][]string{{"'sqlite3'"}}},
				{"testJavascript.js//TestClass", [][]string{{"\"test\""}, {"42"}}},
				{"testJavascript.js//TestClass//helperMethod", [][]string{}},
				{"testJavascript.js//TestClass//deepMethod", [][]string{}},
				{"testJavascript.js//simpleFunction", [][]string{{"1"}, {"2"}}},
				{"testJavascript.js//arrowFunc", [][]string{{"5"}}},
				{"log", [][]string{{"\"Module level call\""}}},
				{"fs//readFileSync", [][]string{{"\"file.txt\""}}},
				{"axios//get", [][]string{{"\"https://example.com\""}}},
				{"testJavascript.js//TestClass//helperMethod", [][]string{}},
				{"testJavascript.js//TestClass//helperMethod//toString", [][]string{}},
				{"testJavascript.js//TestClass//helperMethod", [][]string{}},
				{"testJavascript.js//ClassA", [][]string{}},
				{"testJavascript.js//ClassB", [][]string{}},
				{"testJavascript.js//ClassA//method1", [][]string{}},
				{"testJavascript.js//ClassA//method1", [][]string{}},
				{"testJavascript.js//ClassB//method1", [][]string{}},
				{"testJavascript.js//ClassB//method1", [][]string{}},
				{"testJavascript.js//ClassA//method2", [][]string{}},
				{"testJavascript.js//ClassB//method2", [][]string{}},
				{"testJavascript.js//ClassA//methodUnique", [][]string{}},
				{"testJavascript.js//ClassB//methodUnique", [][]string{}},
				{"sqlite3//Database", [][]string{{"':memory:'"}}},
			},
			"testJavascript.js//simpleFunction": {
				{"log", [][]string{{"\"Simple function called\""}}},
			},
			"testJavascript.js//arrowFunc": {
				{"warn", [][]string{{"\"Arrow function called\""}}},
			},
			"testJavascript.js//TestClass//constructor": {
				{"log", [][]string{{"\"TestClass constructor\""}}},
			},
			"testJavascript.js//TestClass//helperMethod": {
				{"log", [][]string{{"\"Called helper method\""}}},
			},
			"testJavascript.js//TestClass//deepMethod": {
				{"testJavascript.js//TestClass//this//helperMethod", [][]string{}},
				{"log", [][]string{{"\"Called deep method\""}}},
			},
			"testJavascript.js//ClassA//method1": {
				{"log", [][]string{{"\"ClassA method1\""}}},
			},
			"testJavascript.js//ClassA//method2": {
				{"warn", [][]string{{"\"ClassA method2\""}}},
			},
			"testJavascript.js//ClassB//method1": {
				{"log", [][]string{{"\"ClassB method1\""}}},
			},
			"testJavascript.js//ClassB//method2": {
				{"warn", [][]string{{"\"ClassB method2\""}}},
			},
			"testJavascript.js//ClassB//methodUnique": {
				{"log", [][]string{{"\"ClassB unique\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			{Namespace: "testJavascript.js//simpleFunction", CallerNamespace: "testJavascript.js", Text: "simpleFunction"},
			{Namespace: "testJavascript.js//arrowFunc", CallerNamespace: "testJavascript.js", Text: "arrowFunc"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js", Text: "log"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js//simpleFunction", Text: "log"},
			{Namespace: "console//warn", CallerNamespace: "testJavascript.js//arrowFunc", Text: "warn"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js//TestClass//constructor", Text: "log"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js//TestClass//helperMethod", Text: "log"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js//TestClass//deepMethod", Text: "log"},
			{Namespace: "console//log", CallerNamespace: "testJavascript.js//ClassA//method1", Text: "log"},
			{Namespace: "console//warn", CallerNamespace: "testJavascript.js//ClassA//method2", Text: "warn"},
			{Namespace: "fs//readFileSync", CallerNamespace: "testJavascript.js", Text: "fs.readFileSync"},
		},
	},
	{
		Language: core.LanguageCodeTypescript,
		FilePath: "testTypescript.ts",
		ExpectedAssignmentGraph: map[string][]string{
			// Import aliases
			"axios":      {},
			"log":        {"console//log"},
			"warn":       {"console//warn"},
			"createHash": {"crypto//createHash"},
			"printA1":    {"printers//printA1"},
			"printA2":    {"printers//printA2"},
			"printB1":    {"printers//printB1"},
			"printB2":    {"printers//printB2"},
			"printBU":    {"printers//printBU"},
			// Type-only import: should appear in assignment graph but NOT generate call edges
			"AxiosResponse": {"axios//AxiosResponse"},
			// Class/function definitions
			"testTypescript.ts//simpleFunction": {},
			"testTypescript.ts//TestClass":      {},
			// Instance tracking
			"testTypescript.ts//instance": {"testTypescript.ts//TestClass"},
			// Polymorphic variable tracking
			"testTypescript.ts//x": {"testTypescript.ts//ClassA", "testTypescript.ts//ClassB"},
			"testTypescript.ts//y": {"testTypescript.ts//x"},
			// Constructor this-assignment tracking
			"testTypescript.ts//TestClass//constructor//this.name":  {"testTypescript.ts//TestClass//constructor//name"},
			"testTypescript.ts//TestClass//constructor//this.value": {"testTypescript.ts//TestClass//constructor//value"},
		},
		ExpectedCallGraph: map[string][]expectedCallgraphRefs{
			// Root-level calls
			"testTypescript.ts": {
				{"testTypescript.ts//TestClass", [][]string{{"\"test\""}, {"42"}}},
				{"testTypescript.ts//TestClass//helperMethod", [][]string{}},
				{"testTypescript.ts//TestClass//deepMethod", [][]string{}},
				{"testTypescript.ts//simpleFunction", [][]string{{"1"}, {"2"}}},
				{"testTypescript.ts//arrowFunc", [][]string{{"5"}}},
				{"log", [][]string{{"\"Module level call\""}}},
				{"axios//get", [][]string{{"\"https://example.com\""}}},
				{"createHash", [][]string{{"\"sha256\""}}},
				{"testTypescript.ts//TestClass//helperMethod", [][]string{}},
				{"testTypescript.ts//TestClass//helperMethod//toString", [][]string{}},
				{"testTypescript.ts//TestClass//helperMethod", [][]string{}},
				{"testTypescript.ts//ClassA", [][]string{}},
				{"testTypescript.ts//ClassB", [][]string{}},
				// Polymorphic dispatch: x/y resolve to ClassA, so method calls go to ClassA
				{"testTypescript.ts//ClassA//method1", [][]string{}},
				{"testTypescript.ts//ClassA//method1", [][]string{}},
				{"testTypescript.ts//ClassB//method1", [][]string{}},
				{"testTypescript.ts//ClassB//method1", [][]string{}},
				{"testTypescript.ts//ClassA//method2", [][]string{}},
				{"testTypescript.ts//ClassB//method2", [][]string{}},
				{"testTypescript.ts//ClassA//methodUnique", [][]string{}},
				{"testTypescript.ts//ClassB//methodUnique", [][]string{}},
			},
			// Function internals
			"testTypescript.ts//simpleFunction": {
				{"log", [][]string{{"\"Simple function called\""}}},
			},
			"testTypescript.ts//arrowFunc": {
				{"warn", [][]string{{"\"Arrow function called\""}}},
			},
			// Class constructor dispatches to constructor method
			"testTypescript.ts//TestClass": {
				{"testTypescript.ts//TestClass//constructor", [][]string{}},
			},
			// Constructor calls log
			"testTypescript.ts//TestClass//constructor": {
				{"log", [][]string{{"\"TestClass constructor\""}}},
			},
			"testTypescript.ts//TestClass//helperMethod": {
				{"log", [][]string{{"\"Called helper method\""}}},
			},
			// deepMethod calls this.helperMethod() and log
			"testTypescript.ts//TestClass//deepMethod": {
				{"testTypescript.ts//TestClass//this//helperMethod", [][]string{}},
				{"log", [][]string{{"\"Called deep method\""}}},
			},
			// print method uses this.name as argument
			"testTypescript.ts//TestClass//print": {
				{"log", [][]string{{"testTypescript.ts//TestClass//this//name"}}},
			},
			// ClassA methods call distinct imported functions
			"testTypescript.ts//ClassA//method1": {
				{"printA1", [][]string{{"\"ClassA method1\""}}},
			},
			"testTypescript.ts//ClassA//method2": {
				{"printA2", [][]string{{"\"ClassA method2\""}}},
			},
			// ClassB methods call distinct imported functions
			"testTypescript.ts//ClassB//method1": {
				{"printB1", [][]string{{"\"ClassB method1\""}}},
			},
			"testTypescript.ts//ClassB//method2": {
				{"printB2", [][]string{{"\"ClassB method2\""}}},
			},
			"testTypescript.ts//ClassB//methodUnique": {
				{"printBU", [][]string{{"\"ClassB unique\""}}},
			},
		},
		ExpectedDfsResults: []dfsResultExpectation{
			// Constructor chain: new TestClass -> constructor -> log
			{Namespace: "testTypescript.ts//TestClass", CallerNamespace: "testTypescript.ts", Text: "TestClass"},
			{Namespace: "testTypescript.ts//TestClass//constructor", CallerNamespace: "testTypescript.ts//TestClass", Text: ""},
			{Namespace: "console//log", CallerNamespace: "testTypescript.ts//TestClass//constructor", Text: "log"},
			// Method calls on instance
			{Namespace: "testTypescript.ts//TestClass//helperMethod", CallerNamespace: "testTypescript.ts", Text: "instance.helperMethod"},
			{Namespace: "console//log", CallerNamespace: "testTypescript.ts//TestClass//helperMethod", Text: "log"},
			{Namespace: "testTypescript.ts//TestClass//deepMethod", CallerNamespace: "testTypescript.ts", Text: "instance.deepMethod"},
			{Namespace: "testTypescript.ts//TestClass//this//helperMethod", CallerNamespace: "testTypescript.ts//TestClass//deepMethod", Text: "this.helperMethod"},
			{Namespace: "console//log", CallerNamespace: "testTypescript.ts//TestClass//deepMethod", Text: "log"},
			// Module-level function calls
			{Namespace: "testTypescript.ts//simpleFunction", CallerNamespace: "testTypescript.ts", Text: "simpleFunction"},
			{Namespace: "console//log", CallerNamespace: "testTypescript.ts//simpleFunction", Text: "log"},
			{Namespace: "testTypescript.ts//arrowFunc", CallerNamespace: "testTypescript.ts", Text: "arrowFunc"},
			{Namespace: "console//warn", CallerNamespace: "testTypescript.ts//arrowFunc", Text: "warn"},
			// Module-level log call
			{Namespace: "console//log", CallerNamespace: "testTypescript.ts", Text: "log"},
			// Imported module calls
			{Namespace: "axios//get", CallerNamespace: "testTypescript.ts", Text: "axios.get"},
			{Namespace: "crypto//createHash", CallerNamespace: "testTypescript.ts", Text: "createHash"},
			// Polymorphic dispatch: new ClassA/ClassB
			{Namespace: "testTypescript.ts//ClassA", CallerNamespace: "testTypescript.ts", Text: "ClassA"},
			{Namespace: "testTypescript.ts//ClassB", CallerNamespace: "testTypescript.ts", Text: "ClassB"},
			// x.method1()/y.method1() resolve to ClassA methods
			{Namespace: "testTypescript.ts//ClassA//method1", CallerNamespace: "testTypescript.ts", Text: "x.method1"},
			{Namespace: "testTypescript.ts//ClassA//method1", CallerNamespace: "testTypescript.ts", Text: "y.method1"},
			// ClassA methods reach their distinct imported callees
			{Namespace: "printers//printA1", CallerNamespace: "testTypescript.ts//ClassA//method1", Text: "printA1"},
			{Namespace: "printers//printA2", CallerNamespace: "testTypescript.ts//ClassA//method2", Text: "printA2"},
		},
	},
}

func TestCallgraphPlugin(t *testing.T) {
	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("%s__%s", testcase.FilePath, testcase.Language), func(t *testing.T) {
			t.Parallel()

			filePaths := []string{testcase.FilePath}
			treeWalker, fileSystem, err := test.SetupBasicPluginContext("fixtures", filePaths, []core.LanguageCode{testcase.Language})
			assert.NoError(t, err)

			var callgraphCallback CallgraphCallback = func(ctx context.Context, cg *CallGraph) error {
				assert.NotNil(t, cg, "Expected call graph to be non-nil")
				assert.NotNil(t, cg.assignmentGraph, "Expected assignment graph to be non-nil")

				assertAssignmentGraph(t, cg, testcase.ExpectedAssignmentGraph)

				assertCallGraph(t, cg, testcase.ExpectedCallGraph)

				assertDfs(t, cg, testcase.ExpectedDfsResults)

				return nil
			}

			pluginExecutor, err := plugin.NewTreeWalkPluginExecutor(treeWalker, []core.Plugin{
				NewCallGraphPlugin(callgraphCallback),
			})
			assert.NoError(t, err)

			report, err := pluginExecutor.Execute(context.Background(), fileSystem)
			assert.NoError(t, err)
			assert.Empty(t, report.Failed)
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
				for _, argResolution := range arg.Values {
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

func assertDfs(t *testing.T, cg *CallGraph, expectedDfsResults []dfsResultExpectation) {
	dfsResults := cg.DFS()

	actualDfsItems := make(map[dfsResultExpectation]int)
	for _, dfsResultItem := range dfsResults {
		namespace := dfsResultItem.Namespace

		callerNamespace := ""
		if dfsResultItem.Caller != nil {
			callerNamespace = dfsResultItem.Caller.Namespace
		}

		key := dfsResultExpectation{
			Namespace:       namespace,
			CallerNamespace: callerNamespace,
			Text:            dfsResultItem.Text,
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
