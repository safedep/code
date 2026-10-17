package callgraph

import (
	"slices"
	"strings"
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

// Truncation records the first place where the build of a file hit one kind
// of limit. A graph keeps only the first hit of each kind, so a later hit of
// the same kind in the file has no record. The graph is partial after it.
// Line is 1-based, and 0 when no node marks the place.
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

// maxSiteText is the maximum length in bytes of the source text of a call
// site that a graph keeps.
const maxSiteText = 120

// UnresolvedCall is a call site whose callee the graph does not know, or
// does not know in full. Text is the source of the callee, or of the call
// for a dynamic import.
type UnresolvedCall struct {
	Caller string
	Site   ast.NodePosition
	Text   string

	// Receiver is the source of the object of a member call, as client in
	// client.send(). It is empty for a bare call.
	Receiver string

	// Method is the name of the called member or function. It is empty when
	// the call computes the name, as in obj[name]() or f()().
	Method string

	Reason UnresolvedReason
}

// truncations keeps one record for each kind, the first one.
type truncations []Truncation

func (t *truncations) add(kind TruncationKind, namespace string, node *sitter.Node) {
	t.addAt(kind, namespace, ast.GetNodePosition(node))
}

func (t *truncations) addAt(kind TruncationKind, namespace string, position ast.NodePosition) {
	if t == nil || slices.ContainsFunc(*t, func(r Truncation) bool { return r.Kind == kind }) {
		return
	}
	line := int(position.StartLine)
	log.Debugf("CallGraph limit %s hit at %s, line %d", kind, namespace, line)
	*t = append(*t, Truncation{Kind: kind, Namespace: namespace, Line: line})
}

func (cg *CallGraph) truncate(kind TruncationKind, namespace string, node *sitter.Node) {
	cg.truncations.add(kind, namespace, node)
}

// addUnresolved records a call site once, although the resolvers can reach
// one site more than once. The receiver and the method come from callee,
// the function of the call.
func (cg *CallGraph) addUnresolved(caller string, site, callee *sitter.Node, treeData []byte, reason UnresolvedReason) {
	receiver, method := cg.profile.calleeParts(callee, treeData)
	record := UnresolvedCall{
		Caller:   caller,
		Site:     ast.GetNodePosition(site),
		Text:     clip(site.Content(treeData)),
		Receiver: clip(receiver),
		Method:   method,
		Reason:   reason,
	}
	if cg.unresolvedSeen[record] {
		return
	}
	if cg.unresolvedSeen == nil {
		cg.unresolvedSeen = make(map[UnresolvedCall]bool)
	}
	cg.unresolvedSeen[record] = true
	cg.unresolved = append(cg.unresolved, record)
}

// calleeParts splits the function of a call into the source of its object
// and the name of its member. A name with no member, as an identifier, has
// no object. The name is empty when the call computes it.
func (p *languageProfile) calleeParts(callee *sitter.Node, treeData []byte) (string, string) {
	if callee == nil {
		return "", ""
	}
	shape, ok := p.members[callee.Type()]
	if !ok {
		if callee.NamedChildCount() == 0 {
			return "", callee.Content(treeData)
		}
		return "", ""
	}

	var receiver, method string
	if object := shape.object.of(callee); object != nil {
		receiver = object.Content(treeData)
	}
	if name := shape.name.of(callee); name != nil {
		if shape.index {
			method, _ = p.memberName(name, treeData)
		} else {
			method = name.Content(treeData)
		}
	}
	return receiver, method
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
	if len(text) <= maxSiteText {
		return text
	}
	cut := maxSiteText
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

// memberName returns the name that a string literal index gives, as x in
// obj['x']. An index that computes the name gives none.
func (p *languageProfile) memberName(index *sitter.Node, treeData []byte) (string, bool) {
	if !p.staticString(index) {
		return "", false
	}
	var name strings.Builder
	for i := 0; i < int(index.NamedChildCount()); i++ {
		child := index.NamedChild(i)
		switch {
		case p.strings.fragments[child.Type()]:
			name.WriteString(child.Content(treeData))
		case p.strings.quotes[child.Type()]:
		default:
			return "", false
		}
	}
	if name.Len() == 0 || strings.Contains(name.String(), namespaceSeparator) {
		return "", false
	}
	return name.String(), true
}

// staticString reports that node is a string literal with no interpolation.
func (p *languageProfile) staticString(node *sitter.Node) bool {
	if node == nil || !p.strings.literals[node.Type()] {
		return false
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if p.strings.interpolations[node.NamedChild(i).Type()] {
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
