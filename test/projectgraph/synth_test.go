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
	synthExternals[core.LanguageCodeJava] = []externalPackage{
		{"org/apache/commons/lang3/StringUtils", []string{"isBlank", "isEmpty", "trim", "join", "split", "capitalize", "abbreviate", "leftPad", "rightPad", "replace"}},
		{"org/apache/commons/io/FileUtils", []string{"readFileToString", "writeStringToFile", "copyFile", "deleteDirectory", "listFiles", "forceMkdir", "moveFile", "sizeOf", "touch", "cleanDirectory"}},
		{"com/google/common/base/Preconditions", []string{"checkNotNull", "checkArgument", "checkState", "checkElementIndex", "checkPositionIndex", "checkPositionIndexes", "verify", "verifyNotNull", "checkIndex", "checkFromToIndex"}},
		{"com/google/common/collect/Lists", []string{"newArrayList", "newLinkedList", "partition", "reverse", "transform", "asList", "cartesianProduct", "charactersOf", "newArrayListWithCapacity", "newCopyOnWriteArrayList"}},
		{"org/slf4j/LoggerFactory", []string{"getLogger", "getILoggerFactory", "getProvider", "reset", "bind", "findServiceProviders", "versionSanityCheck", "reportMultipleBindingAmbiguity", "reportActualBinding", "isAmbiguousProviderList"}},
		{"com/fasterxml/jackson/databind/ObjectMapper", []string{"readValue", "writeValueAsString", "readTree", "convertValue", "valueToTree", "writeValue", "treeToValue", "createObjectNode", "createArrayNode", "registerModule"}},
		{"org/yaml/snakeyaml/Yaml", []string{"load", "loadAs", "loadAll", "dump", "dumpAll", "dumpAs", "compose", "composeAll", "parse", "serialize"}},
		{"org/apache/commons/codec/digest/DigestUtils", []string{"md5Hex", "sha1Hex", "sha256Hex", "sha512Hex", "md5", "sha1", "sha256", "sha512", "getDigest", "updateDigest"}},
		{"org/apache/http/client/utils/URIBuilder", []string{"setScheme", "setHost", "setPort", "setPath", "addParameter", "setParameter", "build", "setFragment", "setUserInfo", "clearParameters"}},
		{"com/google/gson/Gson", []string{"toJson", "fromJson", "toJsonTree", "getAdapter", "newJsonWriter", "newJsonReader", "getDelegateAdapter", "htmlSafe", "serializeNulls", "excluder"}},
		{"org/springframework/util/StringUtils", []string{"hasText", "hasLength", "tokenizeToStringArray", "collectionToDelimitedString", "cleanPath", "getFilename", "applyRelativePath", "capitalize", "uncapitalize", "trimWhitespace"}},
		{"org/apache/commons/collections4/CollectionUtils", []string{"isEmpty", "isNotEmpty", "union", "intersection", "subtract", "filter", "transform", "select", "collect", "containsAny"}},
		{"io/jsonwebtoken/Jwts", []string{"builder", "parser", "parserBuilder", "header", "claims", "jwsHeader", "SIG", "ENC", "KEY", "ZIP"}},
		{"org/bouncycastle/util/encoders/Hex", []string{"encode", "decode", "toHexString", "decodeStrict", "encodeTo", "decodeTo", "getEncoder", "getDecoder", "encodedLength", "decodedLength"}},
		{"org/jsoup/Jsoup", []string{"parse", "connect", "clean", "isValid", "parseBodyFragment", "newSession", "parseFile", "parseUrl", "parseStream", "parseHtml"}},
		{"freemarker/template/TemplateUtil", []string{"process", "getTemplate", "setTemplateLoader", "setDefaultEncoding", "setObjectWrapper", "setSetting", "getSetting", "clearTemplateCache", "removeTemplateFromCache", "getVersion"}},
		{"org/apache/logging/log4j/LogManager", []string{"getLogger", "getRootLogger", "getContext", "shutdown", "getFormatterLogger", "exists", "getFactory", "setFactory", "hasShutdownHook", "getRootContext"}},
		{"com/zaxxer/hikari/HikariUtil", []string{"getConnection", "close", "isRunning", "evictConnection", "suspendPool", "resumePool", "setMetricRegistry", "setHealthCheckRegistry", "validate", "unwrap"}},
		{"org/h2/tools/Server", []string{"createTcpServer", "createWebServer", "createPgServer", "start", "stop", "shutdownTcpServer", "openBrowser", "startWebServer", "main", "runTool"}},
		{"okhttp3/HttpUrl", []string{"parse", "get", "defaultPort", "newBuilder", "resolve", "redact", "topPrivateDomain", "uri", "url", "queryParameter"}},
	}
	synthExternals[core.LanguageCodeGo] = []externalPackage{
		{"github.com/spf13/cobra", []string{"OnInitialize", "MarkFlagRequired", "ExactArgs", "MinimumNArgs", "NoArgs", "ArbitraryArgs", "CheckErr", "AddTemplateFunc", "Eq", "Gt"}},
		{"github.com/spf13/viper", []string{"Get", "GetString", "GetInt", "GetBool", "Set", "SetDefault", "ReadInConfig", "AutomaticEnv", "BindEnv", "Unmarshal"}},
		{"gopkg.in/yaml.v3", []string{"Marshal", "Unmarshal", "NewDecoder", "NewEncoder", "Node", "Kind", "Style", "TypeError", "IsZeroer", "Marshaler"}},
		{"github.com/gin-gonic/gin", []string{"New", "Default", "SetMode", "Mode", "Logger", "Recovery", "BasicAuth", "WrapF", "WrapH", "Bind"}},
		{"github.com/sirupsen/logrus", []string{"Info", "Warn", "Error", "Debug", "WithField", "WithFields", "SetLevel", "SetFormatter", "New", "StandardLogger"}},
		{"go.uber.org/zap", []string{"NewProduction", "NewDevelopment", "NewExample", "L", "S", "ReplaceGlobals", "String", "Int", "Error", "Any"}},
		{"github.com/gorilla/mux", []string{"NewRouter", "Vars", "CurrentRoute", "SetURLVars", "MiddlewareFunc", "CORSMethodMiddleware", "ErrMethodMismatch", "ErrNotFound", "SkipRouter", "RouteMatch"}},
		{"github.com/google/uuid", []string{"New", "NewString", "Parse", "MustParse", "NewRandom", "NewSHA1", "NewMD5", "SetRand", "EnableRandPool", "Validate"}},
		{"github.com/pkg/errors", []string{"New", "Errorf", "Wrap", "Wrapf", "WithMessage", "WithStack", "Cause", "Is", "As", "Unwrap"}},
		{"github.com/stretchr/objx", []string{"New", "MustFromJSON", "FromJSON", "FromURLQuery", "MSI", "Nil", "MustFromBase64", "FromBase64", "MustFromSignedBase64", "FromSignedBase64"}},
		{"golang.org/x/crypto/bcrypt", []string{"GenerateFromPassword", "CompareHashAndPassword", "Cost", "ErrMismatchedHashAndPassword", "ErrHashTooShort", "MinCost", "MaxCost", "DefaultCost", "InvalidCostError", "HashVersionTooNewError"}},
		{"golang.org/x/net/html", []string{"Parse", "ParseFragment", "Render", "EscapeString", "UnescapeString", "NewTokenizer", "NewTokenizerFragment", "ParseWithOptions", "ParseOptionEnableScripting", "ErrBufferExceeded"}},
		{"github.com/golang-jwt/jwt/v5", []string{"Parse", "ParseWithClaims", "NewWithClaims", "New", "GetSigningMethod", "RegisterSigningMethod", "NewParser", "WithValidMethods", "WithLeeway", "NewNumericDate"}},
		{"github.com/go-redis/redis/v8", []string{"NewClient", "NewClusterClient", "NewFailoverClient", "NewRing", "NewUniversalClient", "ParseURL", "SetLogger", "NewScript", "NewSliceCmd", "NewStringCmd"}},
		{"github.com/lib/pq", []string{"Open", "NewConnector", "Array", "CopyIn", "CopyInSchema", "QuoteIdentifier", "QuoteLiteral", "ParseURL", "NewListener", "EnableInfinityTs"}},
		{"github.com/aws/aws-sdk-go/aws", []string{"String", "StringValue", "Int64", "Int64Value", "Bool", "BoolValue", "NewConfig", "Time", "TimeValue", "Float64"}},
		{"google.golang.org/grpc", []string{"Dial", "DialContext", "NewServer", "WithInsecure", "WithBlock", "WithTimeout", "UnaryInterceptor", "Creds", "MaxRecvMsgSize", "WithDefaultCallOptions"}},
		{"github.com/prometheus/client_golang/prometheus", []string{"NewCounter", "NewGauge", "NewHistogram", "NewSummary", "MustRegister", "Register", "NewCounterVec", "NewGaugeVec", "NewRegistry", "Unregister"}},
		{"github.com/hashicorp/go-multierror", []string{"Append", "Flatten", "Prefix", "ListFormatFunc", "Group", "ErrorFormatFunc", "Error", "WrappedErrors", "ErrorOrNil", "Len"}},
		{"github.com/mitchellh/mapstructure", []string{"Decode", "DecodeMetadata", "WeakDecode", "WeakDecodeMetadata", "NewDecoder", "ComposeDecodeHookFunc", "StringToSliceHookFunc", "StringToTimeDurationHookFunc", "RecursiveStructToMapHookFunc", "TextUnmarshallerHookFunc"}},
	}
}

// reachTargets are the keys of every external function of a language.
func reachTargets(language core.LanguageCode) []string {
	var targets []string
	for _, p := range synthExternals[language] {
		for _, f := range p.funcs {
			targets = append(targets, strings.ReplaceAll(p.name, "/", "//")+"//"+f)
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
	core.LanguageCodeGo:         1,
	core.LanguageCodeJava:       1,
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
	case core.LanguageCodeGo:
		p.renderGo(files)
	case core.LanguageCodeJava:
		p.renderJava(files)
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
		if ext := filepath.Ext(name); ext == ".py" || ext == ".js" || ext == ".ts" || ext == ".go" || ext == ".java" {
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

// goModulePath is the module path of the generated Go project.
const goModulePath = "bench.example/app"

func (p *synthProject) renderGo(files map[string]string) {
	ext := synthExternals[core.LanguageCodeGo]
	for _, pkg := range p.packages {
		for _, m := range pkg {
			files[fmt.Sprintf("%s/%s.go", p.pkgName(m.pkg), p.modName(m))] = p.goModule(m, ext)
		}
		first, last := pkg[0], pkg[len(pkg)-1]
		files[fmt.Sprintf("%s/%s.go", p.pkgName(first.pkg), p.pkgName(first.pkg))] = fmt.Sprintf(
			"// Package %s holds the %s modules.\npackage %s\n\n// Run runs the first module.\nfunc Run(argv []string) any {\n\treturn Run%02d(argv)\n}\n",
			p.pkgName(first.pkg), synthNouns[first.methods[0][1]], p.pkgName(first.pkg), first.idx)

		var test lines
		test.add(0, "package %s", p.pkgName(last.pkg))
		test.add(0, "")
		test.add(0, "import \"testing\"")
		test.add(0, "")
		test.add(0, "func TestRun%02d(t *testing.T) {", last.idx)
		test.add(1, "if Run%02d([]string{\"input.yaml\"}) == nil {", last.idx)
		test.add(2, "t.Fatal(\"no result\")")
		test.add(1, "}")
		test.add(1, "_ = %s(1)", goHelper(last, 0))
		test.add(0, "}")
		files[fmt.Sprintf("%s/%s_test.go", p.pkgName(last.pkg), p.modName(last))] = test.String()
	}

	var main lines
	main.add(0, "package main")
	main.add(0, "")
	main.add(0, "import (")
	main.add(1, "\"os\"")
	main.add(0, "")
	for _, pkg := range p.packages {
		main.add(1, "%q", goModulePath+"/"+p.pkgName(pkg[0].pkg))
	}
	main.add(0, ")")
	main.add(0, "")
	main.add(0, "func main() {")
	for _, pkg := range p.packages {
		main.add(1, "%s.Run(os.Args[1:])", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "}")
	files["cmd/app/main.go"] = main.String()

	var mod lines
	mod.add(0, "module %s", goModulePath)
	mod.add(0, "")
	mod.add(0, "go 1.22")
	mod.add(0, "")
	mod.add(0, "require (")
	for _, e := range ext {
		mod.add(1, "%s v1.0.0", e.name)
	}
	mod.add(0, ")")
	files["go.mod"] = mod.String()
}

// goClass and goHelper add the index of the module to the names, because
// the modules of a package share one namespace in Go.
func goClass(m *synthModule) string { return fmt.Sprintf("%s%02d", m.class, m.idx) }

func goHelper(m *synthModule, i int) string {
	return fmt.Sprintf("%s%02d", title(camel(m.helpers[i])), m.idx)
}

func goMethod(w [2]int) string { return title(camel(w)) }

func (p *synthProject) goModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "package %s", p.pkgName(m.pkg))
	l.add(0, "")
	l.add(0, "import (")
	l.add(1, "\"fmt\"")
	if m.dynamic == dynamicAttribute {
		l.add(1, "\"reflect\"")
	}
	if m.dynamic == dynamicImport {
		l.add(1, "\"plugin\"")
	}
	l.add(0, "")
	imported := map[string]string{}
	for _, e := range m.externals {
		pkg := ext[e.pkg]
		if _, ok := imported[pkg.name]; ok {
			continue
		}
		alias := fmt.Sprintf("ext%d", len(imported))
		imported[pkg.name] = alias
		l.add(1, "%s %q", alias, pkg.name)
	}
	// A dependency in the package of the module needs no import.
	dep := func(i int) string {
		if p.modules[m.deps[i]].pkg == m.pkg {
			return ""
		}
		return fmt.Sprintf("dep%d.", i)
	}
	for i, d := range m.deps {
		if dep(i) != "" {
			l.add(1, "dep%d %q", i, goModulePath+"/"+p.pkgName(p.modules[d].pkg))
		}
	}
	l.add(0, ")")
	l.add(0, "")
	call := func(e externalCall) string {
		return imported[ext[e.pkg].name] + "." + ext[e.pkg].funcs[e.fn]
	}

	class := goClass(m)
	l.add(0, "// Notifier gets each result of %s.", class)
	l.add(0, "type Notifier%02d interface {", m.idx)
	l.add(1, "Notify(v any)")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "// %s keeps the state of one %s.", class, synthNouns[m.methods[0][1]])
	l.add(0, "type %s struct {", class)
	l.add(1, "config map[string]string")
	if len(m.deps) > 0 {
		l.add(1, "store  *%s%s", dep(0), goClass(p.modules[m.deps[0]]))
	}
	l.add(1, "items  []Notifier%02d", m.idx)
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "func New%s(config map[string]string) *%s {", class, class)
	l.add(1, "s := &%s{config: config}", class)
	if len(m.deps) > 0 {
		l.add(1, "s.store = %sNew%s(config)", dep(0), goClass(p.modules[m.deps[0]]))
	}
	l.add(1, "return s")
	l.add(0, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(0, "func (s *%s) %s() int {", class, goMethod(w))
			l.add(1, "return len(s.items)")
			l.add(0, "}")
			continue
		}
		l.add(0, "func (s *%s) %s(value any) any {", class, goMethod(w))
		l.add(1, "data := %s(value)", goHelper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(1, "data = %s(data)", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(1, "data = s.store.%s(data)", goMethod(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(1, "reflect.ValueOf(s).MethodByName(s.config[\"handler\"]).Call(nil)")
		}
		if i+1 < len(m.methods)-1 {
			l.add(1, "data = s.%s(data)", goMethod(m.methods[i+1]))
		}
		l.add(1, "for _, item := range s.items {")
		l.add(2, "item.Notify(data)")
		l.add(1, "}")
		l.add(1, "return data")
		l.add(0, "}")
	}
	l.add(0, "")

	for i := range m.helpers {
		l.add(0, "func %s(value any) map[string]any {", goHelper(m, i))
		l.add(1, "if value == nil {")
		l.add(2, "panic(\"%s needs a value\")", goHelper(m, i))
		l.add(1, "}")
		switch {
		case i+1 < len(m.helpers):
			l.add(1, "value = %s(value)", goHelper(m, i+1))
		case m.sibling >= 0:
			l.add(1, "value = %s(value)", goHelper(p.modules[m.sibling], 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(1, "value = %s(value)", call(m.externals[i]))
		}
		if i < len(m.deps) {
			l.add(1, "value = %s%s(value)", dep(i), goHelper(p.modules[m.deps[i]], 0))
		}
		l.add(1, "return map[string]any{%q: value, \"size\": len(fmt.Sprint(value))}", synthNouns[m.helpers[i][1]])
		l.add(0, "}")
		l.add(0, "")
	}

	l.add(0, "func Run%02d(argv []string) any {", m.idx)
	l.add(1, "service := New%s(map[string]string{\"name\": %q})", class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(1, "peer := New%s(nil)", goClass(sib))
		l.add(1, "peer.%s(argv)", goMethod(sib.methods[0]))
	}
	for i, d := range m.deps {
		l.add(1, "%sRun%02d(argv)", dep(i), p.modules[d].idx)
	}
	if m.dynamic == dynamicImport {
		l.add(1, "if p, err := plugin.Open(argv[0]); err == nil {")
		l.add(2, "run, _ := p.Lookup(\"Run\")")
		l.add(2, "run.(func([]string))(argv)")
		l.add(1, "}")
	}
	l.add(1, "return service.%s(argv)", goMethod(m.methods[0]))
	l.add(0, "}")
	return l.String()
}

// javaRoot holds the sources of the generated Java project.
const javaRoot = "src/main/java/bench/"

func (p *synthProject) javaPackage(pkg int) string { return "bench." + p.pkgName(pkg) }

// javaClassName returns the class of a module, as Go does with the index of
// the module, because the modules of a package share one namespace.
func javaClassName(m *synthModule) string { return goClass(m) }

func javaHelper(m *synthModule, i int) string {
	return fmt.Sprintf("%s%02d", camel(m.helpers[i]), m.idx)
}

func (p *synthProject) renderJava(files map[string]string) {
	ext := synthExternals[core.LanguageCodeJava]
	for _, pkg := range p.packages {
		for _, m := range pkg {
			files[fmt.Sprintf("%s%s/%s.java", javaRoot, p.pkgName(m.pkg), javaClassName(m))] = p.javaModule(m, ext)
		}
		first, last := pkg[0], pkg[len(pkg)-1]
		files[fmt.Sprintf("%s%s/Package.java", javaRoot, p.pkgName(first.pkg))] = fmt.Sprintf(
			"package %s;\n\npublic final class Package {\n    public static Object run(String[] argv) {\n        return %s.run%02d(argv);\n    }\n}\n",
			p.javaPackage(first.pkg), javaClassName(first), first.idx)

		var test lines
		test.add(0, "package %s;", p.javaPackage(last.pkg))
		test.add(0, "")
		test.add(0, "import org.junit.jupiter.api.Test;")
		test.add(0, "")
		test.add(0, "class %sTest {", javaClassName(last))
		test.add(1, "@Test")
		test.add(1, "void runs() {")
		test.add(2, "%s.run%02d(new String[] {\"input.yaml\"});", javaClassName(last), last.idx)
		test.add(2, "%s.%s(1);", javaClassName(last), javaHelper(last, 0))
		test.add(1, "}")
		test.add(0, "}")
		files[fmt.Sprintf("src/test/java/bench/%s/%sTest.java", p.pkgName(last.pkg), javaClassName(last))] = test.String()
	}

	var main lines
	main.add(0, "package bench;")
	main.add(0, "")
	main.add(0, "public class App {")
	main.add(1, "public static void main(String[] args) {")
	for _, pkg := range p.packages {
		main.add(2, "%s.Package.run(args);", p.javaPackage(pkg[0].pkg))
	}
	main.add(1, "}")
	main.add(0, "}")
	files[javaRoot+"App.java"] = main.String()
}

func (p *synthProject) javaModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "package %s;", p.javaPackage(m.pkg))
	l.add(0, "")
	imported := map[string]string{}
	for _, e := range m.externals {
		pkg := ext[e.pkg]
		if _, ok := imported[pkg.name]; ok {
			continue
		}
		imported[pkg.name] = pkg.name[strings.LastIndex(pkg.name, "/")+1:]
		l.add(0, "import %s;", strings.ReplaceAll(pkg.name, "/", "."))
	}
	for _, d := range m.deps {
		if dep := p.modules[d]; dep.pkg != m.pkg {
			l.add(0, "import %s.%s;", p.javaPackage(dep.pkg), javaClassName(dep))
		}
	}
	l.add(0, "import java.util.ArrayList;")
	l.add(0, "import java.util.List;")
	l.add(0, "")
	call := func(e externalCall) string {
		return imported[ext[e.pkg].name] + "." + ext[e.pkg].funcs[e.fn]
	}

	class := javaClassName(m)
	l.add(0, "public class %s {", class)
	if len(m.deps) > 0 {
		l.add(1, "private final %s store;", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "private final Object config;")
	l.add(1, "private final List<Listener%02d> items = new ArrayList<>();", m.idx)
	l.add(0, "")
	l.add(1, "public %s(Object config) {", class)
	l.add(2, "this.config = config;")
	if len(m.deps) > 0 {
		l.add(2, "this.store = new %s(config);", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "public int %s() {", camel(w))
			l.add(2, "return items.size();")
			l.add(1, "}")
			continue
		}
		l.add(1, "public Object %s(Object value) {", camel(w))
		l.add(2, "Object data = %s(value);", javaHelper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(2, "data = %s(data);", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "data = store.%s(data);", camel(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "data = getClass().getMethod(String.valueOf(config)).invoke(this, data);")
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = this.%s(data);", camel(m.methods[i+1]))
		}
		l.add(2, "for (Listener%02d item : items) {", m.idx)
		l.add(3, "item.onEvent(data);")
		l.add(2, "}")
		l.add(2, "return data;")
		l.add(1, "}")
	}
	l.add(0, "")

	for i := range m.helpers {
		l.add(1, "public static Object %s(Object value) {", javaHelper(m, i))
		l.add(2, "if (value == null) {")
		l.add(3, "throw new IllegalArgumentException(\"%s needs a value\");", javaHelper(m, i))
		l.add(2, "}")
		switch {
		case i+1 < len(m.helpers):
			l.add(2, "value = %s(value);", javaHelper(m, i+1))
		case m.sibling >= 0:
			sib := p.modules[m.sibling]
			l.add(2, "value = %s.%s(value);", javaClassName(sib), javaHelper(sib, 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(2, "value = %s(value);", call(m.externals[i]))
		}
		if i < len(m.deps) {
			dep := p.modules[m.deps[i]]
			l.add(2, "value = %s.%s(value);", javaClassName(dep), javaHelper(dep, 0))
		}
		l.add(2, "return String.valueOf(value);")
		l.add(1, "}")
		l.add(0, "")
	}

	l.add(1, "public static Object run%02d(String[] argv) {", m.idx)
	l.add(2, "%s service = new %s(\"%s\");", class, class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(2, "new %s(null).%s(argv);", javaClassName(sib), camel(sib.methods[0]))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(2, "%s.run%02d(argv);", javaClassName(dep), dep.idx)
	}
	if m.dynamic == dynamicImport {
		l.add(2, "try {")
		l.add(3, "Class.forName(\"bench.plugins.\" + argv[0]).getMethod(\"run\").invoke(null);")
		l.add(2, "} catch (ReflectiveOperationException e) {")
		l.add(3, "throw new IllegalStateException(e);")
		l.add(2, "}")
	}
	l.add(2, "return service.%s(argv);", camel(m.methods[0]))
	l.add(1, "}")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "interface Listener%02d {", m.idx)
	l.add(1, "void onEvent(Object v);")
	l.add(0, "}")
	return l.String()
}
