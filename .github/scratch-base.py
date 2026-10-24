"""Write an OCI image layout holding an empty image, with no layers and no
files, for each platform given, under one index. ko has no FROM scratch of its
own; image.yml pushes this to a registry that lasts as long as the job and
builds on it.

    python3 -I .github/scratch-base.py OUTDIR linux/amd64 linux/arm64 ...
"""
import hashlib, json, os, sys

out, platforms = sys.argv[1], sys.argv[2:]
os.makedirs(os.path.join(out, "blobs", "sha256"), exist_ok=True)

def blob(obj, media_type):
    b = json.dumps(obj, separators=(",", ":")).encode()
    digest = hashlib.sha256(b).hexdigest()
    with open(os.path.join(out, "blobs", "sha256", digest), "wb") as f:
        f.write(b)
    return {"mediaType": media_type, "digest": "sha256:" + digest, "size": len(b)}

manifests = []
for p in platforms:
    os_name, arch, *variant = p.split("/")
    platform = {"os": os_name, "architecture": arch, **({"variant": variant[0]} if variant else {})}
    config = blob({**platform, "rootfs": {"type": "layers", "diff_ids": []}, "config": {}},
                  "application/vnd.oci.image.config.v1+json")
    manifest = blob({"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json",
                     "config": config, "layers": []}, "application/vnd.oci.image.manifest.v1+json")
    manifests.append({**manifest, "platform": platform})

with open(os.path.join(out, "index.json"), "w") as f:
    json.dump({"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json", "manifests": manifests}, f)
with open(os.path.join(out, "oci-layout"), "w") as f:
    json.dump({"imageLayoutVersion": "1.0.0"}, f)
