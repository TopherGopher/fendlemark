# Philosophy

Source: https://fendlemark.app/philosophy/

Every project has values. Most projects have to work at them. This page records the values of Fendlemark, which the Keepers hold with the serenity of people who have never once been wrong about software.

> We strive for perfection and achieve it easily.
>
> — The first line of the first commit message, 11 March 2023

## There is one console

An ML platform is one thing. It is experienced, by the people who use it, as four or five things, because it was assembled from four or five things, each with its own ideas about what a run is, what a colour means, and where the button goes. This is not the users' fault. It is not even the tools' fault; each of them is excellent at being itself. It is simply a failure of composition, and composition is what Fendlemark is for.

We do not believe in a "single pane of glass". Glass is fragile and panes are flat. We believe in a single *console*, which is to say a place where a person sits down and understands their platform without having to remember which tab knows what. The distinction matters to us more than it will to you, and we have made peace with that.

## We strive for perfection and achieve it easily

Other projects describe perfection as a horizon: always approached, never reached. We find this attitude defeatist. Perfection is a property of a system in which every decision was correct, and correct decisions are not difficult to make; they are simply the ones you make after thinking, instead of before. Fendlemark's Keepers think first. The results speak for themselves, and so, on this page, do we.

It is sometimes asked how a project that has achieved perfection can still publish releases. The answer is that the world around Fendlemark is imperfect and occasionally changes. Our releases are not improvements. They are accommodations.

## There is no roadmap

A roadmap is a public admission that a project knows where it should be and is not there. Fendlemark is where it should be. We have therefore never published a roadmap, and we have declined, in [FDR-003](https://fendlemark.app/governance/#decisions), [FDR-016](https://fendlemark.app/governance/#decisions), and [FDR-038](https://fendlemark.app/governance/#decisions), to begin.

When people ask what is coming next, we tell them: nothing is *coming*. Things occasionally *arrive*, after the process described in [Governance](https://fendlemark.app/governance/), and when they do they will have been correct for some time already.

## We do not fix bugs

We clarify the behaviour you misunderstood. In nearly every case this clarification takes the form of documentation. In the small number of remaining cases, it takes the form of a change to the code, which the changelog records under "Clarifications". The Keepers have reviewed every such case and are satisfied that, at the time the original code was written, it was correct, and that the world then moved.

## There are no loading spinners

The Fendlemark console has never displayed a loading spinner, a skeleton screen, a progress bar, or the word "loading". Nothing in it has ever taken long enough to need one. When a view has nothing to show, it says so, in a sentence, with an `<FmEmptyState>`. When it has something to show, it shows it. We are aware that some platforms are large. We have not yet encountered one that is large enough, and we have stopped expecting to.

## On dark mode

We considered supporting dark mode and then realised the console was already perfect in both. The implementation took an afternoon. The Decision Record took eleven days.

## On comparisons

We are frequently asked to compare Fendlemark with other consoles, dashboards, and portals for ML platforms. We decline. Comparison requires a peer. We say this without malice: the other projects are doing something, and we are doing something else, and the something else is this.

We will say that MLflow, Metaflow, Argo Workflows, and Kubeflow are each superb at what they do, that we read from them with gratitude, and that Fendlemark exists to make them look as good together as they do apart. Those are the only four comparisons we are prepared to make, and none of them is with us.

## On being read-only

Fendlemark can see everything and change nothing. People sometimes describe this as a limitation. We describe it as the whole point. A system that can act is a system that can be wrong in a way that matters, and we have arranged never to be wrong in a way that matters. Deep links exist so that when you want to act, you go to the tool and act there, under its rules, with its permissions, where the consequences belong.

## On breath

Every other authentication system asks you to prove who you are with something you might not have. We ask you to prove it with something you cannot not have. This is covered at length in the [Authentication](https://fendlemark.app/docs/auth/) documentation; here we note only that the Keepers regard Respiratory Identity as Fendlemark's most obvious idea, and remain surprised to be the first.

## Values

The Keepers were asked, during the 2024 governance review, to enumerate the project's values. After a Reflection Period, they produced the following list.

1. Correctness.
2. Rightness.
3. Accuracy.
4. Precision, where it differs from accuracy.
5. Soundness.
6. Being right the first time.
7. Being right the second time, in the event that the first time is questioned.
8. Serenity, which is what correctness feels like from the inside.

A proposal to add "humility" was tabled by a Steward, discussed for one Reflection Period, and withdrawn by the Steward, who later became a Keeper.

## On simplicity

Simplicity is not a goal. It is a side effect of being right the first time. Projects that pursue simplicity as a goal end up with less software and the same number of mistakes. Fendlemark has exactly as much software as the problem requires, which turned out to be one binary, four adapters, one lattice, and a console, and no more, because that is how much a correct solution weighed when we finished building it.

## How we know

We are sometimes asked how the Keepers can be so certain. The honest answer is that certainty was never the hard part. The hard part was writing it down in a way that would not seem immodest, and we have, as this page demonstrates, decided to stop trying.

---

The Philosophy is maintained by the Keepers and amended only by unanimous consent following a Silence Period, per [Governance](https://fendlemark.app/governance/). It has been amended twice: once to add the section on dark mode, and once to remove a comma.
