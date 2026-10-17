//go:build projectgraph

package projectgraph

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/safedep/code/core"
)

// synthSeed fixes the generated projects, so two runs measure the same code.
const synthSeed = 20261017

// modulesPerPackage is the size of one package. Each package also has a
// barrel file and a test file.
const modulesPerPackage = 25

var (
	synthVerbs = []string{"load", "parse", "build", "render", "fetch", "store", "validate", "merge", "encode", "decode", "resolve", "apply", "collect", "format", "sync", "check"}
	synthNouns = []string{"order", "user", "invoice", "report", "cache", "session", "token", "record", "config", "account", "payment", "event", "profile", "message", "upload", "schedule", "policy", "metric", "address", "ticket", "batch", "query", "template", "document"}
	synthRoles = []string{"Service", "Store", "Handler", "Client", "Builder", "Manager", "Repository", "Worker"}
)

// externalPackage is a dependency that the generated code calls. The 200
// functions of the 20 packages of a language are the reach targets.
type externalPackage struct {
	name  string
	funcs []string
}

var synthExternals = map[core.LanguageCode][]externalPackage{
	core.LanguageCodePython: {
		{"requests", []string{"get", "post", "put", "delete", "head", "patch", "request", "options", "session", "Session"}},
		{"yaml", []string{"load", "safe_load", "dump", "safe_dump", "load_all", "full_load", "add_constructor", "add_representer", "compose", "emit"}},
		{"jinja2", []string{"Template", "Environment", "FileSystemLoader", "select_autoescape", "make_logging_undefined", "pass_context", "pass_environment", "contextfilter", "escape", "Markup"}},
		{"sqlalchemy", []string{"create_engine", "select", "insert", "update", "delete", "text", "and_", "or_", "func", "inspect"}},
		{"boto3", []string{"client", "resource", "session", "setup_default_session", "set_stream_logger", "Session", "get_available_services", "get_partition", "get_regions", "credentials"}},
		{"flask", []string{"Flask", "jsonify", "request", "render_template", "redirect", "url_for", "abort", "make_response", "send_file", "Blueprint"}},
		{"numpy", []string{"array", "zeros", "ones", "arange", "linspace", "mean", "sum", "dot", "reshape", "concatenate"}},
		{"pandas", []string{"DataFrame", "Series", "read_csv", "read_json", "concat", "merge", "to_datetime", "pivot_table", "read_sql", "isna"}},
		{"click", []string{"command", "option", "argument", "group", "echo", "secho", "prompt", "confirm", "pass_context", "Path"}},
		{"pydantic", []string{"BaseModel", "Field", "validator", "root_validator", "parse_obj_as", "create_model", "constr", "conint", "ValidationError", "field_validator"}},
		{"redis", []string{"Redis", "StrictRedis", "from_url", "ConnectionPool", "BlockingConnectionPool", "Sentinel", "lock", "pipeline", "pubsub", "client_list"}},
		{"celery", []string{"Celery", "shared_task", "group", "chain", "chord", "signature", "current_app", "states", "uuid", "result"}},
		{"httpx", []string{"get", "post", "stream", "Client", "AsyncClient", "request", "put", "delete", "Timeout", "Limits"}},
		{"lxml", []string{"etree", "html", "objectify", "builder", "fromstring", "tostring", "parse", "XPath", "XMLParser", "HTMLParser"}},
		{"cryptography", []string{"fernet", "x509", "hazmat", "encrypt", "decrypt", "generate_key", "load_pem", "sign", "verify", "hash"}},
		{"jwt", []string{"encode", "decode", "get_unverified_header", "PyJWKClient", "PyJWT", "register_algorithm", "unregister_algorithm", "get_algorithm_by_name", "ExpiredSignatureError", "InvalidTokenError"}},
		{"paramiko", []string{"SSHClient", "AutoAddPolicy", "RSAKey", "Transport", "SFTPClient", "ProxyCommand", "Agent", "SSHConfig", "HostKeys", "Channel"}},
		{"pymongo", []string{"MongoClient", "ASCENDING", "DESCENDING", "InsertOne", "UpdateOne", "DeleteOne", "ReplaceOne", "IndexModel", "ReturnDocument", "WriteConcern"}},
		{"dateutil", []string{"parser", "relativedelta", "tz", "rrule", "easter", "parse", "isoparse", "gettz", "tzutc", "tzlocal"}},
		{"toml", []string{"load", "loads", "dump", "dumps", "TomlDecoder", "TomlEncoder", "TomlDecodeError", "TomlPreserveInlineDictEncoder", "TomlArraySeparatorEncoder", "TomlNumpyEncoder"}},
	},
	core.LanguageCodeJavascript: {
		{"lodash", []string{"template", "merge", "cloneDeep", "get", "set", "debounce", "throttle", "pick", "omit", "uniq"}},
		{"axios", []string{"get", "post", "put", "delete", "request", "create", "head", "patch", "all", "isCancel"}},
		{"express", []string{"Router", "json", "urlencoded", "static", "raw", "text", "query", "application", "request", "response"}},
		{"moment", []string{"utc", "unix", "duration", "isMoment", "parseZone", "locale", "max", "min", "isDate", "invalid"}},
		{"chalk", []string{"red", "green", "blue", "yellow", "bold", "dim", "cyan", "magenta", "gray", "underline"}},
		{"commander", []string{"program", "Command", "Option", "Argument", "createCommand", "createOption", "createArgument", "Help", "InvalidArgumentError", "CommanderError"}},
		{"uuid", []string{"v1", "v3", "v4", "v5", "validate", "parse", "stringify", "version", "NIL", "v7"}},
		{"yaml", []string{"parse", "stringify", "parseDocument", "parseAllDocuments", "Document", "isMap", "isSeq", "visit", "Scalar", "YAMLMap"}},
		{"ejs", []string{"render", "renderFile", "compile", "clearCache", "escapeXML", "resolveInclude", "fileLoader", "localsName", "promiseImpl", "cache"}},
		{"handlebars", []string{"compile", "registerHelper", "registerPartial", "precompile", "template", "create", "SafeString", "escapeExpression", "unregisterHelper", "noConflict"}},
		{"minimist", []string{"parse", "argv", "boolean", "string", "alias", "default", "stopEarly", "unknown", "dashdash", "opts"}},
		{"debug", []string{"enable", "disable", "enabled", "log", "formatArgs", "save", "load", "useColors", "coerce", "humanize"}},
		{"semver", []string{"valid", "clean", "satisfies", "gt", "lt", "coerce", "inc", "diff", "major", "compare"}},
		{"glob", []string{"sync", "glob", "hasMagic", "escape", "unescape", "Glob", "globSync", "globStream", "iterate", "globIterate"}},
		{"rimraf", []string{"sync", "rimraf", "native", "manual", "windows", "posix", "moveRemove", "nativeSync", "manualSync", "windowsSync"}},
		{"mkdirp", []string{"sync", "mkdirp", "native", "manual", "nativeSync", "manualSync", "useNative", "useNativeSync", "mkdirpNative", "mkdirpManual"}},
		{"ws", []string{"WebSocket", "Server", "createWebSocketStream", "WebSocketServer", "Receiver", "Sender", "send", "close", "ping", "terminate"}},
		{"jsonwebtoken", []string{"sign", "verify", "decode", "JsonWebTokenError", "TokenExpiredError", "NotBeforeError", "signAsync", "verifyAsync", "complete", "algorithms"}},
		{"bcrypt", []string{"hash", "compare", "genSalt", "hashSync", "compareSync", "genSaltSync", "getRounds", "truncates", "encodeBase64", "decodeBase64"}},
		{"dotenv", []string{"config", "parse", "populate", "decrypt", "configDotenv", "load", "expand", "options", "logger", "debug"}},
	},
}

func init() {
	synthExternals[core.LanguageCodeTypescript] = synthExternals[core.LanguageCodeJavascript]
}

// reachTargets are the keys of every external function of a language.
func reachTargets(language core.LanguageCode) []string {
	var targets []string
	for _, p := range synthExternals[language] {
		for _, f := range p.funcs {
			targets = append(targets, p.name+"//"+f)
		}
	}
	return targets
}

type dynamicKind int

const (
	dynamicNone dynamicKind = iota
	dynamicAttribute
	dynamicImport
	dynamicEval
)

type externalCall struct{ pkg, fn int }

// synthModule is the shape of one generated module. The renderers of each
// language write it as source.
type synthModule struct {
	id, pkg, idx int
	class        string
	role         int
	methods      [][2]int
	helpers      [][2]int
	deps         []int
	sibling      int
	externals    []externalCall
	dynamic      dynamicKind
	decorated    bool
}

type synthProject struct {
	language core.LanguageCode
	packages [][]*synthModule
	modules  []*synthModule
}

// entryFiles counts the source files of a project outside its packages.
var entryFiles = map[core.LanguageCode]int{
	core.LanguageCodePython:     1,
	core.LanguageCodeJavascript: 1,
	core.LanguageCodeTypescript: 2,
}

// generate makes the shape of a project with files source files: the
// modules, a barrel file and a test file per package, and the entry files.
func generate(language core.LanguageCode, files int) *synthProject {
	rng := rand.New(rand.NewPCG(synthSeed, uint64(len(language))))
	p := &synthProject{language: language}
	left := files - entryFiles[language]
	for left > 2 {
		n := min(modulesPerPackage, left-2)
		var pkg []*synthModule
		for i := range n {
			m := newModule(rng, len(p.modules), len(p.packages), i, len(synthExternals[language]))
			pkg = append(pkg, m)
			p.modules = append(p.modules, m)
		}
		p.packages = append(p.packages, pkg)
		left -= n + 2
	}
	for _, m := range p.modules {
		m.link(rng, p)
	}
	return p
}

func newModule(rng *rand.Rand, id, pkg, idx, externals int) *synthModule {
	m := &synthModule{id: id, pkg: pkg, idx: idx, sibling: -1, role: rng.IntN(len(synthRoles))}
	names := rng.Perm(len(synthVerbs) * len(synthNouns))
	pair := func(i int) [2]int { return [2]int{names[i] % len(synthVerbs), names[i] / len(synthVerbs)} }
	for i := range 4 {
		m.methods = append(m.methods, pair(i))
		m.helpers = append(m.helpers, pair(4+i))
	}
	m.class = title(synthNouns[pair(8)[1]]) + synthRoles[m.role]
	for range 2 {
		m.externals = append(m.externals, externalCall{pkg: rng.IntN(externals), fn: rng.IntN(10)})
	}
	if rng.IntN(2) == 0 {
		m.externals = append(m.externals, externalCall{pkg: rng.IntN(externals), fn: rng.IntN(10)})
	}
	switch r := rng.IntN(100); {
	case r < 3:
		m.dynamic = dynamicAttribute
	case r < 5:
		m.dynamic = dynamicImport
	case r < 6:
		m.dynamic = dynamicEval
	}
	m.decorated = rng.IntN(4) == 0
	return m
}

// link picks the imports of a module. Most go to an earlier module, so the
// run functions form long chains. One in ten goes to a later module and
// makes a cycle, as real projects have.
func (m *synthModule) link(rng *rand.Rand, p *synthProject) {
	if m.idx > 0 {
		m.sibling = p.packages[m.pkg][rng.IntN(m.idx)].id
	}
	for range 1 + rng.IntN(3) {
		var dep int
		switch {
		case m.id > 0 && rng.IntN(10) != 0:
			dep = rng.IntN(m.id)
		default:
			dep = rng.IntN(len(p.modules))
		}
		if dep != m.id && dep != m.sibling && !slices.Contains(m.deps, dep) {
			m.deps = append(m.deps, dep)
		}
	}
}

func title(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func snake(w [2]int) string { return synthVerbs[w[0]] + "_" + synthNouns[w[1]] }
func camel(w [2]int) string { return synthVerbs[w[0]] + title(synthNouns[w[1]]) }

// write renders the project into dir and returns the number of source
// files.
func (p *synthProject) write(dir string) (int, error) {
	files := map[string]string{}
	switch p.language {
	case core.LanguageCodePython:
		p.renderPython(files)
	case core.LanguageCodeJavascript:
		p.renderJavaScript(files)
	case core.LanguageCodeTypescript:
		p.renderTypeScript(files)
	default:
		return 0, fmt.Errorf("no template for %s", p.language)
	}
	sources := 0
	for name, text := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return 0, err
		}
		if ext := filepath.Ext(name); ext == ".py" || ext == ".js" || ext == ".ts" {
			sources++
		}
	}
	return sources, nil
}

func (p *synthProject) pkgName(pkg int) string { return fmt.Sprintf("pkg%03d", pkg) }

func (p *synthProject) modName(m *synthModule) string { return fmt.Sprintf("mod%02d", m.idx) }

// lines collects source lines with an indent.
type lines struct{ b strings.Builder }

func (l *lines) add(indent int, format string, args ...any) {
	l.b.WriteString(strings.Repeat("    ", indent))
	fmt.Fprintf(&l.b, format, args...)
	l.b.WriteByte('\n')
}

func (l *lines) String() string { return l.b.String() }

func (p *synthProject) renderPython(files map[string]string) {
	ext := synthExternals[core.LanguageCodePython]
	for _, pkg := range p.packages {
		for _, m := range pkg {
			files[fmt.Sprintf("%s/%s.py", p.pkgName(m.pkg), p.modName(m))] = p.pythonModule(m, ext)
		}
		var init lines
		first := pkg[0]
		init.add(0, "from .%s import %s, run", p.modName(first), first.class)
		init.add(0, "")
		init.add(0, "__all__ = [%q, \"run\"]", first.class)
		files[p.pkgName(first.pkg)+"/__init__.py"] = init.String()

		var test lines
		last := pkg[len(pkg)-1]
		test.add(0, "import pytest")
		test.add(0, "")
		test.add(0, "from %s import %s", p.pkgName(last.pkg), p.modName(last))
		test.add(0, "")
		test.add(0, "")
		test.add(0, "def test_run():")
		test.add(1, "assert %s.run([\"input.yaml\"]) is not None", p.modName(last))
		test.add(0, "")
		test.add(0, "")
		test.add(0, "@pytest.mark.parametrize(\"value\", [1, 2, 3])")
		test.add(0, "def test_%s(value):", snake(last.helpers[0]))
		test.add(1, "assert %s.%s(value)", p.modName(last), snake(last.helpers[0]))
		files[fmt.Sprintf("tests/test_%s.py", p.pkgName(last.pkg))] = test.String()
	}

	var main lines
	main.add(0, "import sys")
	main.add(0, "")
	for _, pkg := range p.packages {
		main.add(0, "import %s", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	main.add(0, "")
	main.add(0, "def main():")
	for _, pkg := range p.packages {
		main.add(1, "%s.run(sys.argv[1:])", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	main.add(0, "")
	main.add(0, "if __name__ == \"__main__\":")
	main.add(1, "main()")
	files["main.py"] = main.String()

	var deps []string
	for _, e := range ext {
		deps = append(deps, fmt.Sprintf("    %q,", e.name))
	}
	files["pyproject.toml"] = "[project]\nname = \"bench-app\"\nversion = \"1.0.0\"\ndependencies = [\n" + strings.Join(deps, "\n") +
		"\n]\n\n[project.scripts]\nbench-app = \"main:main\"\n"
}

func (p *synthProject) pythonModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "\"\"\"%s handles the %s of %s.\"\"\"", p.modName(m), synthNouns[m.methods[0][1]], p.pkgName(m.pkg))
	l.add(0, "import functools")
	l.add(0, "import logging")
	if m.dynamic == dynamicImport {
		l.add(0, "import importlib")
	}
	first := ext[m.externals[0].pkg]
	l.add(0, "import %s", first.name)
	from := map[string]bool{}
	for _, e := range m.externals[1:] {
		pkg := ext[e.pkg]
		if pkg.name == first.name || from[pkg.name+pkg.funcs[e.fn]] {
			continue
		}
		from[pkg.name+pkg.funcs[e.fn]] = true
		l.add(0, "from %s import %s as %s_%s", pkg.name, pkg.funcs[e.fn], pkg.name, pkg.funcs[e.fn])
	}
	l.add(0, "")
	for i, d := range m.deps {
		dep := p.modules[d]
		l.add(0, "from %s import %s as dep%d", p.pkgName(dep.pkg), p.modName(dep), i)
	}
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(0, "from .%s import %s, %s", p.modName(sib), sib.class, snake(sib.helpers[0]))
	}
	l.add(0, "")
	l.add(0, "log = logging.getLogger(__name__)")
	l.add(0, "")
	l.add(0, "")

	call := func(e externalCall) string {
		pkg := ext[e.pkg]
		if pkg.name == first.name {
			return fmt.Sprintf("%s.%s", first.name, pkg.funcs[e.fn])
		}
		return fmt.Sprintf("%s_%s", pkg.name, pkg.funcs[e.fn])
	}

	l.add(0, "class %s:", m.class)
	l.add(1, "\"\"\"%s keeps the state of one %s.\"\"\"", m.class, synthNouns[m.methods[0][1]])
	l.add(0, "")
	l.add(1, "def __init__(self, config):")
	l.add(2, "self.config = config")
	if len(m.deps) > 0 {
		dep := p.modules[m.deps[0]]
		l.add(2, "self.store = dep0.%s(config)", dep.class)
	} else {
		l.add(2, "self.store = None")
	}
	l.add(2, "self.items = []")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "@property")
			l.add(1, "def %s(self):", snake(w))
			l.add(2, "return len(self.items)")
			continue
		}
		l.add(1, "def %s(self, value):", snake(w))
		l.add(2, "log.debug(\"%s %%s\", value)", snake(w))
		l.add(2, "result = %s(value)", snake(m.helpers[i%len(m.helpers)]))
		if i == 0 {
			l.add(2, "data = %s(result)", call(m.externals[0]))
		} else {
			l.add(2, "data = result")
		}
		if i == 1 && len(m.deps) > 0 {
			dep := p.modules[m.deps[0]]
			l.add(2, "data = self.store.%s(data)", snake(dep.methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "handler = getattr(self, self.config.get(\"handler\", \"%s\"))", snake(m.methods[0]))
			l.add(2, "data = handler(data)")
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = self.%s(data)", snake(m.methods[i+1]))
		}
		l.add(2, "for item in self.items:")
		l.add(3, "item.notify(data)")
		l.add(2, "self.items.append(data)")
		l.add(2, "return data")
	}
	l.add(0, "")
	l.add(0, "")

	for i, w := range m.helpers {
		if i == 0 && m.decorated {
			l.add(0, "@functools.lru_cache(maxsize=128)")
		}
		l.add(0, "def %s(value):", snake(w))
		l.add(1, "if value is None:")
		l.add(2, "raise ValueError(\"%s needs a value\")", snake(w))
		switch {
		case i+1 < len(m.helpers):
			l.add(1, "value = %s(value)", snake(m.helpers[i+1]))
		case m.sibling >= 0:
			l.add(1, "value = %s(value)", snake(p.modules[m.sibling].helpers[0]))
		}
		if i < len(m.externals) && i > 0 {
			l.add(1, "value = %s(value)", call(m.externals[i]))
		}
		if i < len(m.deps) {
			l.add(1, "value = dep%d.%s(value)", i, snake(p.modules[m.deps[i]].helpers[0]))
		}
		if i == 2 && m.dynamic == dynamicEval {
			l.add(1, "value = eval(str(value))")
		}
		l.add(1, "return {\"%s\": value, \"size\": len(str(value))}", synthNouns[w[1]])
		l.add(0, "")
		l.add(0, "")
	}

	l.add(0, "def run(argv):")
	l.add(1, "service = %s({\"name\": %q})", m.class, p.modName(m))
	if m.sibling >= 0 {
		l.add(1, "peer = %s({})", p.modules[m.sibling].class)
		l.add(1, "peer.%s(argv)", snake(p.modules[m.sibling].methods[0]))
	}
	for i := range m.deps {
		l.add(1, "dep%d.run(argv)", i)
	}
	if m.dynamic == dynamicImport {
		l.add(1, "plugin = importlib.import_module(\"%s.\" + argv[0])", p.pkgName(m.pkg))
		l.add(1, "plugin.run(argv)")
	}
	l.add(1, "return service.%s(argv)", snake(m.methods[0]))
	return l.String()
}

func (p *synthProject) renderJavaScript(files map[string]string) {
	ext := synthExternals[core.LanguageCodeJavascript]
	for _, pkg := range p.packages {
		for _, m := range pkg {
			files[fmt.Sprintf("src/%s/%s.js", p.pkgName(m.pkg), p.modName(m))] = p.javaScriptModule(m, ext)
		}
		first, last := pkg[0], pkg[len(pkg)-1]
		var index lines
		index.add(0, "'use strict';")
		index.add(0, "")
		index.add(0, "const { %s, run } = require('./%s');", first.class, p.modName(first))
		index.add(0, "")
		index.add(0, "module.exports = { %s, run };", first.class)
		files[fmt.Sprintf("src/%s/index.js", p.pkgName(first.pkg))] = index.String()

		var test lines
		test.add(0, "const assert = require('assert');")
		test.add(0, "const { run, %s } = require('./%s');", camel(last.helpers[0]), p.modName(last))
		test.add(0, "")
		test.add(0, "describe('%s', () => {", p.modName(last))
		test.add(1, "it('runs', () => {")
		test.add(2, "assert.ok(run(['input.yaml']));")
		test.add(1, "});")
		test.add(1, "it('%s', () => {", camel(last.helpers[0]))
		test.add(2, "assert.ok(%s(1));", camel(last.helpers[0]))
		test.add(1, "});")
		test.add(0, "});")
		files[fmt.Sprintf("src/%s/%s.test.js", p.pkgName(last.pkg), p.modName(last))] = test.String()
	}

	var main lines
	main.add(0, "#!/usr/bin/env node")
	main.add(0, "'use strict';")
	main.add(0, "")
	for _, pkg := range p.packages {
		main.add(0, "const %s = require('./%s');", p.pkgName(pkg[0].pkg), p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	main.add(0, "function main(argv) {")
	for _, pkg := range p.packages {
		main.add(1, "%s.run(argv);", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "}")
	main.add(0, "")
	main.add(0, "if (require.main === module) {")
	main.add(1, "main(process.argv.slice(2));")
	main.add(0, "}")
	main.add(0, "")
	main.add(0, "module.exports = { main };")
	files["src/index.js"] = main.String()
	files["package.json"] = packageJSON(ext, "src/index.js")
}

func (p *synthProject) javaScriptModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "'use strict';")
	l.add(0, "")
	required := map[string]bool{}
	for i, e := range m.externals {
		pkg := ext[e.pkg]
		if required[pkg.name] {
			continue
		}
		required[pkg.name] = true
		if i == 0 {
			l.add(0, "const %s = require('%s');", pkg.name, pkg.name)
		} else {
			l.add(0, "const { %s: %s%s } = require('%s');", pkg.funcs[e.fn], pkg.name, title(pkg.funcs[e.fn]), pkg.name)
		}
	}
	for i, d := range m.deps {
		dep := p.modules[d]
		l.add(0, "const dep%d = require('../%s/%s');", i, p.pkgName(dep.pkg), p.modName(dep))
	}
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(0, "const { %s, %s } = require('./%s');", sib.class, camel(sib.helpers[0]), p.modName(sib))
	}
	l.add(0, "")
	call := func(e externalCall) string {
		pkg := ext[e.pkg]
		if pkg.name == ext[m.externals[0].pkg].name {
			return fmt.Sprintf("%s.%s", pkg.name, pkg.funcs[e.fn])
		}
		for _, prev := range m.externals {
			if ext[prev.pkg].name == pkg.name {
				return fmt.Sprintf("%s%s", pkg.name, title(pkg.funcs[prev.fn]))
			}
		}
		return ""
	}

	l.add(0, "class %s {", m.class)
	l.add(1, "constructor(config) {")
	l.add(2, "this.config = config;")
	if len(m.deps) > 0 {
		l.add(2, "this.store = new dep0.%s(config);", p.modules[m.deps[0]].class)
	} else {
		l.add(2, "this.store = null;")
	}
	l.add(2, "this.items = [];")
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "get %s() {", camel(w))
			l.add(2, "return this.items.length;")
			l.add(1, "}")
			continue
		}
		l.add(1, "%s(value) {", camel(w))
		l.add(2, "let data = %s(value);", camel(m.helpers[i%len(m.helpers)]))
		if i == 0 {
			l.add(2, "data = %s(data);", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "data = this.store.%s(data);", camel(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "data = this[this.config.handler || '%s'](data);", camel(m.methods[0]))
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = this.%s(data);", camel(m.methods[i+1]))
		}
		l.add(2, "this.items.forEach((item) => item.notify(data));")
		l.add(2, "this.items.push(data);")
		l.add(2, "return data;")
		l.add(1, "}")
	}
	l.add(0, "}")
	l.add(0, "")

	for i, w := range m.helpers {
		l.add(0, "function %s(value) {", camel(w))
		l.add(1, "if (value === undefined) {")
		l.add(2, "throw new Error('%s needs a value');", camel(w))
		l.add(1, "}")
		switch {
		case i+1 < len(m.helpers):
			l.add(1, "value = %s(value);", camel(m.helpers[i+1]))
		case m.sibling >= 0:
			l.add(1, "value = %s(value);", camel(p.modules[m.sibling].helpers[0]))
		}
		if i < len(m.externals) && i > 0 {
			l.add(1, "value = %s(value);", call(m.externals[i]))
		}
		if i < len(m.deps) {
			l.add(1, "value = dep%d.%s(value);", i, camel(p.modules[m.deps[i]].helpers[0]))
		}
		if i == 2 && m.dynamic == dynamicEval {
			l.add(1, "value = eval(String(value));")
		}
		l.add(1, "return { %s: value, size: String(value).length };", synthNouns[w[1]])
		l.add(0, "}")
		l.add(0, "")
	}

	l.add(0, "function run(argv) {")
	l.add(1, "const service = new %s({ name: '%s' });", m.class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(1, "new %s({}).%s(argv);", sib.class, camel(sib.methods[0]))
	}
	for i := range m.deps {
		l.add(1, "dep%d.run(argv);", i)
	}
	if m.dynamic == dynamicImport {
		l.add(1, "const plugin = require(`./plugins/${argv[0]}`);")
		l.add(1, "plugin.run(argv);")
	}
	l.add(1, "return service.%s(argv);", camel(m.methods[0]))
	l.add(0, "}")
	l.add(0, "")
	var exports []string
	exports = append(exports, m.class)
	for _, w := range m.helpers {
		exports = append(exports, camel(w))
	}
	l.add(0, "module.exports = { %s, run };", strings.Join(exports, ", "))
	return l.String()
}

func (p *synthProject) renderTypeScript(files map[string]string) {
	ext := synthExternals[core.LanguageCodeTypescript]
	for _, pkg := range p.packages {
		for _, m := range pkg {
			files[fmt.Sprintf("src/%s/%s.ts", p.pkgName(m.pkg), p.modName(m))] = p.typeScriptModule(m, ext)
		}
		first, last := pkg[0], pkg[len(pkg)-1]
		files[fmt.Sprintf("src/%s/index.ts", p.pkgName(first.pkg))] = fmt.Sprintf("export { %s, run } from './%s';\nexport * as %s from './%s';\n",
			first.class, p.modName(first), p.modName(last), p.modName(last))

		var test lines
		test.add(0, "import { describe, expect, it } from 'vitest';")
		test.add(0, "import { run, %s } from './%s';", camel(last.helpers[0]), p.modName(last))
		test.add(0, "")
		test.add(0, "describe('%s', () => {", p.modName(last))
		test.add(1, "it('runs', async () => {")
		test.add(2, "expect(await run(['input.yaml'])).toBeDefined();")
		test.add(1, "});")
		test.add(1, "it('%s', () => {", camel(last.helpers[0]))
		test.add(2, "expect(%s('1')).toBeTruthy();", camel(last.helpers[0]))
		test.add(1, "});")
		test.add(0, "});")
		files[fmt.Sprintf("src/%s/%s.test.ts", p.pkgName(last.pkg), p.modName(last))] = test.String()
	}

	var main lines
	for _, pkg := range p.packages {
		main.add(0, "import * as %s from '@app/%s';", p.pkgName(pkg[0].pkg), p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	main.add(0, "export async function main(argv: string[]): Promise<void> {")
	for _, pkg := range p.packages {
		main.add(1, "await %s.run(argv);", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "}")
	main.add(0, "")
	main.add(0, "main(process.argv.slice(2)).catch((err: Error) => {")
	main.add(1, "console.error(err);")
	main.add(1, "process.exit(1);")
	main.add(0, "});")
	files["src/index.ts"] = main.String()
	files["src/types.ts"] = "export interface Config {\n    name: string;\n    handler?: string;\n}\n\nexport interface Result {\n    [key: string]: unknown;\n    size: number;\n}\n"
	files["package.json"] = packageJSON(ext, "src/index.ts")
	files["tsconfig.json"] = "{\n  \"compilerOptions\": {\n    \"target\": \"es2022\",\n    \"module\": \"commonjs\",\n    \"strict\": true,\n    \"outDir\": \"dist\",\n    \"baseUrl\": \".\",\n    \"paths\": { \"@app/*\": [\"src/*\"] }\n  },\n  \"include\": [\"src\"]\n}\n"
}

func (p *synthProject) typeScriptModule(m *synthModule, ext []externalPackage) string {
	var l lines
	imported := map[string]bool{}
	for i, e := range m.externals {
		pkg := ext[e.pkg]
		if imported[pkg.name] {
			continue
		}
		imported[pkg.name] = true
		if i == 0 {
			l.add(0, "import * as %s from '%s';", pkg.name, pkg.name)
		} else {
			l.add(0, "import { %s as %s%s } from '%s';", pkg.funcs[e.fn], pkg.name, title(pkg.funcs[e.fn]), pkg.name)
		}
	}
	l.add(0, "import type { Config, Result } from '../types';")
	for i, d := range m.deps {
		dep := p.modules[d]
		if i%2 == 0 {
			l.add(0, "import * as dep%d from '@app/%s/%s';", i, p.pkgName(dep.pkg), p.modName(dep))
		} else {
			l.add(0, "import * as dep%d from '../%s/%s';", i, p.pkgName(dep.pkg), p.modName(dep))
		}
	}
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(0, "import { %s, %s } from './%s';", sib.class, camel(sib.helpers[0]), p.modName(sib))
	}
	l.add(0, "")
	call := func(e externalCall) string {
		pkg := ext[e.pkg]
		if pkg.name == ext[m.externals[0].pkg].name {
			return fmt.Sprintf("%s.%s", pkg.name, pkg.funcs[e.fn])
		}
		for _, prev := range m.externals {
			if ext[prev.pkg].name == pkg.name {
				return fmt.Sprintf("%s%s", pkg.name, title(pkg.funcs[prev.fn]))
			}
		}
		return ""
	}

	l.add(0, "export class %s {", m.class)
	if len(m.deps) > 0 {
		l.add(1, "private readonly store: dep0.%s;", p.modules[m.deps[0]].class)
	}
	l.add(1, "private items: Result[] = [];")
	l.add(0, "")
	l.add(1, "constructor(private readonly config: Config) {")
	if len(m.deps) > 0 {
		l.add(2, "this.store = new dep0.%s(config);", p.modules[m.deps[0]].class)
	}
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "get %s(): number {", camel(w))
			l.add(2, "return this.items.length;")
			l.add(1, "}")
			continue
		}
		l.add(1, "%s(value: unknown): Result {", camel(w))
		l.add(2, "let data: any = %s(value);", camel(m.helpers[i%len(m.helpers)]))
		if i == 0 {
			l.add(2, "data = %s(data);", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "data = this.store.%s(data);", camel(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "data = (this as any)[this.config.handler ?? '%s'](data);", camel(m.methods[0]))
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = this.%s(data);", camel(m.methods[i+1]))
		}
		l.add(2, "this.items.push(data);")
		l.add(2, "return data;")
		l.add(1, "}")
	}
	l.add(0, "}")
	l.add(0, "")

	for i, w := range m.helpers {
		l.add(0, "export function %s(value: any): Result {", camel(w))
		l.add(1, "if (value === undefined) {")
		l.add(2, "throw new Error('%s needs a value');", camel(w))
		l.add(1, "}")
		switch {
		case i+1 < len(m.helpers):
			l.add(1, "value = %s(value);", camel(m.helpers[i+1]))
		case m.sibling >= 0:
			l.add(1, "value = %s(value);", camel(p.modules[m.sibling].helpers[0]))
		}
		if i < len(m.externals) && i > 0 {
			l.add(1, "value = %s(value);", call(m.externals[i]))
		}
		if i < len(m.deps) {
			l.add(1, "value = dep%d.%s(value);", i, camel(p.modules[m.deps[i]].helpers[0]))
		}
		if i == 2 && m.dynamic == dynamicEval {
			l.add(1, "value = eval(String(value));")
		}
		l.add(1, "return { %s: value, size: String(value).length };", synthNouns[w[1]])
		l.add(0, "}")
		l.add(0, "")
	}

	l.add(0, "export async function run(argv: string[]): Promise<Result> {")
	l.add(1, "const service = new %s({ name: '%s' });", m.class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(1, "new %s({ name: 'peer' }).%s(argv);", sib.class, camel(sib.methods[0]))
	}
	for i := range m.deps {
		l.add(1, "await dep%d.run(argv);", i)
	}
	if m.dynamic == dynamicImport {
		l.add(1, "const plugin = await import(`./plugins/${argv[0]}`);")
		l.add(1, "await plugin.run(argv);")
	}
	l.add(1, "return service.%s(argv);", camel(m.methods[0]))
	l.add(0, "}")
	return l.String()
}

func packageJSON(ext []externalPackage, main string) string {
	var deps []string
	for _, e := range ext {
		deps = append(deps, fmt.Sprintf("    %q: \"^1.0.0\"", e.name))
	}
	return fmt.Sprintf("{\n  \"name\": \"bench-app\",\n  \"version\": \"1.0.0\",\n  \"main\": %q,\n  \"bin\": { \"bench-app\": %q },\n  \"dependencies\": {\n%s\n  }\n}\n",
		main, main, strings.Join(deps, ",\n"))
}
