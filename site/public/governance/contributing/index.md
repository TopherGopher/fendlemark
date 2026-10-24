# Contributing: the Fendlemark Contribution Protocol (FCP-1)

Source: https://fendlemark.develops.guru/governance/contributing/

Thank you for considering a contribution to Fendlemark. This page describes the Fendlemark Contribution Protocol, FCP-1: the 112 steps a contributor completes, in order, before opening a pull request. Every pull request ever merged has followed it. Every pull request ever merged has been correct. The Keepers do not regard this as a coincidence.

> **Before step 1**
>
> FCP-1 begins when you first form the intention to change Fendlemark. If you have already opened a pull request, close it, and begin at step 1. The Keepers will not hold it against you, and will say so in a way that makes clear they are holding it against you a little.

## Overview

| Phase | Steps | Typical duration |
| --- | --- | --- |
| I. Preparation | 1 to 12 | One evening |
| II. Intent | 13 to 30 | Two to three weeks, mostly Reflection |
| III. Design | 31 to 58 | Two to four weeks |
| IV. Implementation | 59 to 78 | Shorter than you expect; the thinking is done |
| V. Verification | 79 to 96 | One week |
| VI. Ceremony | 97 to 112 | Fifty-one hours, of which forty-eight are silent |

A single-line typo fix follows the same 112 steps. The Keepers have been asked whether this is proportionate and have answered, in [FDR-047](https://fendlemark.develops.guru/governance/#decisions), that proportion is a property of the outcome, not the process.

## Phase I: Preparation

1. Read the documentation. All of it. The CLI reference counts.
2. Read the [Philosophy](https://fendlemark.develops.guru/philosophy/).
3. Read the Philosophy again, this time without skimming the section on comparisons.
4. Read [Governance](https://fendlemark.develops.guru/governance/), including the Decision Records table, and note which FDR your intended change would most offend.
5. Install Fendlemark from source and run it against at least one real adapter. A fixture does not count. The Keepers can tell.
6. Confirm your Go version matches `go.mod` to the patch release. Record it.
7. Confirm your Node.js version matches `console/.nvmrc` to the patch release. Record it, even if your change does not touch the console.
8. Sign the Contributor Covenant 2.1.
9. Sign the Fendlemark Addendum to the Contributor Covenant, which governs the tone of commit messages, changelog entries, and comments.
10. Read the Addendum's worked examples of acceptable and unacceptable commit message tone. Note that "fix typo" is unacceptable and "Clarify the spelling of *reconciliation* in the CLI help text" is acceptable.
11. Create a contributor profile in the project's contributor registry, including your handle, your time zone, and your preferred Reflection Period notification channel.
12. Wait 24 hours. This is not a Reflection Period. It is simply a day, and the Keepers have found that most intended contributions do not survive one.

## Phase II: Intent

13. File a **Pre-Intent Notice**: one paragraph stating that you are considering a contribution, in which area, and why you believe the area is imperfect. Do not describe the change.
14. Observe a 72-hour Reflection Period on the Pre-Intent Notice.
15. If, during Reflection, a Keeper replies with a link to an FDR, read it. If it resolves your concern, withdraw the Notice and return to step 12 for a different concern, or to your life.
16. If no Keeper replies, that is consent to proceed. Silence is how the Keepers say yes.
17. File an **Intent to Contribute** referencing the Pre-Intent Notice by number. The Intent describes the change in no more than three sentences and no fewer than two.
18. Classify the change as an Accommodation (the world changed), a Clarification (you misunderstood, and the code will now be harder to misunderstand), or an Arrival (something new). Arrivals require two Sponsors; the others require one.
19. Obtain Sponsors from the Steward roster. A Sponsor agrees to read everything you produce from this point on. Sponsors are approached privately and may decline without reason.
20. Obtain a **Sceptic** from the Custodian roster. The Sceptic's role is to believe the change is unnecessary until step 95. The Sceptic may not be someone who has previously sponsored you.
21. Observe a 72-hour Reflection Period on the Intent.
22. During Reflection, your Sceptic will post the **Sceptic's Question**: "What would we lose by not doing this?" Do not answer yet.
23. After Reflection, answer the Sceptic's Question in writing, in one paragraph, without using the words "obviously", "simply", or "just".
24. Your Sponsors confirm that they have read the answer. Your Sceptic confirms that they remain sceptical. This is the expected outcome.
25. Search the Decision Records for any FDR that addresses the change. If one does and it declined the change, your Intent becomes a **Petition to Supersede**, which requires a Keeper co-signer and restarts at step 21.
26. Search closed pull requests for prior attempts. If one exists, read its Reading of the Diff transcript (step 103) and note where it went wrong.
27. Estimate the number of files your change will touch. Record the estimate. It will be compared to the actual number at step 85 and the discrepancy will be discussed.
28. Choose a **working name** for the change, which will appear in all subsequent documents. Working names are reviewed at step 44.
29. A Keeper acknowledges the Intent with the word "Noted". The Keepers use no other word for this.
30. Observe a 48-hour Silence Period. Do not begin design work. The Keepers have asked us to be clear that they can tell when design work has begun during Silence, because the Dossier is always slightly too polished.

## Phase III: Design

31. Open a **Design Dossier** from the template. The template has eleven mandatory sections. All eleven are mandatory for all changes, including typo corrections.
32. Section 1, *Context*: what is true today.
33. Section 2, *Why this change is unnecessary*: the strongest case against, written by you, in good faith. Dossiers with a weak Section 2 are returned.
34. Section 3, *Why we are doing it anyway*: the response to Section 2. May not be longer than Section 2.
35. Section 4, *Proposed change*: what will be different. Diagrams are encouraged. Inline SVG only; raster images are returned.
36. Section 5, *Alternatives considered*: at least three, one of which must be "do nothing", analysed with the same seriousness as the others.
37. Section 6, *Effect on the lattice*: even if none. If none, explain why none, with reference to the reconciler's determinism guarantee.
38. Section 7, *Effect on the console*: even if none. If the change adds a visual element, state why it is not a loading spinner.
39. Section 8, *Effect on Respiratory Identity*: even if none. The Keepers have found that this section occasionally surfaces an effect.
40. Section 9, *Migration*: if any key, flag, field, or export is renamed, the migration guide is drafted here, in full, in the style of the [0.8 to 0.9 guide](https://fendlemark.develops.guru/docs/migration/).
41. Section 10, *Test plan*: every test you will add or change, by name, before any of them exist.
42. Section 11, *Changelog entry*: the exact text, in the Addendum's tone, under the correct heading (Accommodations, Clarifications, Arrivals).
43. Submit the Dossier to your Sponsors and Sceptic. Observe a 72-hour Reflection Period.
44. **Naming Review.** A Custodian reviews every new identifier in the Dossier (types, functions, config keys, flags, CSS tokens, the working name). Identifiers must be `snake_case` in YAML, `camelCase` in TypeScript, `PascalCase` for exported Go, and must not abbreviate any word that has fewer than ten letters.
45. **Secondary Naming Review.** Any identifier longer than twelve characters is reviewed again by a second Custodian, who must independently arrive at the same name from the Dossier's description. If they do not, the name is wrong or the description is.
46. Revise the Dossier. Mark revisions. Do not delete the original text; strike it through so that the Dossier records how the thinking moved.
47. Your Sceptic posts their **Dossier Objection**, which is mandatory and may be as short as "Section 2 was more persuasive than Section 3". Respond in writing.
48. Both Sponsors (or the one Sponsor, for non-Arrivals) sign the Dossier.
49. A Custodian approves the Dossier with the phrase "Sufficiently considered". The Keepers use no other phrase for this.
50. Observe a 72-hour Reflection Period on the approved Dossier.
51. If your change touches the `adapter` package, stop. Read [FDR-027](https://fendlemark.develops.guru/governance/#decisions). Your Intent becomes a Petition to Supersede; return to step 25. The Keepers note that nobody has reached step 51 by this route and continued.
52. If your change adds an adapter, draft its Descriptor and conformance test now, from the Dossier alone, and attach them. The adapter's maturity is Provisional and will remain so for at least two releases.
53. If your change adds a configuration key, draft its row in the [configuration reference](https://fendlemark.develops.guru/docs/configuration/), including a default and a one-sentence rationale for the default. Defaults without rationale are returned.
54. If your change adds a CLI flag, draft its help text. Help text is reviewed for tone under the Addendum.
55. If your change adds a `@fendlemark/ui` prop, draft its row in the component table, and state in one sentence why the component could not infer the value.
56. If your change adds an error code, reserve the number in the error registry. FRI codes are allocated by the Respiratory Identity Keeper personally.
57. Record, in the Dossier, the date and time at which you will begin implementation. This must be at least 24 hours in the future.
58. Wait until that time.

## Phase IV: Implementation

59. Create a branch named `fcp/<intent-number>-<working-name>`. No other branch name format is accepted by the repository's hooks.
60. Implement the change exactly as described in Dossier Section 4. If implementation reveals that Section 4 was wrong, stop, amend the Dossier (strikethrough, not deletion), and return to step 43.
61. Write every test named in Dossier Section 10. Write no test not named there without amending Section 10.
62. Run `gofmt`, `go vet`, and the project's linter configuration. Zero findings. Not "zero new findings". Zero.
63. Run the console's formatter, type checker, and linter if you touched the console, and also if you did not, because the Keepers once found a drift that way.
64. **Alphabetical Import Attestation.** Confirm that every import block you touched is grouped (standard library, third party, Fendlemark) and alphabetised within groups. Attest to this in writing in the pull request template. The linter checks it; the attestation is for you.
65. Commit in logical units. Each commit message follows the Addendum: an imperative first line of at most 60 characters that would make sense to a respected colleague in ten years, a blank line, and a body explaining why, not what.
66. Each commit body ends with `FCP-Intent: <number>` and `FCP-Dossier: <number>` trailers.
67. Reorder, squash, or split commits until each one would be a reasonable change on its own. Force-pushing to your own branch is permitted and expected; the Keepers regard a tidy history as a courtesy to the Reading of the Diff.
68. Add or update documentation for every user-visible change, in the same pull request. Documentation in a follow-up pull request is a Clarification that admits the original was unclear.
69. Update the configuration reference, CLI reference, component table, or API reference as drafted in steps 53 to 56. The drafted text may be used verbatim; it was approved.
70. Add the changelog entry from Dossier Section 11, verbatim, to `CHANGELOG.md` under *Unreleased*. Do not improve it. It was approved.
71. Build the binary with `make release-local` and confirm the build is reproducible by building it twice and comparing checksums.
72. Run the full test suite three times. The Keepers do not believe in flaky tests; a test that fails once in three runs is a bug, and it is now yours.
73. Run `fendlemark doctor` against your built binary with the example configuration. Zero warnings.
74. **Screenshot Notarisation.** If the change is visible in the console, capture screenshots with `npm run screenshots`: every affected view, at 2x device pixel ratio, in both light and dark themes, at 1440 and 390 pixel widths, with the cursor hidden and the lattice generation number visible in the footer. Attach all of them to the Dossier.
75. Review the screenshots for any element that could be mistaken for a loading indicator. Remove it.
76. Measure the production bundle size of the console before and after. Any increase above 2 KiB gzipped requires a sentence in the Dossier explaining what the bytes are for.
77. Measure the binary size before and after. Any increase above 100 KiB requires the same.
78. Record the actual number of files touched. Compare to the estimate from step 27. Note the discrepancy in the Dossier without explanation; the explanation is step 85.

## Phase V: Verification

79. Deploy your branch against a real platform with at least two adapters enabled and at least one correlation between them. Run it for 24 hours.
80. Record the lattice generation number at the start and end of the 24 hours, and confirm that `fendlemark lattice stats` is consistent with the adapters' own counts.
81. Run `fendlemark lattice explain` on three entities of your choosing and one of your Sceptic's choosing. Attach the output.
82. If your change affects reconciliation, run the determinism check: two fresh instances, same adapters, byte-identical `lattice stats --json`.
83. **Benchmark Quorum.** For any change touching the lattice, the server, or an adapter's `Facets` path: run the benchmark suite before and after, on the same machine, three times each, and convene a Quorum of one Custodian and one Sponsor to review the numbers. Regressions above 2% require a Dossier amendment. Improvements above 10% require a Dossier amendment explaining why the previous code was slow, which the Keepers will read with interest.
84. If your change adds or alters a facet panel, your Sponsor opens the panel on an entity with no facets from that adapter and confirms the panel is absent, not empty.
85. **Estimate Reconciliation.** Explain, in the Dossier, the discrepancy between step 27 and step 78. "I did not know about X" is acceptable. "It grew" is not.
86. Self-review the complete diff, top to bottom, as if it had been written by someone you respect but do not trust. Record at least one thing you changed as a result. If you changed nothing, review again.
87. Your Sponsors review the diff against the Dossier. Any difference between what was described and what was built is listed. Each item is either fixed or becomes a Dossier amendment.
88. **Tone Review.** A Custodian reviews every commit message, comment, changelog entry, help string, and error message added or changed, under the Addendum. The standard is: a respected colleague, ten years from now, before coffee.
89. Your Sceptic reviews the diff and posts the **Sceptic's Second Question**: "Is this the smallest change that achieves Section 3?" Answer in writing.
90. If the answer to step 89 is no, make it smaller. Return to step 86.
91. Confirm that no file in the diff contains the words "TODO", "FIXME", "HACK", "temporary", or "for now". Fendlemark has no temporary code. It has code, and it has code that was removed.
92. Confirm that no new dependency was added. If one was, confirm that the Dossier's Section 5 considered writing it ourselves, and that the Keepers' standing preference for writing it ourselves was overcome in writing.
93. Rebase onto the current default branch. Re-run steps 62, 63, and 72.
94. Freeze the branch. From this point until step 112, no commit may be added without returning to step 86.
95. Your Sceptic records their verdict: "Unnecessary, but correct", which is the only verdict a Sceptic may give that permits the Protocol to continue. Any other verdict returns to step 43.
96. Observe a 72-hour Reflection Period on the frozen branch.

## Phase VI: Ceremony

97. Fill in the pull request template, offline, in a file you will paste at step 112. The template has fourteen fields. All are required. Field 9 is the Alphabetical Import Attestation from step 64.
98. Attach the Dossier, with all amendments and strikethroughs, by number.
99. Attach the Benchmark Quorum minutes, if any.
100. Attach the notarised screenshots, if any.
101. Attach the 24-hour deployment record from step 80 and the `explain` outputs from step 81.
102. Schedule the **Reading of the Diff** with at least one Keeper and your Sceptic, at a time when all parties have had coffee.
103. Perform the Reading of the Diff: the contributor reads the complete diff aloud, file by file, hunk by hunk, including context lines. The Keeper may stop the Reading at any point to ask "why?". Readings of more than ninety minutes are adjourned and resumed on another day. The record of the Reading is attached to the pull request.
104. If the Reading surfaces a change, return to step 86. Readings that surface nothing are rare and are noted in the release notes by name.
105. Compose the **Commit Message Haiku**: a three-line summary of the change in 5-7-5 syllables, to be included in the pull request description. The Haiku is optional. No pull request without one has ever been merged.
106. Your Sponsors post their final endorsement, in the words "I have read all of it". The Keepers use no other words for this.
107. Your Sceptic posts their final verdict from step 95, unchanged.
108. A Keeper posts "Noted" on the Intent, closing it as Ready.
109. Observe a 48-hour **Silence Period**. Nobody, including you, discusses the change in any channel. If the Silence is broken, it restarts. The Keepers have found that the Silence is where most contributors understand, for the first time, what they have built.
110. At the end of the Silence, reread the Philosophy one final time.
111. Confirm that you still wish to proceed. The Keepers report that a meaningful fraction of contributors, at this step, do not, and that the project is grateful to them in a way it finds difficult to express.
112. Open the pull request. Paste the template from step 97. Request review from your Sponsors, your Sceptic, and one Keeper. You are now a Contributor, and you have our thanks.

## After step 112

Review is brief. Everything that could be discussed has been. The Keeper confirms that the pull request matches the Dossier, that the Reading of the Diff record is attached, and that the Haiku scans. Merge requires unanimous consent of the Keepers, which is usually given within a day, followed by a 48-hour Silence Period, after which the change is in the default branch and awaits [release governance](https://fendlemark.develops.guru/governance/#releases).

Median time from step 1 to merge is nine weeks. The Keepers note that this is faster than most organisations take to decide whether to *consider* a change, and that Fendlemark has never, in its history, reverted one.

## Frequently asked questions

### Is there an abbreviated path for small changes?

No. See [FDR-047](https://fendlemark.develops.guru/governance/#decisions). The Keepers have noticed that the people who ask this question are usually proposing a change that is not small, and the people whose changes are small do not mind.

### What if I only want to fix a typo in the documentation?

Begin at step 1. The documentation is part of the product. The typo has been there, by definition, for as long as the page has, and a few more weeks will not hurt it. The Keepers also point out that documentation typos are vanishingly rare in Fendlemark, and that they would know, because they have performed the Reading of the Diff on every page.

### Can I skip the Haiku?

Yes. See step 105.

### Why 112?

A proposal for 100 steps was found to be "round, and therefore suspicious". A proposal for 120 was found to be "padding". 112 is where the Protocol landed when every step that could be removed had been, and the Keepers regard it, like port 7411, as something that arrived at its value honestly and need not be revisited.

### Has anyone completed FCP-1 and had their pull request rejected?

No. By step 112 there is nothing left to reject. This is the point.
