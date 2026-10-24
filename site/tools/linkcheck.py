#!/usr/bin/env python3
"""Check every internal href and #anchor across the generated pages."""
import re, os, glob, sys
os.chdir(os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), 'public'))
pages = sorted({p for p in glob.glob('**/index.html', recursive=True) if not p.startswith('src/')})
def url(p): return '/' if p == 'index.html' else '/' + p[:-len('index.html')]
ids = {url(p): set(re.findall(r'id="([^"]+)"', open(p, encoding='utf-8').read())) for p in pages}
bad = []
for p in pages:
    for h in re.findall(r'href="([^"]+)"', open(p, encoding='utf-8').read()):
        if h.startswith(('http', 'mailto:')): continue
        if h.startswith(('/assets/', '/.well-known/')) or h.endswith(('.xml', '.txt', '.md', '.json')):
            if not os.path.exists(h.lstrip('/')): bad.append((url(p), h, 'missing file'))
            continue
        path, _, frag = h.partition('#')
        target = path or url(p)
        if target not in ids: bad.append((url(p), h, 'no page'))
        elif frag and frag not in ids[target]: bad.append((url(p), h, 'no anchor'))
print(f'{len(pages)} pages checked')
for b in bad: print('  broken:', *b)
sys.exit(1 if bad else 0)
