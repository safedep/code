package callgraph

import (
	"context"
	"slices"

	"github.com/safedep/code/core"
	"github.com/safedep/code/core/ast"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// syntax is the part of a tree node that a graph keeps. The graph outlives
// the tree, so it keeps no tree node.
type syntax struct {
	kind     string
	position ast.NodePosition
}

func syntaxOf(node *sitter.Node) syntax {
	if node == nil {
		return syntax{}
	}
	return syntax{kind: node.Type(), position: ast.GetNodePosition(node)}
}

func (s syntax) known() bool {
	return s.kind != ""
}

func newCallGraphNode(namespace string, s syntax) *CallGraphNode {
	return &CallGraphNode{
		Namespace: namespace,
		Kind:      s.kind,
		Position:  s.position,
		CallsTo:   []CallReference{},
	}
}

func argumentOf(nodes []*assignmentNode) CallArgument {
	values := make([]ArgumentValue, len(nodes))
	for i, node := range nodes {
		values[i] = ArgumentValue{Namespace: node.Namespace, Literal: node.IsLiteralValue()}
	}
	return CallArgument{Values: values}
}

func newCallGraph(fileName string, language core.LanguageCode) *CallGraph {
	cg := &CallGraph{
		FileName:          fileName,
		Language:          language,
		profile:           profileOf(language),
		Nodes:             make(map[string]*CallGraphNode),
		classConstructors: make(map[string]bool),
		unresolvedSeen:    make(map[UnresolvedCall]bool),
	}
	cg.assignmentGraph = *newAssignmentGraph(&cg.truncations)

	for _, namespace := range cg.profile.builtins {
		cg.assignmentGraph.addNode(namespace, nil)
	}
	return cg
}

// build adds the imports and processes the tree from root. It stops when
// ctx is done.
func (cg *CallGraph) build(ctx context.Context, root *sitter.Node, imports []*ast.ImportNode, source []byte) {
	cg.ctx = ctx
	cg.source = source
	defer func() {
		cg.ctx = nil
		cg.source = nil
	}()

	cg.addRootNode(root)
	cg.addImports(root, imports)
	processChildren(root, source, cg.FileName, cg, processorMetadata{})
	cg.resolveTargets()
}

func (cg *CallGraph) addImports(root *sitter.Node, imports []*ast.ImportNode) {
	// Required to map identifiers to imported modules as assignments
	// and register default calls for wildcard imports
	importedIdentifiers, wildcardImports := parseImports(imports, cg.profile)

	for _, wildcardImport := range wildcardImports {
		// For wildcard imports, we add a call to importeditem//*
		// assuming that anything under that namespace is posssibly used
		cg.addEdge(cg.FileName, root, wildcardImport.NamespaceTreeNode, wildcardImport.Namespace, wildcardImport.NamespaceTreeNode, []CallArgument{})
	}

	for _, importedIdentifier := range importedIdentifiers {
		cg.addNode(importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)

		if importedIdentifier.Identifier != importedIdentifier.Namespace {
			cg.assignmentGraph.addAssignment(importedIdentifier.Identifier, importedIdentifier.IdentifierTreeNode, importedIdentifier.Namespace, importedIdentifier.NamespaceTreeNode)
		}
	}
}

// resolveTargets fills the targets of each call once the assignment graph
// is complete.
func (cg *CallGraph) resolveTargets() {
	for _, node := range cg.order {
		for i := range node.CallsTo {
			ref := &node.CallsTo[i]
			ref.Targets = cg.targetsOf(ref.CalleeNamespace)
		}
	}
}

func (cg *CallGraph) targetsOf(callee string) []string {
	targets := namespacesOf(cg.assignmentGraph.resolve(callee))
	if len(targets) == 0 {
		return nil
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

// ctxCheckInterval is the number of nodes between two checks of the context.
const ctxCheckInterval = 4096

// stopped reports that the build must stop, because ctx is done. It checks
// ctx every ctxCheckInterval nodes, so the check costs little per node.
func (cg *CallGraph) stopped() bool {
	if cg.ctxErr != nil {
		return true
	}
	if cg.ctx == nil {
		return false
	}
	cg.visited++
	if cg.visited%ctxCheckInterval != 0 {
		return false
	}
	if err := cg.ctx.Err(); err != nil {
		log.Debugf("CallGraph build stopped for file %s: %v", cg.FileName, err)
		cg.ctxErr = err
		cg.truncate(TruncationDeadline, cg.FileName, nil)
		return true
	}
	return false
}

func (cg *CallGraph) addRootNode(treeNode *sitter.Node) {
	cg.RootNode = cg.addNode(cg.FileName, treeNode)
}

// addNode adds a node to the call graph and to the assignment graph. Past
// maxCallGraphNodes it adds no node to the call graph and returns nil. With
// no tree node, the call graph node takes the syntax of the assignment node.
func (cg *CallGraph) addNode(identifier string, treeNode *sitter.Node) *CallGraphNode {
	s := syntaxOf(treeNode)
	if assigned := cg.assignmentGraph.addNode(identifier, treeNode); assigned != nil && !s.known() {
		s = assigned.syntax
	}
	return cg.addCallGraphNode(identifier, s)
}

// addDefinition adds the node of a definition. A call before the
// definition, as g in g(); function g() {}, can have made the node with no
// kind, and the definition fills it. It reports that the node is new.
func (cg *CallGraph) addDefinition(namespace string, treeNode *sitter.Node) bool {
	_, exists := cg.Nodes[namespace]
	cg.addNode(namespace, treeNode)
	return !exists
}

func (cg *CallGraph) addCallGraphNode(identifier string, s syntax) *CallGraphNode {
	if node, exists := cg.Nodes[identifier]; exists {
		if node.Kind == "" {
			node.Kind, node.Position = s.kind, s.position
		}
		return node
	}

	if len(cg.order) >= maxCallGraphNodes {
		cg.truncations.addAt(TruncationNodes, identifier, s.position)
		return nil
	}

	node := newCallGraphNode(identifier, s)
	cg.Nodes[identifier] = node
	cg.order = append(cg.order, node)
	return node
}

// reference makes the reference of a call to callee from the expression at
// site.
func (cg *CallGraph) reference(callee string, site *sitter.Node, arguments []CallArgument) CallReference {
	ref := CallReference{CalleeNamespace: callee, Arguments: arguments}
	if site != nil {
		ref.Site = ast.GetNodePosition(site)
		ref.Text = clip(site.Content(cg.source))
	}
	return ref
}

// addEdge adds an edge from one function to another
func (cg *CallGraph) addEdge(caller string, callerTreeNode *sitter.Node, site *sitter.Node, callee string, calleeTreeNode *sitter.Node, arguments []CallArgument) {
	cg.addCall(caller, callerTreeNode, calleeTreeNode, cg.reference(callee, site, arguments))
}

// addCall adds the edge of ref from caller. It adds no edge when the graph
// has no room for the caller or the callee.
func (cg *CallGraph) addCall(caller string, callerTreeNode, calleeTreeNode *sitter.Node, ref CallReference) {
	callerNode := cg.addNode(caller, callerTreeNode)

	// An unbound callee stays out of the assignment graph. Otherwise the
	// scope chain finds the fallback key, and a second call to the name
	// looks bound.
	var calleeNode *CallGraphNode
	if ref.Unbound {
		calleeNode = cg.addCallGraphNode(ref.CalleeNamespace, syntaxOf(calleeTreeNode))
	} else {
		calleeNode = cg.addNode(ref.CalleeNamespace, calleeTreeNode)
	}

	if callerNode == nil || calleeNode == nil {
		return
	}
	callerNode.CallsTo = append(callerNode.CallsTo, ref)
}
