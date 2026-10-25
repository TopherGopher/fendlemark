# Contributing to Fendlemark

Contributions are welcome. Human review is a required part of the process:
every pull request is read and approved by a maintainer before it is merged,
whatever its size. Say what your change does and why in the pull request, and
make sure the checks under [Developing](#developing) pass.

To report a problem or ask a question, use the
[contact form](https://contact.fendlemark.app/).

## Spelling

Fendlemark is written in British English: *behaviour*, *colour*, *recognise*,
*licence* for the noun, *enrolment*. That covers documentation, comments, help
text, error messages, changelog entries, and commit messages, and review
checks it. Names that come from elsewhere keep their own
spelling (CSS `color`, `NO_COLOR`, MLflow's *artifacts*), and existing
identifiers are never renamed just to change their spelling.

## Developing

You need Go 1.26.10 or later and Node.js 22.12 or later.

```sh
go test ./...                       # control plane, adapters, lattice, end-to-end
(cd ui && npm ci && npm run build)  # @fendlemark/ui
```

CI runs the same two.

## Security

Please don't open public issues for security reports. See [SECURITY.md](SECURITY.md).
