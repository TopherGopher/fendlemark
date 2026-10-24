#!/usr/bin/env python3
"""Build a self-contained preview of the site for the Artifact tool.

Every page becomes a flat file (docs/getting-started/ -> docs-getting-started.html)
with CSS and JS inlined and internal links rewritten. The home page is emitted as a
fragment (no html/head/body) because the artifact host wraps it in its own skeleton.
Usage: python3 site/tools/artifact.py OUTDIR
"""
import os, re, sys, glob, json
SITE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PUB = os.path.join(SITE, "public")
OUT = sys.argv[1]
os.makedirs(os.path.join(OUT, "assets"), exist_ok=True)
css = open(os.path.join(PUB, "assets", "site.css"), encoding="utf-8").read()
js = open(os.path.join(PUB, "assets", "site.js"), encoding="utf-8").read()

def flat(url):
    if url == "/": return "index.html"
    return url.strip("/").replace("/", "-") + ".html"

pages = sorted(p for p in glob.glob(os.path.join(PUB, "**", "index.html"), recursive=True))
urls = {p: ("/" if p == os.path.join(PUB, "index.html") else "/" + os.path.relpath(os.path.dirname(p), PUB) + "/") for p in pages}

def rewrite(html):
    def link(m):
        href = m.group(1)
        if href.startswith(("http", "mailto:", "#")): return m.group(0)
        if href.startswith("/assets/"): return f'href="{href[1:]}"'
        if href.endswith((".xml", ".txt")): return f'href="{href[1:]}"'
        path, _, frag = href.partition("#")
        if not path.endswith("/"): path += "/"
        return f'href="{flat(path)}{"#" + frag if frag else ""}"'
    html = re.sub(r'href="([^"]+)"', link, html)
    html = re.sub(r'src="/assets/', 'src="assets/', html)
    html = html.replace('<link rel="stylesheet" href="assets/site.css">', "<style>\n" + css + "\n</style>")
    html = html.replace('<script src="assets/site.js" defer></script>', "<script>\n" + js + "\n</script>")
    return html

manifest = {}
for p in pages:
    html = rewrite(open(p, encoding="utf-8").read())
    name = flat(urls[p])
    if name == "index.html":
        # fragment for the wrapped main page
        title = re.search(r"<title>(.*?)</title>", html).group(1)
        style = re.search(r"<style>.*?</style>", html, re.S).group(0)
        fonts = re.search(r'<link rel="stylesheet" href="https://fonts.googleapis.com[^>]+>', html).group(0)
        body = re.search(r"<body>(.*)</body>", html, re.S).group(1)
        html = f"<title>{title}</title>\n{fonts}\n{style}\n{body}"
    open(os.path.join(OUT, name), "w", encoding="utf-8").write(html)
    if name != "index.html": manifest[name] = os.path.join(OUT, name)
for a in ("favicon.svg", "og-image.png", "apple-touch-icon.png"):
    src = os.path.join(PUB, "assets", a)
    dst = os.path.join(OUT, "assets", a)
    open(dst, "wb").write(open(src, "rb").read())
    manifest["assets/" + a] = dst
for t in ("llms.txt", "feed.xml", "sitemap.xml", "robots.txt"):
    dst = os.path.join(OUT, t)
    open(dst, "wb").write(open(os.path.join(PUB, t), "rb").read())
    manifest[t] = dst
json.dump(manifest, open(os.path.join(OUT, "manifest.json"), "w"), indent=1)
print(len(pages), "pages ->", OUT)
