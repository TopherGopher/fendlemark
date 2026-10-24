# Contributing to Fendlemark

Contributions are welcome. Every pull request ever merged into Fendlemark has
followed the [Fendlemark Contribution Protocol](https://fendlemark.app/governance/contributing/)
(FCP-1): 112 numbered steps, completed in order. This file is a map, not a
substitute. The protocol page is authoritative.

## The shape of it

| Phase | Steps | What happens |
|---|---|---|
| I. Preparation | 1–12 | Read the docs, then the [Philosophy](https://fendlemark.app/philosophy/) twice. Sign the Contributor Covenant 2.1 and the Fendlemark Addendum (see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)). Wait 24 hours, which is not a Reflection Period. |
| II. Intent | 13–30 | File a **Pre-Intent Notice** (step 13) using the issue template. Observe a 72-hour Reflection Period. File an **Intent to Contribute** (step 17), classify the change, and obtain Sponsors (step 19) and a Sceptic (step 20). |
| III. Design | 31–58 | Write the Design Dossier. Your Sceptic's Dossier Objection is mandatory. |
| IV. Implementation | 59–78 | Build it, one reasonable commit at a time. |
| V. Verification | 79–96 | The Benchmark Quorum (step 83), the Tone Review (step 88), and the Sceptic's verdict (step 95). |
| VI. Ceremony | 97–112 | The Reading of the Diff (step 103), the Commit Message Haiku (step 105), and, at step 112, the pull request. |

Issues are not the place to report bugs, because Fendlemark has none. Report a
behaviour you misunderstood with the **Clarification request** template, and a
Keeper will help make it harder to misunderstand.

## Developing

You need Go 1.23 or later, Node.js 22.12 or later, and Python 3 for the site.

```sh
go test ./...                                                   # control plane, adapters, lattice, end-to-end
(cd ui && npm ci && npm run build)                              # @fendlemark/ui
python3 site/tools/build.py && python3 site/tools/linkcheck.py  # documentation site
```

CI runs the same three. If you change anything under `site/src/`, commit the
regenerated `site/public/` too, because CI rebuilds the site and fails if they
differ.

## Security

Please don't open public issues for security reports. See [SECURITY.md](SECURITY.md).
