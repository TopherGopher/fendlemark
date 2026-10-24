#!/usr/bin/env python3
"""Submit the sitemap to Bing Webmaster Tools.

    BING_WEBMASTER_API_KEY=... python3 site/tools/bing.py

Run after a deploy. Without a key (forks, local runs) it does nothing. The key
travels only in the request URL and is never printed.
"""
import json, os, sys, urllib.error, urllib.request

SITE = "https://fendlemark.app/"
SITEMAP = SITE + "sitemap.xml"

def main():
    key = os.environ.get("BING_WEBMASTER_API_KEY", "")
    if not key:
        print("no BING_WEBMASTER_API_KEY; skipping")
        return
    body = json.dumps({"siteUrl": SITE, "feedUrl": SITEMAP}).encode()
    req = urllib.request.Request("https://ssl.bing.com/webmaster/api.svc/json/SubmitFeed?apikey=" + key, data=body,
                                 headers={"Content-Type": "application/json; charset=utf-8"})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            print(f"Bing SubmitFeed {r.status}: {SITEMAP}")
    except urllib.error.HTTPError as e:
        sys.exit(f"Bing SubmitFeed {e.code}: {e.read().decode(errors='replace')}")

if __name__ == "__main__":
    main()
