package projectgraph

import (
	"github.com/safedep/code/langcode"
)

// rule names the step of the link that made an edge. The resolver name of
// an edge joins the language of the step and the rule, as python.import.
type rule string

const (
	ruleExternal         rule = "external"
	ruleWildcardExternal rule = "wildcard-external"
	ruleImport           rule = "import"
	ruleLocal            rule = "local"
	rulePackage          rule = "package"
	ruleNested           rule = "nested"
	ruleNamespace        rule = "namespace"
	ruleWildcard         rule = "wildcard"
	ruleWildcardImport   rule = "wildcard-import"
	ruleDotImport        rule = "dot-import"
	ruleSuper            rule = "super"
	ruleModuleGetattr    rule = "module-getattr"
	ruleMember           rule = "member"
	ruleAssignment       rule = "assignment"
	ruleDeclaredType     rule = "declared-type"
	ruleConstructor      rule = "constructor"
	ruleJavaLang         rule = "java-lang"
	ruleCHA              rule = "cha"
	ruleReference        rule = "reference"
	ruleReexport         rule = "reexport"
	ruleReexportStar     rule = "reexport-star"
	ruleOverride         rule = "override"
	ruleArgumentFunction rule = "argument-function"
	ruleDecorator        rule = "decorator"
	ruleDecoratorClosure rule = "decorator-closure"
	ruleInit             rule = "init"
	ruleClassInit        rule = "class-init"
	ruleFileInit         rule = "file-init"
	ruleInstance         rule = "instance"
	ruleImplicit         rule = "implicit"
	ruleExtension        rule = "extension"
)

// via returns the rule with the way that a resolver found the target, as
// import.relative.
func (r rule) via(v string) rule {
	return rule(string(r) + "." + v)
}

func resolverName(lang langcode.Code, r rule) string {
	return string(lang) + "." + string(r)
}

// link adds an edge of the kind that the rule made in a file of the
// language.
func (b *builder) link(lang langcode.Code, from, to string, site Site, kind EdgeKind, r rule) {
	b.addEdge(newEdge(from, to, site, kind, resolverName(lang, r)))
}

// siteOf returns the site of a node of the graph: its file and its line.
func (b *builder) siteOf(key string) Site {
	n := b.g.Nodes[key]
	return Site{Path: n.Path, Line: n.Line}
}

// externalResolution resolves a target to its external node.
func (b *builder) externalResolution(target string) resolution {
	return resolution{key: b.external(target), kind: EdgeDirect, rule: ruleExternal}
}

// wildcardExternal resolves a name to the external node that a wildcard
// import of a dependency can give it.
func (b *builder) wildcardExternal(key string) resolution {
	return resolution{key: b.external(key), kind: EdgeWildcard, rule: ruleWildcardExternal}
}
