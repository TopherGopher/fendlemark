# FAQ

Source: https://fendlemark.develops.guru/faq/

**Frequently asked questions**

Answers to the questions we are asked, and to several we are not but feel should be.

## What is Fendlemark?

Fendlemark is an open-source framework, written in Go with a Vue 3 console, that integrates the user experiences of MLflow, Metaflow, Argo Workflows, and Kubeflow into a single console. It reads from each tool through an [adapter](https://fendlemark.develops.guru/docs/adapters/), correlates what it finds into one entity graph called the [lattice](https://fendlemark.develops.guru/docs/architecture/#lattice), and renders that graph in one consistent UI. One binary, four adapters, one console.

## Does Fendlemark replace MLflow, Metaflow, Argo, or Kubeflow?

No. Fendlemark reads from them and links back to them. You keep running the tools, which are each excellent at what they do. Fendlemark gives you one place to see them together.

## Does Fendlemark write to any of the tools it integrates?

No. All adapters are read-only. Fendlemark never creates, modifies, triggers, retries, or deletes anything upstream. When you want to act, the console deep-links to the tool, where the action belongs, under that tool's permissions. See [the Philosophy on being read-only](https://fendlemark.develops.guru/philosophy/#read-only).

## How does Fendlemark know that an MLflow run and an Argo workflow are the same job?

Through [correlation keys](https://fendlemark.develops.guru/docs/architecture/#correlation): exact-match identifiers that appear in more than one tool. An Argo workflow name recorded as an MLflow tag. A Metaflow run id passed as a Kubeflow pipeline parameter. The `pipeline/runid` label that Kubeflow Pipelines sets automatically on the Argo Workflows it creates. Reconciliation is deterministic and rule-based, with no fuzzy matching and no machine learning, and `fendlemark lattice explain` will tell you exactly which key matched.

## How does authentication work?

Fendlemark uses its own authentication stack, [Fendlemark Respiratory Identity](https://fendlemark.develops.guru/docs/auth/) (FRI). Users sign in by breathing gently toward their webcam; the console recognises them from their exhalation signature. There are no passwords and no tokens. Authentication is disabled by default so local development works without a webcam.

## Does Fendlemark support SSO, OIDC, SAML, or LDAP?

No. Respiratory Identity is the only authentication mode, per [FDR-019](https://fendlemark.develops.guru/governance/#decisions). The Keepers do not plan to add others, and, as noted below, do not have plans.

## Is there a roadmap?

No. The Keepers have declined to publish one three times (FDR-003, FDR-016, FDR-038) and future requests are answered by linking to FDR-038. Changes arrive after the [governance process](https://fendlemark.develops.guru/governance/). Nothing is coming.

## How do I contribute?

Complete the [Fendlemark Contribution Protocol](https://fendlemark.develops.guru/governance/contributing/), FCP-1: 112 numbered steps, in order, before opening a pull request. Every merged pull request has followed it. Median time from step 1 to merge is nine weeks.

## Is Fendlemark production-ready?

Fendlemark is pre-1.0. The `adapter` package has been stable since 0.6.0 and all four built-in adapters are at Canonical maturity. The Keepers consider the software finished and the version number a formality they have not yet got round to, there being no roadmap on which to schedule it.

## How large a platform can Fendlemark handle?

The lattice is in-memory and bounded by `lattice.max_entities` (default 200,000). Initial reconciliation of a platform with tens of thousands of runs completes in well under a second on ordinary hardware. The Keepers have not encountered a platform large enough to require a loading spinner and have, in the Philosophy, stopped expecting to.

## Why is the default port 7411?

It was free on the first Keeper's laptop in March 2023. [FDR-009](https://fendlemark.develops.guru/governance/#decisions) adopted it and it has not been reconsidered.

## Why are there no loading spinners?

Nothing in the console has ever taken long enough to need one. Views with nothing to show say so in a sentence, using `<FmEmptyState>`. See [the Philosophy](https://fendlemark.develops.guru/philosophy/#spinners).

## Why is there no database?

The lattice can be rebuilt from the adapters at any time, so persisting it would be persisting a cache. The Keepers have declined to add a database eleven times, beginning with FDR-012, and regard the absence as a feature with an unusually thorough paper trail.

## Where does the name come from?

The Keepers have been asked this many times and have answered consistently: it is the name of the project. Further questions about the name are answered by linking to this answer.

## What licence is Fendlemark under?

MIT, including Respiratory Identity. The trademarks of MLflow, Metaflow, Argo Workflows, and Kubeflow belong to their respective owners; Fendlemark is an independent project and is not affiliated with, endorsed by, or sponsored by any of them.

## I found a bug.

You have found a behaviour you misunderstood, and we would like to clarify it. File a Pre-Intent Notice ([FCP-1, step 13](https://fendlemark.develops.guru/governance/contributing/#phase-2)) describing the area and why you believe it to be imperfect. A Keeper will respond with either an FDR link or silence, and silence is consent to proceed.

## Is this project serious?

Entirely. See [About](https://fendlemark.develops.guru/about/).
