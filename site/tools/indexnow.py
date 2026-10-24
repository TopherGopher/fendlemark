#!/usr/bin/env python3
"""Tell IndexNow search engines (Bing, Yandex, Seznam, Naver, Yep) which pages changed.

    python3 site/tools/indexnow.py [BEFORE_SHA]

Run from the repo root after a deploy. With a commit, submits the sitemap pages
whose generated HTML changed since it; without one, or when the commit is
unknown (a repository's first push, a force push), submits every page.
"""
import json, os, re, subprocess, sys, urllib.error, urllib.request

KEY = "d62f5c1b3f61ccd3ff333f8d0e87b3c4"   # served at /<KEY>.txt
HOST = "fendlemark.app"
BASE = "https://" + HOST
PUBLIC = os.path.join("site", "public")

def page_file(url):
    path = url[len(BASE):].strip("/")
    return os.path.join(PUBLIC, path, "index.html") if path else os.path.join(PUBLIC, "index.html")

def changed_since(sha):
    if not sha or set(sha) == {"0"}:
        return None
    if subprocess.run(["git", "cat-file", "-e", sha + "^{commit}"], capture_output=True).returncode:
        return None
    out = subprocess.run(["git", "diff", "--name-only", sha, "HEAD", "--", PUBLIC],
                         capture_output=True, text=True, check=True).stdout
    return set(out.split())

def main():
    if open(os.path.join(PUBLIC, KEY + ".txt")).read().strip() != KEY:
        sys.exit(f"{PUBLIC}/{KEY}.txt does not contain the key")
    urls = re.findall(r"<loc>([^<]+)</loc>", open(os.path.join(PUBLIC, "sitemap.xml")).read())
    changed = changed_since(sys.argv[1] if len(sys.argv) > 1 else "")
    if changed is not None:
        urls = [u for u in urls if page_file(u) in changed]
    if not urls:
        print("no pages changed; nothing to submit")
        return
    body = json.dumps({"host": HOST, "key": KEY, "keyLocation": f"{BASE}/{KEY}.txt", "urlList": urls}).encode()
    req = urllib.request.Request("https://api.indexnow.org/indexnow", data=body,
                                 headers={"Content-Type": "application/json; charset=utf-8"})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            print(f"IndexNow {r.status}: submitted {len(urls)} URL(s)")
    except urllib.error.HTTPError as e:
        sys.exit(f"IndexNow {e.code}: {e.read().decode(errors='replace')}")
    for u in urls:
        print("  ", u)

if __name__ == "__main__":
    main()
