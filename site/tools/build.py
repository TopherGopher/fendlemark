#!/usr/bin/env python3
"""Stamp src/pages/**/*.html fragments into the shared layout.

No dependencies. Run from the repo root:  python3 tools/build.py
Output HTML is committed so GitHub Pages serves it as-is.
"""
import json, os, re, sys, html
from html.parser import HTMLParser

DEFAULT_DATE = "2026-10-24"  # bump when content changes; keeps builds reproducible

SITE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))   # site/
ROOT = os.path.join(SITE, "public")                                    # generated output
SRC = os.path.join(SITE, "src", "pages")
BASE = "https://fendlemark.develops.guru"
LAYOUT = open(os.path.join(SITE, "src", "layout.html"), encoding="utf-8").read()

# Sidebar navigation. (label, path). Paths are site-absolute with trailing slash.
NAV = [
    ("Documentation", [
        ("Overview", "/docs/"),
        ("Getting started", "/docs/getting-started/"),
        ("Architecture", "/docs/architecture/"),
        ("Configuration", "/docs/configuration/"),
        ("Adapters", "/docs/adapters/"),
        ("Writing an adapter", "/docs/adapters/writing-an-adapter/"),
        ("The console (UI)", "/docs/ui/"),
        ("Authentication", "/docs/auth/"),
        ("CLI reference", "/docs/cli/"),
        ("Go API reference", "/docs/api/"),
        ("Migrating 0.8 to 0.9", "/docs/migration/"),
    ]),
    ("Project", [
        ("Philosophy", "/philosophy/"),
        ("Governance", "/governance/"),
        ("Contributing (FCP-1)", "/governance/contributing/"),
        ("Changelog", "/changelog/"),
        ("Packages", "/fendlemark/"),
        ("FAQ", "/faq/"),
        ("About", "/about/"),
    ]),
]

META_RE = re.compile(r"^\s*<!--\s*meta\s*(\{.*?\})\s*-->", re.S)

def read_fragment(path):
    text = open(path, encoding="utf-8").read()
    m = META_RE.match(text)
    if not m:
        sys.exit(f"{path}: missing <!-- meta {{...}} --> header")
    meta = json.loads(m.group(1))
    body = text[m.end():].strip()
    return meta, body

def url_for(src_path):
    rel = os.path.relpath(src_path, SRC)
    rel = rel[:-len(".html")]
    if rel == "index":
        return "/"
    if rel.endswith("/index"):
        rel = rel[:-len("/index")]
    return "/" + rel + "/"

def out_for(url):
    if url == "/":
        return os.path.join(ROOT, "index.html")
    return os.path.join(ROOT, url.strip("/"), "index.html")

def sidebar(current):
    parts = []
    for group, items in NAV:
        parts.append(f'<div class="nav-group"><div class="nav-group-title">{html.escape(group)}</div><ul>')
        for label, path in items:
            cls = ' class="active" aria-current="page"' if path == current else ""
            parts.append(f'<li><a href="{path}"{cls}>{html.escape(label)}</a></li>')
        parts.append("</ul></div>")
    return "\n".join(parts)

def flat_nav():
    return [item for _, items in NAV for item in items]

def prev_next(current):
    items = flat_nav()
    paths = [p for _, p in items]
    if current not in paths:
        return ""
    i = paths.index(current)
    out = ['<nav class="pager" aria-label="Previous and next pages">']
    if i > 0:
        out.append(f'<a class="pager-prev" href="{items[i-1][1]}"><span>Previous</span>{html.escape(items[i-1][0])}</a>')
    else:
        out.append('<span></span>')
    if i < len(items) - 1:
        out.append(f'<a class="pager-next" href="{items[i+1][1]}"><span>Next</span>{html.escape(items[i+1][0])}</a>')
    out.append("</nav>")
    return "\n".join(out)

def breadcrumbs(url, title):
    crumbs = [("Fendlemark", "/")]
    if url.startswith("/docs/") and url != "/docs/":
        crumbs.append(("Documentation", "/docs/"))
    if url.startswith("/governance/") and url != "/governance/":
        crumbs.append(("Governance", "/governance/"))
    if url != "/":
        crumbs.append((title, url))
    items = []
    for i, (name, path) in enumerate(crumbs, 1):
        items.append({"@type": "ListItem", "position": i, "name": name, "item": BASE + path})
    return {"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": items}

def jsonld(meta, url, title, description):
    blocks = [breadcrumbs(url, title)]
    kind = meta.get("type", "TechArticle")
    if url == "/":
        blocks.append({
            "@context": "https://schema.org",
            "@type": "SoftwareApplication",
            "name": "Fendlemark",
            "applicationCategory": "DeveloperApplication",
            "operatingSystem": "Linux, macOS",
            "programmingLanguage": ["Go", "JavaScript"],
            "softwareVersion": meta.get("version", "0.9.4"),
            "url": BASE + "/",
            "description": description,
            "license": "https://opensource.org/license/mit",
            "offers": {"@type": "Offer", "price": "0", "priceCurrency": "USD"},
        })
        blocks.append({
            "@context": "https://schema.org",
            "@type": "WebSite",
            "name": "Fendlemark",
            "url": BASE + "/",
            "sameAs": [
                "https://github.com/TopherGopher/fendlemark",
                "https://pkg.go.dev/fendlemark.develops.guru/fendlemark",
                # add https://www.npmjs.com/package/@fendlemark/ui once it is published
            ],
        })
    elif kind == "FAQPage" and meta.get("faq"):
        blocks.append({
            "@context": "https://schema.org",
            "@type": "FAQPage",
            "mainEntity": [
                {"@type": "Question", "name": q, "acceptedAnswer": {"@type": "Answer", "text": a}}
                for q, a in meta["faq"]
            ],
        })
    else:
        blocks.append({
            "@context": "https://schema.org",
            "@type": kind,
            "headline": title,
            "description": description,
            "url": BASE + url,
            "datePublished": meta.get("published", DEFAULT_DATE),
            "dateModified": meta.get("modified", DEFAULT_DATE),
            "author": {"@type": "Organization", "name": "The Fendlemark Keepers"},
            "publisher": {"@type": "Organization", "name": "Fendlemark", "url": BASE + "/"},
            "inLanguage": "en",
        })
    return "\n".join(
        '<script type="application/ld+json">' + json.dumps(b, ensure_ascii=False) + "</script>" for b in blocks
    )

def render(src_path):
    meta, body = read_fragment(src_path)
    url = url_for(src_path)
    title = meta["title"]
    description = meta["description"]
    full_title = "Fendlemark: one console for MLflow, Metaflow, Argo Workflows, and Kubeflow" if url == "/" else f"{title} · Fendlemark"
    layout = meta.get("layout", "docs")
    page = LAYOUT
    repl = {
        "{{title}}": html.escape(full_title),
        "{{description}}": html.escape(description),
        "{{canonical}}": BASE + url,
        "{{markdown}}": url + "index.md",
        "{{body}}": body,
        "{{sidebar}}": sidebar(url),
        "{{pager}}": prev_next(url) if layout == "docs" else "",
        "{{jsonld}}": jsonld(meta, url, title, description),
        "{{layout}}": layout,
        "{{year}}": "2026",
        "{{version}}": "0.9.4",
    }
    for k, v in repl.items():
        page = page.replace(k, v)
    out = out_for(url)
    os.makedirs(os.path.dirname(out), exist_ok=True)
    open(out, "w", encoding="utf-8").write(page)
    return url

def text_of(body):
    t = re.sub(r"<script.*?</script>", "", body, flags=re.S)
    t = re.sub(r"<pre.*?</pre>", lambda m: "\n```\n" + html.unescape(re.sub(r"<[^>]+>", "", m.group(0))) + "\n```\n", t, flags=re.S)
    t = re.sub(r"<h2[^>]*>(.*?)</h2>", lambda m: "\n\n## " + re.sub(r"<[^>]+>", "", m.group(1)) + "\n", t, flags=re.S)
    t = re.sub(r"<h3[^>]*>(.*?)</h3>", lambda m: "\n\n### " + re.sub(r"<[^>]+>", "", m.group(1)) + "\n", t, flags=re.S)
    t = re.sub(r"<li[^>]*>", "- ", t)
    t = re.sub(r"</(p|li|tr|div|h1|figcaption|blockquote)>", "\n", t)
    t = re.sub(r"</t[dh]>", " | ", t)
    t = re.sub(r"<[^>]+>", "", t)
    t = html.unescape(t)
    t = re.sub(r"[ \t]+", " ", t)
    t = re.sub(r"\n{3,}", "\n\n", t)
    return t.strip()

# Page chrome rewritten to plain structure before conversion; the page title replaces <h1>.
MD_PREP = [
    (r'<p class="eyebrow">.*?</p>|<h1[^>]*>.*?</h1>|<(\w+)[^>]*aria-hidden="true"[^>]*>.*?</\1>', ""),
    (r'<a class="card" href="([^"]+)"><h3>(.*?)</h3><p>(.*?)</p></a>', r'<li><a href="\1">\2</a>: \3</li>'),
    (r'<div class="stat"><b>(.*?)</b><span>(.*?)</span></div>', r"<li><b>\1</b> \2</li>"),
    (r'<div class="(?:cards|stat-row)">(.*?)</div>', r"<ul>\1</ul>"),
    (r'<span class="tag">(.*?)</span>(<h3>.*?</h3>)', r"\2<p><code>\1</code></p>"),
    (r'<p class="callout-title">(.*?)</p>', r"<p><strong>\1</strong></p>"),
    (r'<div class="callout">\s*<div>(.*?)</div>\s*</div>', r"<blockquote>\1</blockquote>"),
]
INLINE = {"a", "b", "strong", "em", "i", "code", "span", "small", "kbd", "sup", "sub"}
VOID = {"br", "hr", "img", "wbr"}

class Tree(HTMLParser):
    def __init__(self, src):
        super().__init__()
        self.root = ["root", {}, []]
        self.stack = [self.root]
        self.feed(src)
        self.close()
    def handle_starttag(self, tag, attrs):
        node = [tag, dict(attrs), []]
        self.stack[-1][2].append(node)
        if tag not in VOID:
            self.stack.append(node)
    def handle_endtag(self, tag):
        for i in range(len(self.stack) - 1, 0, -1):
            if self.stack[i][0] == tag:
                del self.stack[i:]
                break
    def handle_data(self, data):
        self.stack[-1][2].append(data)

def raw(n):
    return n if isinstance(n, str) else "".join(raw(k) for k in n[2])

def find(n, tag):
    for k in n[2]:
        if not isinstance(k, str):
            yield from ([k] if k[0] == tag else find(k, tag))

def inline(n, url):
    if isinstance(n, str):
        return re.sub(r"([\\`*_\[\]<])", r"\\\1", re.sub(r"\s+", " ", n))
    tag, attrs, kids = n
    if tag == "code":
        t = re.sub(r"\s+", " ", raw(n))
        return f"`` {t} ``" if "`" in t else f"`{t}`"
    s = "".join(inline(k, url) for k in kids)
    if tag in ("b", "strong") and s.strip():
        return f"**{s.strip()}**"
    if tag in ("em", "i") and s.strip():
        return f"*{s.strip()}*"
    if tag == "a" and attrs.get("href"):
        h = attrs["href"]
        h = BASE + url + h if h.startswith("#") else BASE + h if h.startswith("/") else h
        return f"[{s.strip()}]({h})"
    return s

def blocks(kids, url):
    out, buf = [], []
    def flush():
        t = "".join(buf).strip()
        buf.clear()
        if t:
            out.append(t)
    for k in kids:
        if isinstance(k, str) or k[0] in INLINE:
            buf.append(inline(k, url))
        else:
            flush()
            out += block(k, url)
    flush()
    return out

def block(n, url):
    tag, attrs, kids = n
    if tag in ("h2", "h3", "h4", "h5", "h6"):
        return ["#" * int(tag[1]) + " " + "".join(inline(k, url) for k in kids).strip()]
    if tag == "pre":
        code, fence = raw(n).strip("\n"), "```"
        while fence in code:
            fence += "`"
        return [f"{fence}{attrs.get('data-lang', '')}\n{code}\n{fence}"]
    if tag in ("ul", "ol"):
        n0, items = int(attrs.get("start", 1)), []
        for i, li in enumerate(k for k in kids if not isinstance(k, str) and k[0] == "li"):
            mark = f"{n0 + i}. " if tag == "ol" else "- "
            body = "\n\n".join(blocks(li[2], url)).replace("\n", "\n" + " " * len(mark))
            items.append(mark + re.sub(r"\n +\n", "\n\n", body))
        return ["\n".join(items)] if items else []
    if tag == "blockquote":
        return ["\n".join(("> " + l).rstrip() for l in "\n\n".join(blocks(kids, url)).split("\n"))]
    if tag == "cite":
        return ["— " + "".join(inline(k, url) for k in kids).strip()]
    if tag == "table":
        rows = [[re.sub(r"\s+", " ", "".join(inline(k, url) for k in c[2])).strip().replace("|", r"\|")
                 for c in r[2] if not isinstance(c, str) and c[0] in ("th", "td")] for r in find(n, "tr")]
        if not rows:
            return []
        w = max(len(r) for r in rows)
        rows = [r + [""] * (w - len(r)) for r in rows]
        lines = ["| " + " | ".join(r) + " |" for r in rows]
        return ["\n".join([lines[0], "|" + " --- |" * w] + lines[1:])]
    if tag == "svg":
        t = next(find(n, "title"), None)
        return [f"*Diagram: {raw(t).strip()}*"] if t else []
    if tag == "hr":
        return ["---"]
    if tag in ("script", "style"):
        return []
    return blocks(kids, url)

def markdown_of(body, url):
    for pat, rep in MD_PREP:
        body = re.sub(pat, rep, body, flags=re.S)
    return "\n\n".join(blocks(Tree(body).root[2], url))

def markdown_page(url, meta, body):
    head = f"# {meta['title']}\n\nSource: {BASE}{url}\n\n"
    h1 = re.search(r"<h1[^>]*>(.*?)</h1>", body, re.S)
    lede = html.unescape(re.sub(r"<[^>]+>", "", h1.group(1))).strip() if h1 else ""
    a, b = lede.lower(), meta["title"].lower()
    if lede and a not in b and b not in a:   # keep an <h1> that says more than the title, like the home tagline
        head += f"**{lede}**\n\n"
    return head + markdown_of(body, url) + "\n"

def write_feed(changelog_body):
    entries = re.findall(r'<h2 id="([^"]+)">([\d.]+) <small>· ([\d-]+)</small></h2>(.*?)(?=<h2 id=|$)', changelog_body, flags=re.S)
    items = []
    for anchor, ver, date, body in entries:
        summary = html.escape(text_of(body)[:600])
        items.append(f"""  <entry>
    <title>Fendlemark {ver}</title>
    <link href="{BASE}/changelog/#{anchor}"/>
    <id>{BASE}/changelog/#{anchor}</id>
    <updated>{date}T00:00:00Z</updated>
    <summary>{summary}</summary>
  </entry>""")
    feed = f"""<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Fendlemark releases</title>
  <subtitle>Release notes for the Fendlemark ML platform console.</subtitle>
  <link href="{BASE}/feed.xml" rel="self"/>
  <link href="{BASE}/changelog/"/>
  <id>{BASE}/changelog/</id>
  <updated>{entries[0][2] if entries else DEFAULT_DATE}T00:00:00Z</updated>
  <author><name>The Fendlemark Keepers</name></author>
{chr(10).join(items)}
</feed>
"""
    open(os.path.join(ROOT, "feed.xml"), "w", encoding="utf-8").write(feed)

def write_llms(pages):
    lines = ["# Fendlemark", "",
             "> Fendlemark is an open-source Go and Vue 3 framework that unifies the user experiences of MLflow, Metaflow, Argo Workflows, and Kubeflow into a single ML platform console. One binary, four read-only adapters, one deterministic entity graph (the lattice), one console.",
             "", "Module path: fendlemark.develops.guru/fendlemark · npm: @fendlemark/ui · Licence: MIT · Current version: 0.9.4", "",
             "## Documentation", ""]
    for url, meta in pages:
        if url.startswith("/docs/"):
            lines.append(f"- [{meta['title']}]({BASE}{url}index.md): {meta['description']}")
    lines += ["", "## Project", ""]
    for url, meta in pages:
        if not url.startswith("/docs/") and url != "/":
            lines.append(f"- [{meta['title']}]({BASE}{url}index.md): {meta['description']}")
    lines += ["", "## Optional", "", f"- [Full documentation as one text file]({BASE}/llms-full.txt)"]
    open(os.path.join(ROOT, "llms.txt"), "w", encoding="utf-8").write("\n".join(lines) + "\n")
    full = ["# Fendlemark: complete documentation", ""]
    for url, meta in pages:
        full += [meta["_md"].rstrip("\n"), "", "---", ""]
    open(os.path.join(ROOT, "llms-full.txt"), "w", encoding="utf-8").write("\n".join(full))

def write_404():
    page = LAYOUT.replace('<link rel="alternate" type="text/markdown" href="{{markdown}}">\n', "")
    body = ('<p class="eyebrow">404</p><h1>There is nothing here.</h1>'
            '<p class="lede">The page you asked for is not part of Fendlemark. Nothing has ever been removed, so it was never here. '
            'Try the <a href="/docs/">documentation</a>, the <a href="/docs/getting-started/">getting started guide</a>, or the <a href="/">home page</a>.</p>'
            '<p class="fine">If you arrived here from <code>go get</code>, the module path is <code>fendlemark.develops.guru/fendlemark</code> and this page carries the import metadata the Go toolchain needs. See <a href="/fendlemark/">Packages</a>.</p>')
    for k, v in {"{{title}}": "Not found · Fendlemark", "{{description}}": "Page not found.", "{{canonical}}": BASE + "/404.html",
                 "{{body}}": body, "{{sidebar}}": sidebar("/404"), "{{pager}}": "", "{{jsonld}}": "", "{{layout}}": "docs",
                 "{{year}}": "2026", "{{version}}": "0.9.4"}.items():
        page = page.replace(k, v)
    page = page.replace('<meta name="robots" content="index, follow, max-snippet:-1, max-image-preview:large">', '<meta name="robots" content="noindex">')
    open(os.path.join(ROOT, "404.html"), "w", encoding="utf-8").write(page)

def main():
    urls = []
    pages = []
    for dirpath, _, files in os.walk(SRC):
        for f in sorted(files):
            if f.endswith(".html"):
                src = os.path.join(dirpath, f)
                url = render(src)
                urls.append(url)
                meta, body = read_fragment(src)
                meta["_md"] = markdown_page(url, meta, body)
                open(os.path.splitext(out_for(url))[0] + ".md", "w", encoding="utf-8").write(meta["_md"])
                pages.append((url, meta))
                if url_for(src) == "/changelog/":
                    write_feed(body)
    pages.sort(key=lambda t: (t[0] != "/", t[0]))
    write_llms(pages)
    write_404()
    urls.sort(key=lambda u: (u != "/", u))
    sm = ['<?xml version="1.0" encoding="UTF-8"?>',
          '<urlset xmlns="http://www.sitemap.org/schemas/sitemap/0.9">']
    for u in urls:
        pri = "1.0" if u == "/" else ("0.9" if u == "/docs/getting-started/" else "0.7")
        sm.append(f"  <url><loc>{BASE}{u}</loc><lastmod>{DEFAULT_DATE}</lastmod><priority>{pri}</priority></url>")
    sm.append("</urlset>")
    open(os.path.join(ROOT, "sitemap.xml"), "w").write("\n".join(sm) + "\n")
    print(f"built {len(urls)} pages")
    for u in urls:
        print("  ", u)

if __name__ == "__main__":
    main()
