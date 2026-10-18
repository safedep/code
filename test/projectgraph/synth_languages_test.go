//go:build projectgraph

package projectgraph

import (
	"fmt"
	"strings"

	"github.com/safedep/code/core"
)

func init() {
	synthExternals[core.LanguageCodeCSharp] = []externalPackage{
		{"Newtonsoft/Json/JsonConvert", []string{"SerializeObject", "DeserializeObject", "PopulateObject", "SerializeXmlNode", "DeserializeXmlNode", "ToString", "SerializeXNode", "DeserializeXNode", "DeserializeAnonymousType", "EnsureObjectType"}},
		{"Dapper/SqlMapper", []string{"Query", "QueryFirst", "QuerySingle", "Execute", "ExecuteScalar", "QueryMultiple", "ExecuteReader", "AddTypeHandler", "ResetTypeHandlers", "PurgeQueryCache"}},
		{"Serilog/Log", []string{"Information", "Warning", "Error", "Debug", "Fatal", "Verbose", "Write", "ForContext", "CloseAndFlush", "BindMessageTemplate"}},
		{"AutoMapper/Mapper", []string{"Map", "Initialize", "AssertConfigurationIsValid", "Reset", "DynamicMap", "CreateMap", "FindTypeMapFor", "GetAllTypeMaps", "ProjectTo", "ConfigurationProvider"}},
		{"Polly/Policy", []string{"Handle", "HandleResult", "Timeout", "TimeoutAsync", "Bulkhead", "BulkheadAsync", "NoOp", "NoOpAsync", "Wrap", "WrapAsync"}},
		{"FluentValidation/ValidatorOptions", []string{"Global", "PropertyNameResolver", "DisplayNameResolver", "LanguageManager", "CascadeMode", "ErrorCodeResolver", "MessageFormatterFactory", "PropertyChainSeparator", "ValidatorSelectors", "DefaultClassLevelCascadeMode"}},
		{"MediatR/Mediator", []string{"Send", "Publish", "CreateStream", "SendAsync", "PublishAsync", "PublishCore", "GetHandler", "Configure", "Resolve", "Register"}},
		{"RestSharp/RestClient", []string{"Execute", "ExecuteAsync", "Get", "Post", "Put", "Delete", "Patch", "Head", "Options", "DownloadData"}},
		{"YamlDotNet/Serialization/Deserializer", []string{"Deserialize", "Builder", "FromValueDeserializer", "WithNamingConvention", "WithTypeConverter", "WithTagMapping", "IgnoreUnmatchedProperties", "WithNodeDeserializer", "WithObjectFactory", "Build"}},
		{"Npgsql/NpgsqlConnection", []string{"Open", "OpenAsync", "Close", "BeginTransaction", "CreateCommand", "ChangeDatabase", "GlobalTypeMapper", "ClearPool", "ClearAllPools", "UnprepareAll"}},
		{"StackExchange/Redis/ConnectionMultiplexer", []string{"Connect", "ConnectAsync", "GetDatabase", "GetServer", "GetSubscriber", "Close", "Dispose", "GetEndPoints", "Configure", "SetStreamLogger"}},
		{"Microsoft/Extensions/Logging/LoggerFactory", []string{"Create", "AddProvider", "CreateLogger", "Dispose", "AddConsole", "AddDebug", "AddEventLog", "AddFilter", "SetMinimumLevel", "ClearProviders"}},
		{"Hangfire/BackgroundJob", []string{"Enqueue", "Schedule", "Delete", "Requeue", "ContinueJobWith", "ContinueWith", "Reschedule", "Trigger", "RecurringJob", "AddOrUpdate"}},
		{"Quartz/SchedulerBuilder", []string{"Create", "UseDefaultThreadPool", "UseInMemoryStore", "UsePersistentStore", "UseTimeZoneConverter", "UseJobFactory", "BuildScheduler", "WithId", "WithName", "Properties"}},
		{"Sodium/PasswordHash", []string{"ArgonHashString", "ArgonHashStringVerify", "ScryptHashString", "ScryptHashStringVerify", "ArgonHashBinary", "ScryptHashBinary", "ArgonPasswordNeedsRehash", "ScryptPasswordNeedsRehash", "ArgonGenerateSalt", "ScryptGenerateSalt"}},
		{"HtmlAgilityPack/HtmlDocument", []string{"Load", "LoadHtml", "Save", "CreateElement", "CreateTextNode", "GetElementbyId", "DetectEncoding", "HtmlEncode", "IsWhiteSpace", "CreateAttribute"}},
		{"Azure/Storage/Blobs/BlobClient", []string{"Upload", "UploadAsync", "Download", "DownloadAsync", "DeleteIfExists", "Exists", "GetProperties", "SetMetadata", "StartCopyFromUri", "GenerateSasUri"}},
		{"Amazon/S3/AmazonS3Client", []string{"PutObject", "GetObject", "DeleteObject", "ListObjects", "CopyObject", "GetObjectMetadata", "PutBucket", "DeleteBucket", "ListBuckets", "GetPreSignedURL"}},
		{"Grpc/Net/Client/GrpcChannel", []string{"ForAddress", "ShutdownAsync", "Dispose", "CreateCallInvoker", "ConnectAsync", "State", "Target", "WaitForStateChangedAsync", "HttpHandler", "Credentials"}},
		{"CsvHelper/CsvReader", []string{"GetRecords", "Read", "ReadHeader", "GetField", "GetRecord", "TryGetField", "ValidateHeader", "Configuration", "Context", "Parser"}},
	}
	synthExternals[core.LanguageCodePHP] = []externalPackage{
		{"GuzzleHttp/Psr7/Utils", []string{"streamFor", "tryFopen", "copyToString", "copyToStream", "hash", "readLine", "redactUserInfo", "uriFor", "caselessRemove", "modifyRequest"}},
		{"Monolog/Registry", []string{"getInstance", "addLogger", "removeLogger", "hasLogger", "clear", "__callStatic", "loggers", "logger", "instance", "registry"}},
		{"Symfony/Component/Yaml/Yaml", []string{"parse", "parseFile", "dump", "PARSE_OBJECT", "DUMP_OBJECT", "PARSE_CONSTANT", "DUMP_MULTI_LINE_LITERAL_BLOCK", "PARSE_CUSTOM_TAGS", "DUMP_NULL_AS_TILDE", "DUMP_EMPTY_ARRAY_AS_SEQUENCE"}},
		{"Illuminate/Support/Str", []string{"slug", "random", "uuid", "camel", "snake", "studly", "plural", "singular", "limit", "contains"}},
		{"Illuminate/Support/Facades/DB", []string{"table", "select", "insert", "update", "delete", "statement", "raw", "transaction", "connection", "unprepared"}},
		{"Carbon/Carbon", []string{"now", "parse", "create", "createFromFormat", "today", "tomorrow", "yesterday", "setTestNow", "instance", "createFromTimestamp"}},
		{"Firebase/JWT/JWT", []string{"encode", "decode", "sign", "verify", "jsonEncode", "jsonDecode", "urlsafeB64Encode", "urlsafeB64Decode", "constantTimeEquals", "getKey"}},
		{"League/Flysystem/Util", []string{"normalizePath", "normalizeRelativePath", "dirname", "pathinfo", "map", "guessMimeType", "contentSize", "emulateDirectories", "ensureConfig", "rewindStream"}},
		{"Ramsey/Uuid/Uuid", []string{"uuid1", "uuid3", "uuid4", "uuid5", "uuid6", "uuid7", "fromString", "fromBytes", "isValid", "fromInteger"}},
		{"Twig/Environment", []string{"render", "load", "display", "createTemplate", "addExtension", "addFilter", "addFunction", "addGlobal", "setLoader", "enableDebug"}},
		{"Doctrine/DBAL/DriverManager", []string{"getConnection", "getAvailableDrivers", "parseDatabaseUrl", "createConnection", "registerDriver", "getDriver", "getParams", "configure", "reset", "connect"}},
		{"PHPMailer/PHPMailer/PHPMailer", []string{"send", "addAddress", "setFrom", "addAttachment", "isSMTP", "isHTML", "addReplyTo", "addCC", "addBCC", "validateAddress"}},
		{"Predis/Client", []string{"get", "set", "del", "exists", "expire", "incr", "decr", "hget", "hset", "pipeline"}},
		{"Aws/S3/S3Client", []string{"putObject", "getObject", "deleteObject", "listObjects", "copyObject", "headObject", "createBucket", "deleteBucket", "listBuckets", "createPresignedRequest"}},
		{"Intervention/Image/ImageManagerStatic", []string{"make", "canvas", "cache", "configure", "resize", "crop", "encode", "save", "fit", "orientate"}},
		{"Dompdf/Dompdf", []string{"loadHtml", "render", "stream", "output", "setPaper", "setOptions", "loadHtmlFile", "getCanvas", "setBasePath", "getOptions"}},
		{"Respect/Validation/Validator", []string{"email", "numericVal", "stringType", "intVal", "length", "notEmpty", "url", "ip", "date", "regex"}},
		{"Stripe/Charge", []string{"create", "retrieve", "update", "all", "capture", "search", "refund", "close", "markAsFraudulent", "markAsSafe"}},
		{"Faker/Factory", []string{"create", "getDefaultLocale", "setDefaultLocale", "getProviderClassname", "findProviderClassname", "addProvider", "getProviders", "seed", "unique", "optional"}},
		{"Psr/Log/LoggerTrait", []string{"emergency", "alert", "critical", "error", "warning", "notice", "info", "debug", "log", "interpolate"}},
	}
	synthExternals[core.LanguageCodeRuby] = []externalPackage{
		{"JSON", []string{"parse", "generate", "dump", "load", "pretty_generate", "fast_generate", "parse!", "load_file", "create_id", "state"}},
		{"YAML", []string{"load", "safe_load", "dump", "load_file", "safe_load_file", "parse", "parse_file", "unsafe_load", "load_stream", "to_json"}},
		{"Nokogiri", []string{"HTML", "XML", "parse", "Slop", "XSLT", "HTML5", "make", "uses_libxml?", "jruby?", "install_default_aliases"}},
		{"Faraday", []string{"new", "get", "post", "put", "delete", "head", "patch", "default_connection", "default_adapter", "register_middleware"}},
		{"HTTParty", []string{"get", "post", "put", "delete", "head", "patch", "options", "move", "copy", "normalize_param"}},
		{"Redis", []string{"new", "current", "exists_returns_integer", "raise_deprecations", "silence_deprecations", "deprecate!", "connect", "url", "driver", "reconnect"}},
		{"Sidekiq", []string{"configure_server", "configure_client", "redis", "redis_pool", "logger", "options", "server?", "load_json", "dump_json", "default_job_options"}},
		{"Rails", []string{"application", "env", "root", "logger", "cache", "configuration", "groups", "public_path", "autoloaders", "backtrace_cleaner"}},
		{"ActiveRecord/Base", []string{"establish_connection", "connection", "transaction", "find_by_sql", "create", "where", "find", "all", "first", "destroy_all"}},
		{"Aws/S3/Client", []string{"new", "put_object", "get_object", "delete_object", "list_objects", "copy_object", "head_object", "create_bucket", "delete_bucket", "list_buckets"}},
		{"Net/HTTP", []string{"get", "post", "start", "new", "get_response", "post_form", "version_1_2", "proxy_class?", "Proxy", "socket_type"}},
		{"Open3", []string{"capture2", "capture2e", "capture3", "popen2", "popen2e", "popen3", "pipeline", "pipeline_r", "pipeline_w", "pipeline_rw"}},
		{"FileUtils", []string{"rm_rf", "mkdir_p", "cp", "cp_r", "mv", "rm", "touch", "ln_s", "chmod", "chown"}},
		{"Digest/SHA256", []string{"hexdigest", "digest", "base64digest", "file", "new", "update", "reset", "bubblebabble", "length", "block_length"}},
		{"ERB", []string{"new", "result", "version", "util", "run", "src", "filename", "lineno", "location", "def_method"}},
		{"Thor", []string{"start", "desc", "method_option", "option", "class_option", "subcommand", "default_task", "map", "package_name", "check_unknown_options!"}},
		{"Rack/Utils", []string{"escape", "unescape", "parse_query", "build_query", "escape_html", "status_code", "parse_nested_query", "build_nested_query", "set_cookie_header", "delete_cookie_header"}},
		{"Jwt", []string{"encode", "decode", "configure", "configuration", "gem_version", "openssl_3?", "rbnacl?", "rbnacl_6_or_greater?", "openssl_3_hmac_empty_key_regression?", "decode_verify_signature"}},
		{"BCrypt/Password", []string{"create", "new", "valid_hash?", "cost", "salt", "version", "checksum", "to_s", "is_password?", "secure_compare"}},
		{"Mail", []string{"new", "deliver", "defaults", "read", "read_from_string", "all", "find", "first", "last", "delete_all"}},
	}
	synthExternals[core.LanguageCodeRust] = []externalPackage{
		{"serde_json", []string{"to_string", "from_str", "to_vec", "from_slice", "to_value", "from_value", "to_writer", "from_reader", "to_string_pretty", "to_writer_pretty"}},
		{"serde_yaml", []string{"to_string", "from_str", "to_value", "from_value", "to_writer", "from_reader", "from_slice", "to_vec", "with", "value"}},
		{"reqwest", []string{"get", "Client", "ClientBuilder", "Proxy", "Url", "Body", "multipart", "redirect", "header", "Method"}},
		{"tokio/fs", []string{"read", "write", "read_to_string", "remove_file", "create_dir_all", "copy", "rename", "metadata", "read_dir", "remove_dir_all"}},
		{"regex", []string{"escape", "Regex", "RegexSet", "RegexBuilder", "Captures", "Match", "Replacer", "bytes", "Error", "SubCaptureMatches"}},
		{"base64", []string{"encode", "decode", "encode_config", "decode_config", "encode_engine", "decode_engine", "engine", "prelude", "display", "alphabet"}},
		{"chrono", []string{"Utc", "Local", "NaiveDate", "NaiveDateTime", "Duration", "DateTime", "TimeZone", "FixedOffset", "Weekday", "Month"}},
		{"log", []string{"info", "warn", "error", "debug", "trace", "set_max_level", "max_level", "set_logger", "logger", "log_enabled"}},
		{"rand", []string{"random", "thread_rng", "rngs", "seq", "distributions", "Rng", "SeedableRng", "prelude", "RngCore", "CryptoRng"}},
		{"sha2", []string{"Sha256", "Sha512", "Sha224", "Sha384", "Digest", "digest", "compress256", "compress512", "Sha512_256", "Sha512_224"}},
		{"uuid", []string{"Uuid", "Builder", "Variant", "Version", "fmt", "uuid", "Error", "Bytes", "timestamp", "NoContext"}},
		{"sqlx", []string{"query", "query_as", "query_scalar", "migrate", "Pool", "postgres", "sqlite", "mysql", "Row", "Executor"}},
		{"redis", []string{"Client", "cmd", "pipe", "transaction", "Connection", "Commands", "parse_redis_url", "Script", "pack_command", "from_redis_value"}},
		{"hyper", []string{"Server", "Client", "Body", "Request", "Response", "service", "StatusCode", "Method", "Uri", "header"}},
		{"clap", []string{"Command", "Arg", "ArgMatches", "Parser", "ValueEnum", "Subcommand", "Args", "arg", "value_parser", "crate_version"}},
		{"anyhow", []string{"anyhow", "bail", "ensure", "Error", "Result", "Context", "Chain", "format_err", "Ok", "private"}},
		{"flate2", []string{"Compression", "Crc", "GzBuilder", "Decompress", "Compress", "read", "write", "bufread", "Status", "FlushCompress"}},
		{"tera", []string{"Tera", "Context", "Template", "Value", "to_value", "try_get_value", "escape_html", "dotted_pointer", "get_json_pointer", "Error"}},
		{"walkdir", []string{"WalkDir", "DirEntry", "IntoIter", "FilterEntry", "Error", "DirEntryExt", "Result", "max_depth", "min_depth", "follow_links"}},
		{"tempfile", []string{"tempfile", "tempdir", "NamedTempFile", "TempDir", "Builder", "tempfile_in", "tempdir_in", "SpooledTempFile", "spooled_tempfile", "env"}},
	}
	entryFiles[core.LanguageCodeCSharp] = 1
	entryFiles[core.LanguageCodePHP] = 1
	entryFiles[core.LanguageCodeRuby] = 1
	entryFiles[core.LanguageCodeRust] = 1
}

// externalRef returns the qualified name of an external, as the source of
// the language writes it, and the name that a use or an import binds.
func externalRef(pkg externalPackage, sep string) (string, string) {
	name := strings.ReplaceAll(pkg.name, "/", sep)
	return name, name[strings.LastIndex(name, sep)+len(sep):]
}

func pascal(w [2]int) string { return title(camel(w)) }

// csharpNamespace returns the namespace of a package.
func (p *synthProject) csharpNamespace(pkg int) string { return "Bench." + title(p.pkgName(pkg)) }

// csharpPath, csharpPackageFiles and csharpEntries write the modules as Java does, with one namespace per
// package, properties and a static Package class per namespace.
func (p *synthProject) csharpPath(m *synthModule) string {
	return fmt.Sprintf("src/%s/%s.cs", title(p.pkgName(m.pkg)), javaClassName(m))
}

// csharpPackageFiles writes the barrel file and the test file of a package.
func (p *synthProject) csharpPackageFiles(pkg []*synthModule, files map[string]string) {
	first, last := pkg[0], pkg[len(pkg)-1]
	files[fmt.Sprintf("src/%s/Package.cs", title(p.pkgName(first.pkg)))] = fmt.Sprintf(
		"namespace %s;\n\npublic static class Package\n{\n    public static object Run(string[] argv) => %s.Run%02d(argv);\n}\n",
		p.csharpNamespace(first.pkg), javaClassName(first), first.idx)

	var test lines
	test.add(0, "using Xunit;")
	test.add(0, "using %s;", p.csharpNamespace(last.pkg))
	test.add(0, "")
	test.add(0, "public class %sTests", javaClassName(last))
	test.add(0, "{")
	test.add(1, "[Fact]")
	test.add(1, "public void Runs()")
	test.add(1, "{")
	test.add(2, "%s.Run%02d(new[] { \"input.yaml\" });", javaClassName(last), last.idx)
	test.add(2, "%s.%s(1);", javaClassName(last), title(javaHelper(last, 0)))
	test.add(1, "}")
	test.add(0, "}")
	files[fmt.Sprintf("tests/%s/%sTests.cs", title(p.pkgName(last.pkg)), javaClassName(last))] = test.String()
}

// csharpEntries writes the entry files and the manifests of the project.
func (p *synthProject) csharpEntries(ext []externalPackage, files map[string]string) {

	var main lines
	main.add(0, "public static class Program")
	main.add(0, "{")
	main.add(1, "public static void Main(string[] args)")
	main.add(1, "{")
	for _, pkg := range p.packages {
		main.add(2, "%s.Package.Run(args);", p.csharpNamespace(pkg[0].pkg))
	}
	main.add(1, "}")
	main.add(0, "}")
	files["src/Program.cs"] = main.String()
}

func (p *synthProject) csharpModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "using System;")
	l.add(0, "using System.Collections.Generic;")
	imported := map[string]string{}
	for _, e := range m.externals {
		pkg := ext[e.pkg]
		if _, ok := imported[pkg.name]; ok {
			continue
		}
		qualified, class := externalRef(pkg, ".")
		imported[pkg.name] = class
		l.add(0, "using %s;", qualified[:strings.LastIndex(qualified, ".")])
	}
	for _, d := range m.deps {
		if dep := p.modules[d]; dep.pkg != m.pkg {
			l.add(0, "using %s;", p.csharpNamespace(dep.pkg))
		}
	}
	l.add(0, "")
	l.add(0, "namespace %s;", p.csharpNamespace(m.pkg))
	l.add(0, "")
	call := func(e externalCall) string {
		return imported[ext[e.pkg].name] + "." + ext[e.pkg].funcs[e.fn]
	}
	helper := func(m *synthModule, i int) string { return title(javaHelper(m, i)) }

	class := javaClassName(m)
	l.add(0, "public class %s", class)
	l.add(0, "{")
	if len(m.deps) > 0 {
		l.add(1, "private readonly %s store;", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "private readonly object config;")
	l.add(1, "private readonly List<IListener%02d> items = new();", m.idx)
	l.add(1, "public string Name { get; } = \"%s\";", p.modName(m))
	l.add(0, "")
	l.add(1, "public %s(object config)", class)
	l.add(1, "{")
	l.add(2, "this.config = config;")
	if len(m.deps) > 0 {
		l.add(2, "store = new %s(config);", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "public int %s() => items.Count;", pascal(w))
			continue
		}
		l.add(1, "public object %s(object value)", pascal(w))
		l.add(1, "{")
		l.add(2, "var data = %s(value);", helper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(2, "data = %s(data);", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "data = store.%s(data);", pascal(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "data = GetType().GetMethod(config.ToString()).Invoke(this, new[] { data });")
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = this.%s(data);", pascal(m.methods[i+1]))
		}
		l.add(2, "items.ForEach(item => item.OnEvent(data));")
		l.add(2, "return data;")
		l.add(1, "}")
	}
	l.add(0, "")
	for i := range m.helpers {
		l.add(1, "public static object %s(object value)", helper(m, i))
		l.add(1, "{")
		l.add(2, "ArgumentNullException.ThrowIfNull(value);")
		switch {
		case i+1 < len(m.helpers):
			l.add(2, "value = %s(value);", helper(m, i+1))
		case m.sibling >= 0:
			sib := p.modules[m.sibling]
			l.add(2, "value = %s.%s(value);", javaClassName(sib), helper(sib, 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(2, "value = %s(value);", call(m.externals[i]))
		}
		if i < len(m.deps) {
			dep := p.modules[m.deps[i]]
			l.add(2, "value = %s.%s(value);", javaClassName(dep), helper(dep, 0))
		}
		l.add(2, "return value.ToString();")
		l.add(1, "}")
		l.add(0, "")
	}
	l.add(1, "public static object Run%02d(string[] argv)", m.idx)
	l.add(1, "{")
	l.add(2, "var service = new %s(\"%s\");", class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(2, "new %s(null).%s(argv);", javaClassName(sib), pascal(sib.methods[0]))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(2, "%s.Run%02d(argv);", javaClassName(dep), dep.idx)
	}
	if m.dynamic == dynamicImport {
		l.add(2, "Type.GetType(\"Bench.Plugins.\" + argv[0]).GetMethod(\"Run\").Invoke(null, null);")
	}
	l.add(2, "return service.%s(argv);", pascal(m.methods[0]))
	l.add(1, "}")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "public interface IListener%02d", m.idx)
	l.add(0, "{")
	l.add(1, "void OnEvent(object v);")
	l.add(0, "}")
	return l.String()
}

// phpNamespace returns the namespace of a package.
func (p *synthProject) phpNamespace(pkg int) string { return `Bench\` + title(p.pkgName(pkg)) }

// phpPath, phpPackageFiles and phpEntries write the modules as Java does, with one namespace per
// package under a PSR-4 prefix and a script under bin.
func (p *synthProject) phpPath(m *synthModule) string {
	return fmt.Sprintf("src/%s/%s.php", title(p.pkgName(m.pkg)), javaClassName(m))
}

// phpPackageFiles writes the barrel file and the test file of a package.
func (p *synthProject) phpPackageFiles(pkg []*synthModule, files map[string]string) {
	first, last := pkg[0], pkg[len(pkg)-1]
	files[fmt.Sprintf("src/%s/Package.php", title(p.pkgName(first.pkg)))] = fmt.Sprintf(
		"<?php\n\nnamespace %s;\n\nfinal class Package\n{\n    public static function run(array $argv)\n    {\n        return %s::run%02d($argv);\n    }\n}\n",
		p.phpNamespace(first.pkg), javaClassName(first), first.idx)

	var test lines
	test.add(0, "<?php")
	test.add(0, "")
	test.add(0, "use PHPUnit\\Framework\\TestCase;")
	test.add(0, "use %s\\%s;", p.phpNamespace(last.pkg), javaClassName(last))
	test.add(0, "")
	test.add(0, "final class %sTest extends TestCase", javaClassName(last))
	test.add(0, "{")
	test.add(1, "public function testRuns(): void")
	test.add(1, "{")
	test.add(2, "%s::run%02d(['input.yaml']);", javaClassName(last), last.idx)
	test.add(2, "%s::%s(1);", javaClassName(last), javaHelper(last, 0))
	test.add(1, "}")
	test.add(0, "}")
	files[fmt.Sprintf("tests/%s/%sTest.php", title(p.pkgName(last.pkg)), javaClassName(last))] = test.String()
}

// phpEntries writes the entry files and the manifests of the project.
func (p *synthProject) phpEntries(ext []externalPackage, files map[string]string) {

	var main lines
	main.add(0, "<?php")
	main.add(0, "")
	main.add(0, "require __DIR__ . '/../vendor/autoload.php';")
	main.add(0, "")
	for _, pkg := range p.packages {
		main.add(0, "\\%s\\Package::run($argv);", p.phpNamespace(pkg[0].pkg))
	}
	files["bin/app.php"] = main.String()
	files["composer.json"] = "{\n    \"name\": \"bench/app\",\n    \"autoload\": {\"psr-4\": {\"Bench\\\\\": \"src/\"}},\n    \"bin\": [\"bin/app.php\"]\n}\n"
}

func (p *synthProject) phpModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "<?php")
	l.add(0, "")
	l.add(0, "namespace %s;", p.phpNamespace(m.pkg))
	l.add(0, "")
	imported := map[string]string{}
	for _, e := range m.externals {
		pkg := ext[e.pkg]
		if _, ok := imported[pkg.name]; ok {
			continue
		}
		qualified, class := externalRef(pkg, `\`)
		imported[pkg.name] = class
		l.add(0, "use %s;", qualified)
	}
	for _, d := range m.deps {
		if dep := p.modules[d]; dep.pkg != m.pkg {
			l.add(0, "use %s\\%s;", p.phpNamespace(dep.pkg), javaClassName(dep))
		}
	}
	l.add(0, "")
	call := func(e externalCall) string {
		return imported[ext[e.pkg].name] + "::" + ext[e.pkg].funcs[e.fn]
	}

	class := javaClassName(m)
	l.add(0, "class %s", class)
	l.add(0, "{")
	if len(m.deps) > 0 {
		l.add(1, "private %s $store;", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "private array $items = [];")
	l.add(0, "")
	l.add(1, "public function __construct(private $config)")
	l.add(1, "{")
	if len(m.deps) > 0 {
		l.add(2, "$this->store = new %s($config);", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "public function %s(): int", camel(w))
			l.add(1, "{")
			l.add(2, "return count($this->items);")
			l.add(1, "}")
			continue
		}
		l.add(1, "public function %s($value)", camel(w))
		l.add(1, "{")
		l.add(2, "$data = self::%s($value);", javaHelper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(2, "$data = %s($data);", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "$data = $this->store->%s($data);", camel(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "$data = $this->{$this->config}($data);")
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "$data = $this->%s($data);", camel(m.methods[i+1]))
		}
		l.add(2, "array_map(fn ($item) => $item->onEvent($data), $this->items);")
		l.add(2, "return $data;")
		l.add(1, "}")
	}
	l.add(0, "")
	for i := range m.helpers {
		l.add(1, "public static function %s($value)", javaHelper(m, i))
		l.add(1, "{")
		l.add(2, "if ($value === null) {")
		l.add(3, "throw new \\InvalidArgumentException('%s needs a value');", javaHelper(m, i))
		l.add(2, "}")
		switch {
		case i+1 < len(m.helpers):
			l.add(2, "$value = self::%s($value);", javaHelper(m, i+1))
		case m.sibling >= 0:
			sib := p.modules[m.sibling]
			l.add(2, "$value = %s::%s($value);", javaClassName(sib), javaHelper(sib, 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(2, "$value = %s($value);", call(m.externals[i]))
		}
		if i < len(m.deps) {
			dep := p.modules[m.deps[i]]
			l.add(2, "$value = %s::%s($value);", javaClassName(dep), javaHelper(dep, 0))
		}
		l.add(2, "return (string) $value;")
		l.add(1, "}")
		l.add(0, "")
	}
	l.add(1, "public static function run%02d(array $argv)", m.idx)
	l.add(1, "{")
	l.add(2, "$service = new %s('%s');", class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(2, "(new %s(null))->%s($argv);", javaClassName(sib), camel(sib.methods[0]))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(2, "%s::run%02d($argv);", javaClassName(dep), dep.idx)
	}
	switch m.dynamic {
	case dynamicImport:
		l.add(2, "require __DIR__ . '/plugins/' . $argv[0] . '.php';")
	case dynamicEval:
		l.add(2, "eval($argv[0]);")
	}
	l.add(2, "return $service->%s($argv);", camel(m.methods[0]))
	l.add(1, "}")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "interface Listener%02d", m.idx)
	l.add(0, "{")
	l.add(1, "public function onEvent($v);")
	l.add(0, "}")
	return l.String()
}

// rubyPath, rubyPackageFiles and rubyEntries write the modules of a gem: one module per package under
// lib/bench, a require_relative per dependency and a script under exe.
func (p *synthProject) rubyPath(m *synthModule) string {
	return fmt.Sprintf("lib/bench/%s/%s.rb", p.pkgName(m.pkg), p.modName(m))
}

// rubyPackageFiles writes the barrel file and the test file of a package.
func (p *synthProject) rubyPackageFiles(pkg []*synthModule, files map[string]string) {
	first, last := pkg[0], pkg[len(pkg)-1]
	var barrel lines
	for _, m := range pkg {
		barrel.add(0, "require_relative '%s/%s'", p.pkgName(m.pkg), p.modName(m))
	}
	barrel.add(0, "")
	barrel.add(0, "module Bench")
	barrel.add(1, "module %s", title(p.pkgName(first.pkg)))
	barrel.add(2, "def self.run(argv)")
	barrel.add(3, "%s.run%02d(argv)", javaClassName(first), first.idx)
	barrel.add(2, "end")
	barrel.add(1, "end")
	barrel.add(0, "end")
	files[fmt.Sprintf("lib/bench/%s.rb", p.pkgName(first.pkg))] = barrel.String()

	var test lines
	test.add(0, "require 'minitest/autorun'")
	test.add(0, "require_relative '../lib/bench/%s'", p.pkgName(last.pkg))
	test.add(0, "")
	test.add(0, "class %sTest < Minitest::Test", javaClassName(last))
	test.add(1, "def test_runs")
	test.add(2, "Bench::%s::%s.run%02d(['input.yaml'])", title(p.pkgName(last.pkg)), javaClassName(last), last.idx)
	test.add(2, "Bench::%s::%s.%s(1)", title(p.pkgName(last.pkg)), javaClassName(last), snake(last.helpers[0]))
	test.add(1, "end")
	test.add(0, "end")
	files[fmt.Sprintf("test/%s_test.rb", p.pkgName(last.pkg))] = test.String()
}

// rubyEntries writes the entry files and the manifests of the project.
func (p *synthProject) rubyEntries(ext []externalPackage, files map[string]string) {

	var main lines
	for _, pkg := range p.packages {
		main.add(0, "require_relative '../lib/bench/%s'", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	for _, pkg := range p.packages {
		main.add(0, "Bench::%s.run(ARGV)", title(p.pkgName(pkg[0].pkg)))
	}
	files["exe/bench.rb"] = main.String()
}

func (p *synthProject) rubyModule(m *synthModule, ext []externalPackage) string {
	var l lines
	l.add(0, "require 'json'")
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(0, "require_relative '../%s/%s'", p.pkgName(dep.pkg), p.modName(dep))
	}
	l.add(0, "")
	qualified := func(dep *synthModule) string {
		if dep.pkg == m.pkg {
			return javaClassName(dep)
		}
		return fmt.Sprintf("Bench::%s::%s", title(p.pkgName(dep.pkg)), javaClassName(dep))
	}
	call := func(e externalCall) string {
		name, _ := externalRef(ext[e.pkg], "::")
		return name + "." + ext[e.pkg].funcs[e.fn]
	}
	helper := func(m *synthModule, i int) string { return fmt.Sprintf("%s%02d", snake(m.helpers[i]), m.idx) }

	l.add(0, "module Bench")
	l.add(1, "module %s", title(p.pkgName(m.pkg)))
	l.add(2, "class %s", javaClassName(m))
	l.add(3, "def initialize(config)")
	l.add(4, "@config = config")
	l.add(4, "@items = []")
	if len(m.deps) > 0 {
		l.add(4, "@store = %s.new(config)", qualified(p.modules[m.deps[0]]))
	}
	l.add(3, "end")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(3, "def %s", snake(w))
			l.add(4, "@items.size")
			l.add(3, "end")
			continue
		}
		l.add(3, "def %s(value)", snake(w))
		l.add(4, "data = self.class.%s(value)", helper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(4, "data = %s(data)", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(4, "data = @store.%s(data)", snake(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(4, "data = public_send(@config, data)")
		}
		if i+1 < len(m.methods)-1 {
			l.add(4, "data = %s(data)", snake(m.methods[i+1]))
		}
		l.add(4, "@items.each { |item| item.on_event(data) }")
		l.add(4, "data")
		l.add(3, "end")
	}
	l.add(0, "")
	for i := range m.helpers {
		l.add(3, "def self.%s(value)", helper(m, i))
		l.add(4, "raise ArgumentError, '%s needs a value' if value.nil?", helper(m, i))
		switch {
		case i+1 < len(m.helpers):
			l.add(4, "value = %s(value)", helper(m, i+1))
		case m.sibling >= 0:
			sib := p.modules[m.sibling]
			l.add(4, "value = %s.%s(value)", qualified(sib), helper(sib, 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(4, "value = %s(value)", call(m.externals[i]))
		}
		if i < len(m.deps) {
			dep := p.modules[m.deps[i]]
			l.add(4, "value = %s.%s(value)", qualified(dep), helper(dep, 0))
		}
		l.add(4, "value.to_s")
		l.add(3, "end")
		l.add(0, "")
	}
	l.add(3, "def self.run%02d(argv)", m.idx)
	l.add(4, "service = new('%s')", p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(4, "%s.new(nil).%s(argv)", qualified(sib), snake(sib.methods[0]))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(4, "%s.run%02d(argv)", qualified(dep), dep.idx)
	}
	switch m.dynamic {
	case dynamicImport:
		l.add(4, "require \"bench/plugins/#{argv[0]}\"")
	case dynamicEval:
		l.add(4, "eval(argv[0])")
	}
	l.add(4, "service.%s(argv)", snake(m.methods[0]))
	l.add(3, "end")
	l.add(2, "end")
	l.add(1, "end")
	l.add(0, "end")
	return l.String()
}

// rustPath, rustPackageFiles and rustEntries write the modules of a binary crate: one module directory per
// package, whose mod.rs declares its modules, and a test file per package.
func (p *synthProject) rustPath(m *synthModule) string {
	return fmt.Sprintf("src/%s/%s.rs", p.pkgName(m.pkg), p.modName(m))
}

// rustPackageFiles writes the barrel file and the test file of a package.
func (p *synthProject) rustPackageFiles(pkg []*synthModule, files map[string]string) {
	var barrel lines
	for _, m := range pkg {
		barrel.add(0, "pub mod %s;", p.modName(m))
	}
	first, last := pkg[0], pkg[len(pkg)-1]
	barrel.add(0, "")
	barrel.add(0, "pub fn run(argv: &[String]) -> String {")
	barrel.add(1, "%s::run%02d(argv)", p.modName(first), first.idx)
	barrel.add(0, "}")
	files[fmt.Sprintf("src/%s/mod.rs", p.pkgName(first.pkg))] = barrel.String()

	var test lines
	test.add(0, "use bench::%s::%s;", p.pkgName(last.pkg), p.modName(last))
	test.add(0, "")
	test.add(0, "#[test]")
	test.add(0, "fn runs() {")
	test.add(1, "%s::run%02d(&[String::from(\"input.yaml\")]);", p.modName(last), last.idx)
	test.add(1, "%s::%s%02d(\"1\");", p.modName(last), snake(last.helpers[0]), last.idx)
	test.add(0, "}")
	files[fmt.Sprintf("tests/%s.rs", p.pkgName(last.pkg))] = test.String()
}

// rustEntries writes the entry files and the manifests of the project.
func (p *synthProject) rustEntries(ext []externalPackage, files map[string]string) {

	var main lines
	for _, pkg := range p.packages {
		main.add(0, "mod %s;", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "")
	main.add(0, "fn main() {")
	main.add(1, "let args: Vec<String> = std::env::args().collect();")
	for _, pkg := range p.packages {
		main.add(1, "%s::run(&args);", p.pkgName(pkg[0].pkg))
	}
	main.add(0, "}")
	files["src/main.rs"] = main.String()
	files["Cargo.toml"] = "[package]\nname = \"bench\"\nversion = \"1.0.0\"\nedition = \"2021\"\n"
}

func (p *synthProject) rustModule(m *synthModule, ext []externalPackage) string {
	var l lines
	path := func(dep *synthModule) string {
		return fmt.Sprintf("crate::%s::%s", p.pkgName(dep.pkg), p.modName(dep))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(0, "use %s::%s;", path(dep), javaClassName(dep))
	}
	l.add(0, "")
	call := func(e externalCall) string {
		name, _ := externalRef(ext[e.pkg], "::")
		return name + "::" + ext[e.pkg].funcs[e.fn]
	}
	helper := func(m *synthModule, i int) string { return fmt.Sprintf("%s%02d", snake(m.helpers[i]), m.idx) }

	class := javaClassName(m)
	l.add(0, "pub trait Listener%02d {", m.idx)
	l.add(1, "fn on_event(&self, v: &str);")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "pub struct %s {", class)
	if len(m.deps) > 0 {
		l.add(1, "store: %s,", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(1, "config: String,")
	l.add(1, "items: Vec<Box<dyn Listener%02d>>,", m.idx)
	l.add(1, "handlers: Vec<fn(&str) -> String>,")
	l.add(0, "}")
	l.add(0, "")
	l.add(0, "impl %s {", class)
	l.add(1, "pub fn new(config: &str) -> Self {")
	l.add(2, "%s {", class)
	if len(m.deps) > 0 {
		l.add(3, "store: %s::new(config),", javaClassName(p.modules[m.deps[0]]))
	}
	l.add(3, "config: config.to_string(),")
	l.add(3, "items: Vec::new(),")
	l.add(3, "handlers: vec![%s],", helper(m, 0))
	l.add(2, "}")
	l.add(1, "}")
	for i, w := range m.methods {
		l.add(0, "")
		if i == len(m.methods)-1 {
			l.add(1, "pub fn %s(&self) -> usize {", snake(w))
			l.add(2, "self.items.len()")
			l.add(1, "}")
			continue
		}
		l.add(1, "pub fn %s(&self, value: &str) -> String {", snake(w))
		l.add(2, "let mut data = %s(value);", helper(m, i%len(m.helpers)))
		if i == 0 {
			l.add(2, "data = format!(\"{:?}\", %s(&data));", call(m.externals[0]))
		}
		if i == 1 && len(m.deps) > 0 {
			l.add(2, "data = self.store.%s(&data);", snake(p.modules[m.deps[0]].methods[0]))
		}
		if i == 1 && m.dynamic == dynamicAttribute {
			l.add(2, "data = (self.handlers[self.config.len() %% 1])(&data);")
		}
		if i+1 < len(m.methods)-1 {
			l.add(2, "data = self.%s(&data);", snake(m.methods[i+1]))
		}
		l.add(2, "for item in &self.items {")
		l.add(3, "item.on_event(&data);")
		l.add(2, "}")
		l.add(2, "data")
		l.add(1, "}")
	}
	l.add(0, "}")
	l.add(0, "")
	for i := range m.helpers {
		l.add(0, "pub fn %s(value: &str) -> String {", helper(m, i))
		l.add(1, "assert!(!value.is_empty(), \"%s needs a value\");", helper(m, i))
		l.add(1, "let mut value = value.to_string();")
		switch {
		case i+1 < len(m.helpers):
			l.add(1, "value = %s(&value);", helper(m, i+1))
		case m.sibling >= 0:
			sib := p.modules[m.sibling]
			l.add(1, "value = super::%s::%s(&value);", p.modName(sib), helper(sib, 0))
		}
		if i < len(m.externals) && i > 0 {
			l.add(1, "value = format!(\"{:?}\", %s(&value));", call(m.externals[i]))
		}
		if i < len(m.deps) {
			dep := p.modules[m.deps[i]]
			l.add(1, "value = %s::%s(&value);", path(dep), helper(dep, 0))
		}
		l.add(1, "value")
		l.add(0, "}")
		l.add(0, "")
	}
	l.add(0, "pub fn run%02d(argv: &[String]) -> String {", m.idx)
	l.add(1, "let service = %s::new(\"%s\");", class, p.modName(m))
	if m.sibling >= 0 {
		sib := p.modules[m.sibling]
		l.add(1, "super::%s::%s::new(\"\").%s(&argv[0]);", p.modName(sib), javaClassName(sib), snake(sib.methods[0]))
	}
	for _, d := range m.deps {
		dep := p.modules[d]
		l.add(1, "%s::run%02d(argv);", path(dep), dep.idx)
	}
	l.add(1, "service.%s(&argv[0])", snake(m.methods[0]))
	l.add(0, "}")
	return l.String()
}
