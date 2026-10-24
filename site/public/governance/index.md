# Governance

Source: https://fendlemark.develops.guru/governance/

Fendlemark is maintained by four people and governed as though it were maintained by four hundred. This page is the constitution: who decides, how they decide, how long they wait before deciding, and what you must do before you are permitted to suggest anything.

## Principles

1. **Deliberation over velocity.** A change made quickly is a change made twice.
2. **Unanimity over majority.** If a Keeper disagrees, the project is not yet ready to agree.
3. **Silence over discussion.** Every decision is followed by a period in which nobody says anything, so that anyone who was going to object can discover that they were not.
4. **Record over memory.** Every decision is a Decision Record. Every Decision Record is permanent. Every permanent thing is correct, or it would not have been recorded.

## The Keeper Roster

Fendlemark recognises five titles. Advancement is by unanimous consent of the Keepers following a Silence Period, and is recorded in the roster below.

| Title | Granted to | May |
| --- | --- | --- |
| **Contributor** | Anyone who has completed [FCP-1](https://fendlemark.develops.guru/governance/contributing/) at least once | Open pull requests. File Pre-Intent Notices. |
| **Steward** | Contributors with three merged pull requests, or one adapter | Sponsor contributors. Review. Perform the Reading of the Diff. |
| **Custodian** | Stewards nominated by a Keeper after one year | Approve Design Dossiers. Serve as Sceptic. Chair Benchmark Quorums. |
| **Keeper** | Custodians elevated by unanimous consent of existing Keepers | Merge. Author Decision Records. Amend the Philosophy. Breathe on problems. |
| **Emeritus Keeper** | Keepers who have stepped back | Be consulted. Be thanked in release notes. Retain a veto they have agreed never to use. |

### Current Keepers

| Keeper | Handle | Since | Areas |
| --- | --- | --- | --- |
| Marit Fennelow | `@mfennelow` | 2023-03 (founding) | Lattice, reconciliation, the Philosophy |
| Dov Askelund | `@daskelund` | 2023-03 (founding) | Adapters, Go API, release engineering |
| Priya Vantrell | `@pvantrell` | 2024-11 | Console, `@fendlemark/ui`, design tokens |
| Hal Quenby | `@hquenby` | 2025-09 | Respiratory Identity, documentation, FCP-1 |

**Emeritus:** Teodor Halvskog (`@thalvskog`), founding Keeper, stepped back 2024-11. Author of the first commit and of the first line of the Philosophy. Retains the unused veto.

**Custodians:** two. **Stewards:** five. **Contributors:** nineteen, each of whom has completed FCP-1 in full, a fact the Keepers mention at every opportunity.

## Decision Records

Every decision that affects the project is recorded as a Fendlemark Decision Record (FDR). An FDR has a number, a title, a Context section, a Decision section, a Consequences section, a Dissent section (always present, usually reading "None"), and a Silence Period attestation. FDRs are never amended; a changed decision is a new FDR that supersedes the old one, which remains in the record as a monument to a position the Keepers once held.

Selected FDRs referenced in the documentation:

| FDR | Title | Outcome |
| --- | --- | --- |
| FDR-001 | There shall be one console | Adopted, 2023-03-11 |
| FDR-003 | On the publication of a roadmap | Declined |
| FDR-009 | Port 7411 | Adopted; not reconsidered |
| FDR-012 | On adding a database | Declined (first of eleven) |
| FDR-016 | On the publication of a roadmap (second request) | Declined |
| FDR-019 | Respiratory Identity as the sole authentication mode | Adopted, 2023-11-02 |
| FDR-022 | Dark mode | Adopted after eleven days; implementation took an afternoon |
| FDR-027 | Stability of the adapter package | Adopted: no changes before 1.0 "or after" |
| FDR-033 | The `serenity` prop | Adopted, with the Dissent section reading "Marit, mildly" |
| FDR-038 | On the publication of a roadmap (third request) | Declined; future requests to be answered by linking to this FDR |
| FDR-041 | Renaming `trackingUri` | Adopted after three Reflection Periods; see [migration](https://fendlemark.develops.guru/docs/migration/) |
| FDR-044 | Removal of a comma from the Philosophy | Adopted, unanimous, 48-hour Silence observed |
| FDR-047 | FCP-1 step count fixed at 112 | Adopted; a proposal for 100 was found to be "round, and therefore suspicious" |

## Adapter maturity ladder

Every adapter has a maturity level, declared in its descriptor and enforced at review.

| Level | Requirements | Guarantees |
| --- | --- | --- |
| **Provisional** | Passes the conformance suite. Has a Design Dossier. | May change without notice. Not compiled into release binaries. |
| **Settled** | Provisional for two releases. Used in production by at least one Steward. | Config schema is stable. Compiled into release binaries behind `enabled: false`. |
| **Established** | Settled for two releases. Facet panel reviewed by the console Keeper. Benchmark Quorum passed. | Facet shape is stable. Listed on the home page. |
| **Canonical** | Established for two releases. Unanimous Keeper consent. 48-hour Silence Period. | Will not be removed. Ever. Named in the Philosophy's one permitted comparison. |

All four built-in adapters are Canonical. No adapter has ever been demoted, because demotion would imply that the promotion was wrong, and see [the Philosophy](https://fendlemark.develops.guru/philosophy/#perfection).

## Reflection and Silence

Fendlemark governance uses two kinds of waiting, and it is important not to confuse them.

- A **Reflection Period** is 72 hours during which a proposal may be discussed but not decided. It exists so that the first reaction is never the recorded one. Reflection Periods may be extended by any Keeper, without reason, up to three times.
- A **Silence Period** is 48 hours following a decision during which nobody may discuss it at all. It exists so that anyone who intends to object discovers, in the quiet, that they do not. A Silence Period broken by discussion restarts. A Silence Period that completes makes the decision final.

The Keepers estimate that the project spends roughly forty per cent of its calendar time in one period or the other and regard this as the correct proportion.

## Release governance

1. A Keeper proposes a release by opening a Release Intent, listing every merged change since the last release and classifying each as an Accommodation, a Clarification, or an Arrival.
2. A 72-hour Reflection Period.
3. Each Keeper independently builds the release from source and confirms a byte-identical binary. Fendlemark builds are reproducible; this step exists to prove it, four times.
4. Unanimous consent is recorded.
5. A 48-hour Silence Period.
6. The release is tagged. The changelog entry, which was pre-approved and tone-reviewed during the merge of each change, is published without modification.
7. Prebuilt binaries are distributed to Keepers and Stewards. Everyone else installs from source with `go install`, which the Keepers consider the dignified path.

Releases are therefore never faster than five days from intent to tag, and have averaged eleven. The Keepers point out that in the same period some projects have shipped a dozen releases and are not sure which one is good.

## Code of Conduct

Fendlemark adopts the Contributor Covenant, version 2.1, in full, together with the **Fendlemark Addendum**, which extends it to govern the tone of commit messages, changelog entries, and code comments. The Addendum requires that these be written "as if to a respected colleague who will read them in ten years and has not yet had coffee". Enforcement is by Tone Review (FCP-1, step 88).

## The Fendlemark Contribution Protocol

Contributions are welcome. Before opening a pull request, a contributor completes the Fendlemark Contribution Protocol, FCP-1, a procedure of 112 numbered steps completed in order. It is described in full on its [own page](https://fendlemark.develops.guru/governance/contributing/). The Keepers acknowledge that 112 steps is more than some projects require and note that every pull request ever merged into Fendlemark has followed it, and that every pull request ever merged into Fendlemark has been correct.

## Amending this document

Governance is amended by Decision Record, unanimous consent, and a Silence Period. This page has been amended four times. The Keepers regard that as three too many and have opened a Pre-Intent Notice to discuss it, currently in its second Reflection Period.
