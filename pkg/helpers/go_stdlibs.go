// Code generated by scripts/generate_go_stdlibs.sh; DO NOT EDIT.
package helpers

// GoStdLibPkgs is a map of all Go standard library packages
var GoStdLibs = map[string]bool{
    "archive/tar": true,
    "archive/zip": true,
    "bufio": true,
    "bytes": true,
    "cmp": true,
    "compress/bzip2": true,
    "compress/flate": true,
    "compress/gzip": true,
    "compress/lzw": true,
    "compress/zlib": true,
    "container/heap": true,
    "container/list": true,
    "container/ring": true,
    "context": true,
    "crypto": true,
    "crypto/aes": true,
    "crypto/cipher": true,
    "crypto/des": true,
    "crypto/dsa": true,
    "crypto/ecdh": true,
    "crypto/ecdsa": true,
    "crypto/ed25519": true,
    "crypto/elliptic": true,
    "crypto/hmac": true,
    "crypto/internal/alias": true,
    "crypto/internal/bigmod": true,
    "crypto/internal/boring": true,
    "crypto/internal/boring/bbig": true,
    "crypto/internal/boring/bcache": true,
    "crypto/internal/boring/sig": true,
    "crypto/internal/cryptotest": true,
    "crypto/internal/edwards25519": true,
    "crypto/internal/edwards25519/field": true,
    "crypto/internal/hpke": true,
    "crypto/internal/mlkem768": true,
    "crypto/internal/nistec": true,
    "crypto/internal/nistec/fiat": true,
    "crypto/internal/randutil": true,
    "crypto/md5": true,
    "crypto/rand": true,
    "crypto/rc4": true,
    "crypto/rsa": true,
    "crypto/sha1": true,
    "crypto/sha256": true,
    "crypto/sha512": true,
    "crypto/subtle": true,
    "crypto/tls": true,
    "crypto/x509": true,
    "crypto/x509/internal/macos": true,
    "crypto/x509/pkix": true,
    "database/sql": true,
    "database/sql/driver": true,
    "debug/buildinfo": true,
    "debug/dwarf": true,
    "debug/elf": true,
    "debug/gosym": true,
    "debug/macho": true,
    "debug/pe": true,
    "debug/plan9obj": true,
    "embed": true,
    "embed/internal/embedtest": true,
    "encoding": true,
    "encoding/ascii85": true,
    "encoding/asn1": true,
    "encoding/base32": true,
    "encoding/base64": true,
    "encoding/binary": true,
    "encoding/csv": true,
    "encoding/gob": true,
    "encoding/hex": true,
    "encoding/json": true,
    "encoding/pem": true,
    "encoding/xml": true,
    "errors": true,
    "expvar": true,
    "flag": true,
    "fmt": true,
    "go/ast": true,
    "go/build": true,
    "go/build/constraint": true,
    "go/constant": true,
    "go/doc": true,
    "go/doc/comment": true,
    "go/format": true,
    "go/importer": true,
    "go/internal/gccgoimporter": true,
    "go/internal/gcimporter": true,
    "go/internal/srcimporter": true,
    "go/internal/typeparams": true,
    "go/parser": true,
    "go/printer": true,
    "go/scanner": true,
    "go/token": true,
    "go/types": true,
    "go/version": true,
    "hash": true,
    "hash/adler32": true,
    "hash/crc32": true,
    "hash/crc64": true,
    "hash/fnv": true,
    "hash/maphash": true,
    "html": true,
    "html/template": true,
    "image": true,
    "image/color": true,
    "image/color/palette": true,
    "image/draw": true,
    "image/gif": true,
    "image/internal/imageutil": true,
    "image/jpeg": true,
    "image/png": true,
    "index/suffixarray": true,
    "internal/abi": true,
    "internal/asan": true,
    "internal/bisect": true,
    "internal/buildcfg": true,
    "internal/bytealg": true,
    "internal/byteorder": true,
    "internal/cfg": true,
    "internal/chacha8rand": true,
    "internal/concurrent": true,
    "internal/coverage": true,
    "internal/coverage/calloc": true,
    "internal/coverage/cfile": true,
    "internal/coverage/cformat": true,
    "internal/coverage/cmerge": true,
    "internal/coverage/decodecounter": true,
    "internal/coverage/decodemeta": true,
    "internal/coverage/encodecounter": true,
    "internal/coverage/encodemeta": true,
    "internal/coverage/pods": true,
    "internal/coverage/rtcov": true,
    "internal/coverage/slicereader": true,
    "internal/coverage/slicewriter": true,
    "internal/coverage/stringtab": true,
    "internal/coverage/test": true,
    "internal/coverage/uleb128": true,
    "internal/cpu": true,
    "internal/dag": true,
    "internal/diff": true,
    "internal/filepathlite": true,
    "internal/fmtsort": true,
    "internal/fuzz": true,
    "internal/goarch": true,
    "internal/godebug": true,
    "internal/godebugs": true,
    "internal/goexperiment": true,
    "internal/goos": true,
    "internal/goroot": true,
    "internal/gover": true,
    "internal/goversion": true,
    "internal/itoa": true,
    "internal/lazyregexp": true,
    "internal/lazytemplate": true,
    "internal/msan": true,
    "internal/nettrace": true,
    "internal/obscuretestdata": true,
    "internal/oserror": true,
    "internal/pkgbits": true,
    "internal/platform": true,
    "internal/poll": true,
    "internal/profile": true,
    "internal/profilerecord": true,
    "internal/race": true,
    "internal/reflectlite": true,
    "internal/runtime/atomic": true,
    "internal/runtime/exithook": true,
    "internal/saferio": true,
    "internal/singleflight": true,
    "internal/stringslite": true,
    "internal/syscall/execenv": true,
    "internal/syscall/unix": true,
    "internal/sysinfo": true,
    "internal/testenv": true,
    "internal/testlog": true,
    "internal/testpty": true,
    "internal/trace": true,
    "internal/trace/event": true,
    "internal/trace/event/go122": true,
    "internal/trace/internal/oldtrace": true,
    "internal/trace/internal/testgen/go122": true,
    "internal/trace/raw": true,
    "internal/trace/testtrace": true,
    "internal/trace/traceviewer": true,
    "internal/trace/traceviewer/format": true,
    "internal/trace/version": true,
    "internal/txtar": true,
    "internal/types/errors": true,
    "internal/unsafeheader": true,
    "internal/weak": true,
    "internal/xcoff": true,
    "internal/zstd": true,
    "io": true,
    "io/fs": true,
    "io/ioutil": true,
    "iter": true,
    "log": true,
    "log/internal": true,
    "log/slog": true,
    "log/slog/internal": true,
    "log/slog/internal/benchmarks": true,
    "log/slog/internal/buffer": true,
    "log/slog/internal/slogtest": true,
    "log/syslog": true,
    "maps": true,
    "math": true,
    "math/big": true,
    "math/bits": true,
    "math/cmplx": true,
    "math/rand": true,
    "math/rand/v2": true,
    "mime": true,
    "mime/multipart": true,
    "mime/quotedprintable": true,
    "net": true,
    "net/http": true,
    "net/http/cgi": true,
    "net/http/cookiejar": true,
    "net/http/fcgi": true,
    "net/http/httptest": true,
    "net/http/httptrace": true,
    "net/http/httputil": true,
    "net/http/internal": true,
    "net/http/internal/ascii": true,
    "net/http/internal/testcert": true,
    "net/http/pprof": true,
    "net/internal/cgotest": true,
    "net/internal/socktest": true,
    "net/mail": true,
    "net/netip": true,
    "net/rpc": true,
    "net/rpc/jsonrpc": true,
    "net/smtp": true,
    "net/textproto": true,
    "net/url": true,
    "os": true,
    "os/exec": true,
    "os/exec/internal/fdtest": true,
    "os/signal": true,
    "os/user": true,
    "path": true,
    "path/filepath": true,
    "plugin": true,
    "reflect": true,
    "reflect/internal/example1": true,
    "reflect/internal/example2": true,
    "regexp": true,
    "regexp/syntax": true,
    "runtime": true,
    "runtime/cgo": true,
    "runtime/coverage": true,
    "runtime/debug": true,
    "runtime/internal/math": true,
    "runtime/internal/sys": true,
    "runtime/internal/wasitest": true,
    "runtime/metrics": true,
    "runtime/pprof": true,
    "runtime/race": true,
    "runtime/trace": true,
    "slices": true,
    "sort": true,
    "strconv": true,
    "strings": true,
    "structs": true,
    "sync": true,
    "sync/atomic": true,
    "syscall": true,
    "testing": true,
    "testing/fstest": true,
    "testing/internal/testdeps": true,
    "testing/iotest": true,
    "testing/quick": true,
    "testing/slogtest": true,
    "text/scanner": true,
    "text/tabwriter": true,
    "text/template": true,
    "text/template/parse": true,
    "time": true,
    "time/tzdata": true,
    "unicode": true,
    "unicode/utf16": true,
    "unicode/utf8": true,
    "unique": true,
    "unsafe": true,
    "vendor/golang.org/x/crypto/chacha20": true,
    "vendor/golang.org/x/crypto/chacha20poly1305": true,
    "vendor/golang.org/x/crypto/cryptobyte": true,
    "vendor/golang.org/x/crypto/cryptobyte/asn1": true,
    "vendor/golang.org/x/crypto/hkdf": true,
    "vendor/golang.org/x/crypto/internal/alias": true,
    "vendor/golang.org/x/crypto/internal/poly1305": true,
    "vendor/golang.org/x/crypto/sha3": true,
    "vendor/golang.org/x/net/dns/dnsmessage": true,
    "vendor/golang.org/x/net/http/httpguts": true,
    "vendor/golang.org/x/net/http/httpproxy": true,
    "vendor/golang.org/x/net/http2/hpack": true,
    "vendor/golang.org/x/net/idna": true,
    "vendor/golang.org/x/net/nettest": true,
    "vendor/golang.org/x/net/route": true,
    "vendor/golang.org/x/sys/cpu": true,
    "vendor/golang.org/x/text/secure/bidirule": true,
    "vendor/golang.org/x/text/transform": true,
    "vendor/golang.org/x/text/unicode/bidi": true,
    "vendor/golang.org/x/text/unicode/norm": true,
}
