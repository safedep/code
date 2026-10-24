#!/usr/bin/env python3
"""Write the PyPI advisories of the method measure into advisories.json.

OSV has no symbol list for a PyPI advisory, unlike the Go vulndb. So this
script holds the symbols by hand, as the text of each advisory names
them, and reads the id and the version ranges from OSV. Run it to refresh
the ranges. Add a row to SYMBOLS when the corpus gains an advisory.

    python3 scripts/pypi_advisories.py

A version in a range is padded to three parts, so the Go test compares it
as semver. A pre-release, as 2.0.0a1, becomes 2.0.0-a1.
"""

import json
import re
import sys
import urllib.request
from pathlib import Path

OSV = "https://api.osv.dev/v1/vulns/"
OUT = Path(__file__).resolve().parent.parent / "test/projectgraph/public/methods/advisories.json"

# SYMBOLS maps an OSV id to the module and the symbols that the advisory
# names, as the Go vulndb lists them: a module path and a symbol, with a
# dot between a class and its method. The source of each row is
# safedep:hand, from the advisory text.
SYMBOLS = {
    # PyYAML: code execution through full_load and FullLoader before 5.4.
    "PYSEC-2021-142": {"yaml": ["load", "load_all", "full_load", "full_load_all"]},
    # Jinja2: attribute injection through the xmlattr filter, and the
    # sandbox breakouts through format.
    "GHSA-h5c8-rqwp-cp95": {"jinja2.filters": ["do_xmlattr"]},
    "GHSA-h75v-3vvj-5mfj": {"jinja2.filters": ["do_xmlattr"]},
    "GHSA-q2x7-8rv6-6q7h": {"jinja2.sandbox": ["SandboxedEnvironment.call", "SandboxedEnvironment.format_string"]},
    "GHSA-cpwx-vrp4-4pq7": {"jinja2.sandbox": ["SandboxedEnvironment.call", "SandboxedEnvironment.is_safe_attribute"], "jinja2.filters": ["do_attr"]},
    # Tornado: the open redirect of StaticFileHandler, and the cookie and
    # multipart parsers.
    "PYSEC-2023-75": {"tornado.web": ["StaticFileHandler.get", "StaticFileHandler.validate_absolute_path"]},
    "GHSA-8w49-h785-mj3c": {"tornado.httputil": ["parse_cookie"]},
    "GHSA-7cx3-6m66-7c5m": {"tornado.httputil": ["parse_multipart_form_data", "parse_body_arguments"]},
    "GHSA-qjxf-f2mg-c6mc": {"tornado.httputil": ["parse_multipart_form_data"]},
    "GHSA-pr2v-jx2c-wg9f": {"tornado.web": ["RequestHandler.set_status"]},
    "GHSA-fqwm-6jpj-5wxc": {"tornado.web": ["RequestHandler.set_cookie"]},
    "GHSA-c2m8-h5v5-343r": {"tornado.web": ["StaticFileHandler.get", "StaticFileHandler.validate_absolute_path"]},
    # requests: the Proxy-Authorization leak on a redirect, the lost
    # verify=False, and the .netrc credentials.
    "PYSEC-2023-74": {"requests.sessions": ["Session.rebuild_proxies", "Session.send"], "requests.adapters": ["HTTPAdapter.send"]},
    "GHSA-9wx4-h78v-vm56": {"requests.adapters": ["HTTPAdapter.send", "HTTPAdapter.get_connection"], "requests.sessions": ["Session.request"]},
    "GHSA-9hjg-9r4m-mvj7": {"requests.utils": ["get_netrc_auth"], "requests.sessions": ["Session.prepare_request"]},
    # urllib3: the headers and the body that a redirect keeps.
    "PYSEC-2023-192": {"urllib3.poolmanager": ["PoolManager.urlopen"], "urllib3.connectionpool": ["HTTPConnectionPool.urlopen"]},
    "PYSEC-2023-212": {"urllib3.poolmanager": ["PoolManager.urlopen"], "urllib3.connectionpool": ["HTTPConnectionPool.urlopen"]},
    "GHSA-34jh-p97f-mpxf": {"urllib3.poolmanager": ["PoolManager.urlopen"], "urllib3.connectionpool": ["HTTPConnectionPool.urlopen"]},
    # idna: quadratic time in encode.
    "PYSEC-2024-60": {"idna": ["encode"], "idna.core": ["encode"]},
    # Flask: the session cookie in a cached response.
    "PYSEC-2023-62": {"flask.sessions": ["SecureCookieSessionInterface.save_session"]},
    # Werkzeug: the nameless cookie, the multipart parser, the debugger
    # and safe_join.
    "PYSEC-2023-57": {"werkzeug.http": ["parse_cookie"], "werkzeug._internal": ["_cookie_parse_impl"]},
    "PYSEC-2023-58": {"werkzeug.formparser": ["MultiPartParser.parse", "FormDataParser.parse"]},
    "PYSEC-2023-221": {"werkzeug.formparser": ["MultiPartParser.parse"], "werkzeug.sansio.multipart": ["MultipartDecoder.next_event"]},
    "GHSA-2g68-c3qc-8985": {"werkzeug.debug": ["DebuggedApplication.__call__"]},
    "GHSA-f9vj-2wh5-fj8j": {"werkzeug.security": ["safe_join"]},
    "GHSA-q34m-jh98-gwm2": {"werkzeug.formparser": ["MultiPartParser.parse"], "werkzeug.sansio.multipart": ["MultipartDecoder.next_event"]},
    "PYSEC-2026-4312": {"werkzeug.security": ["safe_join"]},
    # paramiko: the race in write_private_key_file.
    "PYSEC-2022-166": {"paramiko.pkey": ["PKey.write_private_key_file", "PKey._write_private_key_file"]},
    # Pillow: the open of an image and the pure Python callers of the
    # decoders that the advisories name.
    "PYSEC-2022-42980": {"PIL.Image": ["open"], "PIL.TiffImagePlugin": ["TiffImageFile._setup"]},
    "PYSEC-2023-227": {"PIL.ImageFont": ["FreeTypeFont.getmask2"], "PIL.ImageDraw": ["ImageDraw.text"]},
    "PYSEC-2026-457": {"PIL.ImageMath": ["eval"]},
    "GHSA-74r2-7ff9-mjj2": {"PIL.Image": ["open"], "PIL.GifImagePlugin": ["GifImageFile._open"]},
    "GHSA-xgh8-jp6q-p5x8": {"PIL.Image": ["open"], "PIL.XbmImagePlugin": ["XbmImageFile._open"]},
    "PYSEC-2026-2257": {"PIL.ImageShow": ["WindowsViewer.get_command"]},
    "PYSEC-2026-3453": {"PIL.ImageCms": ["ImageCmsTransform.apply"]},
    # cryptography: the PKCS7 and PKCS12 loaders and the SSH certificates.
    "PYSEC-2023-254": {"cryptography.hazmat.primitives.serialization.pkcs7": ["load_pem_pkcs7_certificates", "load_der_pkcs7_certificates"]},
    "PYSEC-2024-225": {"cryptography.hazmat.primitives.serialization.pkcs12": ["serialize_key_and_certificates"]},
    "PYSEC-2023-112": {"cryptography.hazmat.primitives.serialization.ssh": ["load_ssh_public_identity", "SSHCertificateBuilder.sign"]},
    # lxml: iterwalk and canonicalize, and iterparse. The code is compiled,
    # so the link step has no source for it.
    "PYSEC-2022-230": {"lxml.etree": ["iterwalk", "canonicalize"]},
    "GHSA-vfmq-68hx-4jfw": {"lxml.etree": ["iterparse", "ETCompatXMLParser"]},
    # Django: the validators, the text utilities, the template filters and
    # the query set methods that the advisories name.
    "PYSEC-2023-100": {"django.core.validators": ["EmailValidator.__call__", "URLValidator.__call__"]},
    "PYSEC-2023-222": {"django.contrib.auth.forms": ["UsernameField.to_python"]},
    "PYSEC-2023-225": {"django.utils.encoding": ["uri_to_iri"]},
    "PYSEC-2023-226": {"django.utils.text": ["Truncator.chars", "Truncator.words"]},
    "PYSEC-2023-61": {"django.forms.fields": ["FileField.clean"], "django.forms.widgets": ["ClearableFileInput.value_from_datadict"]},
    "PYSEC-2024-28": {"django.contrib.humanize.templatetags.humanize": ["intcomma"]},
    "PYSEC-2024-47": {"django.utils.text": ["Truncator.words"]},
    "PYSEC-2024-56": {"django.utils.html": ["urlize"], "django.template.defaultfilters": ["urlize", "urlizetrunc"]},
    "PYSEC-2024-57": {"django.contrib.auth.backends": ["ModelBackend.authenticate"]},
    "PYSEC-2024-58": {"django.core.files.storage.base": ["Storage.save"], "django.db.models.fields.files": ["FieldFile.save"]},
    "PYSEC-2024-59": {"django.utils.translation": ["get_supported_language_variant"]},
    "PYSEC-2024-67": {"django.template.defaultfilters": ["floatformat"]},
    "PYSEC-2024-68": {"django.utils.html": ["urlize", "Urlizer.__call__"]},
    "PYSEC-2024-69": {"django.utils.html": ["urlize", "Urlizer.__call__"]},
    "PYSEC-2024-70": {"django.db.models.query": ["QuerySet.values", "QuerySet.values_list"]},
    "PYSEC-2024-102": {"django.utils.html": ["urlize", "Urlizer.__call__"]},
    "PYSEC-2024-156": {"django.utils.html": ["strip_tags"]},
    "PYSEC-2024-157": {"django.db.models.fields.json": ["HasKey.as_oracle"]},
    "PYSEC-2025-1": {"django.core.validators": ["validate_ipv6_address"], "django.utils.ipv6": ["is_valid_ipv6_address"]},
    "PYSEC-2025-13": {"django.utils.text": ["wrap"], "django.template.defaultfilters": ["wordwrap"]},
    "PYSEC-2025-37": {"django.utils.html": ["strip_tags"]},
    "PYSEC-2025-47": {"django.utils.log": ["log_response"]},
    "PYSEC-2025-106": {"django.db.models.query": ["QuerySet.annotate", "QuerySet.alias"]},
    "PYSEC-2025-108": {"django.db.models.query": ["QuerySet.filter", "QuerySet.exclude", "QuerySet.get"]},
    "PYSEC-2026-45": {"django.utils.text": ["Truncator.chars", "Truncator.words"]},
    "PYSEC-2026-47": {"django.db.models.query": ["QuerySet.order_by"]},
    "PYSEC-2026-48": {"django.http.multipartparser": ["MultiPartParser.parse"]},
    # sqlparse: the lexer and the grouping of nested lists.
    "PYSEC-2023-87": {"sqlparse.lexer": ["Lexer.get_tokens"], "sqlparse": ["parse", "format"]},
    "GHSA-2m57-hf25-phgg": {"sqlparse": ["parse"], "sqlparse.engine.grouping": ["group"]},
    "GHSA-27jp-wm6q-gp25": {"sqlparse": ["format"]},
    "GHSA-cfqr-cjx5-5jcm": {"sqlparse": ["format"]},
    "GHSA-f2ff-p2ww-7p4p": {"sqlparse": ["parse"], "sqlparse.engine.grouping": ["group_comments"]},
}

# PACKAGES are the PyPI names of the dependencies of corpus.txt, in the
# case that OSV uses as the key of advisories.json.
PACKAGES = [
    "pyyaml", "jinja2", "tornado", "requests", "urllib3", "idna", "flask", "werkzeug", "paramiko", "pillow",
    "cryptography", "lxml", "django", "sqlparse",
]

PRERELEASE = re.compile(r"^(\d+(?:\.\d+)*)(?:\.?(a|b|rc|post|dev)(\d*))?$")


def semver(version):
    """Pad a PyPI version to three parts for the semver compare of the Go test."""
    m = PRERELEASE.match(version)
    if not m:
        return version
    parts = m.group(1).split(".")
    while len(parts) < 3:
        parts.append("0")
    out = ".".join(parts[:3])
    if m.group(2):
        out += "-" + m.group(2) + (m.group(3) or "0")
    return out


def fetch(vuln_id):
    with urllib.request.urlopen(OSV + vuln_id, timeout=60) as r:
        return json.load(r)


def ranges_of(vuln, package):
    out = []
    for a in vuln.get("affected", []):
        pkg = a.get("package", {})
        if pkg.get("ecosystem") != "PyPI" or pkg.get("name", "").lower() != package:
            continue
        for r in a.get("ranges", []):
            if r.get("type") != "ECOSYSTEM":
                continue
            for e in r.get("events", []):
                if "introduced" in e:
                    out.append({"introduced": semver(e["introduced"])})
                if "fixed" in e:
                    out.append({"fixed": semver(e["fixed"])})
    return out


def main():
    data = json.loads(OUT.read_text())
    pypi = {}
    for vuln_id, modules in sorted(SYMBOLS.items()):
        vuln = fetch(vuln_id)
        names = {a["package"]["name"].lower() for a in vuln.get("affected", []) if a.get("package", {}).get("ecosystem") == "PyPI"}
        matched = [p for p in PACKAGES if p in names]
        if not matched:
            sys.exit(f"{vuln_id} affects none of PACKAGES: {sorted(names)}")
        for package in matched:
            pypi.setdefault(package, []).append({
                "id": vuln_id,
                "source": "safedep:hand",
                "ranges": ranges_of(vuln, package),
                "imports": [{"path": path, "symbols": symbols} for path, symbols in sorted(modules.items())],
            })
    for rows in pypi.values():
        rows.sort(key=lambda a: a["id"])
    data["pypi"] = dict(sorted(pypi.items()))
    OUT.write_text(json.dumps(data, indent=1, sort_keys=True) + "\n")
    print(f"{sum(len(v) for v in pypi.values())} advisories of {len(pypi)} packages")


if __name__ == "__main__":
    main()
