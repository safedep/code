package langcode

// ecmascriptBuiltins are the globals of JavaScript and of the runtime of
// Node and the browser, which TypeScript shares.
var ecmascriptBuiltins = []string{
	"console", "parseInt", "parseFloat", "isNaN", "isFinite", "setTimeout", "setInterval",
	"clearTimeout", "clearInterval", "eval", "encodeURI", "decodeURI", "encodeURIComponent",
	"decodeURIComponent", "Number", "String", "Boolean", "Array", "Object", "Function", "Date",
	"Math", "RegExp", "JSON", "Promise", "Symbol", "BigInt", "Map", "Set", "WeakMap", "WeakSet",
	"Reflect", "Proxy", "Intl", "undefined", "NaN", "Infinity", "process", "global",
	"globalThis", "window", "document", "navigator", "Buffer", "module", "exports", "require",
	"URL", "fetch", "Error", "TypeError", "AbortController", "TextEncoder", "TextDecoder",
	"crypto",
}
