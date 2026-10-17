package callgraph

import (
	"slices"
	"unicode/utf8"

	"github.com/safedep/code/core/ast"
	"github.com/safedep/dry/log"
	sitter "github.com/smacker/go-tree-sitter"
)

// TruncationKind names a limit of the build of a call graph.
type TruncationKind string

const (
	TruncationNodes           TruncationKind = "nodes"
	TruncationAssignmentNodes TruncationKind = "assignment-nodes"
	TruncationDepth           TruncationKind = "depth"
	TruncationMemberDepth     TruncationKind = "member-depth"
	TruncationAttributeDepth  TruncationKind = "attribute-depth"
	TruncationPolymorphic     TruncationKind = "polymorphic"
	TruncationDFSResults      TruncationKind = "dfs-results"
	TruncationDeadline        TruncationKind = "deadline"
)

// Truncation records the first place where the build hit a limit. The graph
// is partial after it. Line is 1-based, and 0 when no node marks the place.
type Truncation struct {
	Kind      TruncationKind
	Namespace string
	Line      int
}

// UnresolvedReason tells why the graph has no callee for a call site.
type UnresolvedReason string

const (
	UnresolvedComputedName    UnresolvedReason = "computed-name"
	UnresolvedDynamicImport   UnresolvedReason = "dynamic-import"
	UnresolvedUnknownReceiver UnresolvedReason = "unknown-receiver"
	UnresolvedAmbiguous       UnresolvedReason = "ambiguous"
	UnresolvedLimit           UnresolvedReason = "limit"
)

// maxUnresolvedText is the maximum length in bytes of UnresolvedCall.Text.
const maxUnresolvedText = 120

// UnresolvedCall is a call site whose callee the graph does not know, or
// does not know in full. Text is the source of the callee, or of the call
// for a dynamic import.
type UnresolvedCall struct {
	Caller string
	Site   ast.NodePosition
	Text   string
	Reason UnresolvedReason
}

// truncations keeps one record for each kind, the first one.
type truncations []Truncation

func (t *truncations) add(kind TruncationKind, namespace string, node *sitter.Node) {
	if t == nil || slices.ContainsFunc(*t, func(r Truncation) bool { return r.Kind == kind }) {
		return
	}
	line := int(ast.GetNodePosition(node).StartLine)
	log.Debugf("CallGraph limit %s hit at %s, line %d", kind, namespace, line)
	*t = append(*t, Truncation{Kind: kind, Namespace: namespace, Line: line})
}

func (cg *CallGraph) truncate(kind TruncationKind, namespace string, node *sitter.Node) {
	cg.truncations.add(kind, namespace, node)
}

// addUnresolved records a call site once, although the resolvers can reach
// one site more than once.
func (cg *CallGraph) addUnresolved(caller string, site *sitter.Node, treeData []byte, reason UnresolvedReason) {
	record := UnresolvedCall{
		Caller: caller,
		Site:   ast.GetNodePosition(site),
		Text:   clip(site.Content(treeData)),
		Reason: reason,
	}
	if cg.unresolvedSeen[record] {
		return
	}
	cg.unresolvedSeen[record] = true
	cg.unresolved = append(cg.unresolved, record)
}

// depthLimit records a resolver that stops at a depth limit and returns no
// namespace for the site.
func (cg *CallGraph) depthLimit(kind TruncationKind, caller string, site *sitter.Node, treeData []byte) {
	cg.truncate(kind, caller, site)
	cg.addUnresolved(caller, site, treeData, UnresolvedLimit)
}

// receiverTargets resolves an object to its values, at most
// maxPolymorphicResolutions of them.
func (cg *CallGraph) receiverTargets(namespace, caller string, site *sitter.Node) []*assignmentNode {
	targets := cg.assignmentGraph.resolve(namespace)
	if len(targets) > maxPolymorphicResolutions {
		cg.truncate(TruncationPolymorphic, caller, site)
		targets = targets[:maxPolymorphicResolutions]
	}
	return targets
}

// Truncations returns the limits that the build hit, in the order it hit
// them.
func (cg *CallGraph) Truncations() []Truncation {
	return slices.Clone(cg.truncations)
}

// Unresolved returns the call sites with no callee, or with a partial one,
// in the order of the build.
func (cg *CallGraph) Unresolved() []UnresolvedCall {
	return slices.Clone(cg.unresolved)
}

// LimitExceeded reports that the build hit a limit, so the graph is partial.
func (cg *CallGraph) LimitExceeded() bool {
	return len(cg.truncations) > 0
}

func clip(text string) string {
	if len(text) <= maxUnresolvedText {
		return text
	}
	cut := maxUnresolvedText
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// qualify appends name to each base namespace.
func qualify(bases []string, name string) []string {
	if len(bases) == 0 {
		return nil
	}
	names := make([]string, len(bases))
	for i, base := range bases {
		names[i] = base + namespaceSeparator + name
	}
	return names
}

func namespacesOf(nodes []*assignmentNode) []string {
	names := make([]string, len(nodes))
	for i, node := range nodes {
		names[i] = node.Namespace
	}
	return names
}

// staticString reports that node is a string literal with no interpolation,
// in Python or JavaScript.
func staticString(node *sitter.Node) bool {
	if node == nil || (node.Type() != "string" && node.Type() != "template_string") {
		return false
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		switch node.NamedChild(i).Type() {
		case "interpolation", "template_substitution":
			return false
		}
	}
	return true
}

func firstArgument(argumentsNode *sitter.Node) *sitter.Node {
	if argumentsNode == nil || argumentsNode.NamedChildCount() == 0 {
		return nil
	}
	return argumentsNode.NamedChild(0)
}
