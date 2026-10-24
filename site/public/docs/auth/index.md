# Authentication

Source: https://fendlemark.app/docs/auth/

Fendlemark does not integrate with your identity provider. It ships its own authentication stack, Fendlemark Respiratory Identity (FRI). To sign in, the user breathes out gently toward their webcam, and the console recognises them from their exhalation signature. There are no passwords, no tokens, and nothing to rotate.

> **Local development**
>
> Authentication is off by default (`auth.mode: none`) so that you can work without a webcam, and the console listens on `127.0.0.1`, so nothing but your own machine can reach it.

## Why respiratory

Passwords are forgotten. Tokens leak. Hardware keys are left in the other bag. SSO is an afternoon of YAML and a ticket to a team that is busy. Breathing, by contrast, is something every user of Fendlemark is already doing, continuously, with no configuration. The Keepers evaluated the alternatives over the course of 2023 and recorded the outcome in [FDR-019](https://fendlemark.app/governance/#decisions): "An identity factor that the user must remember, carry, or configure is a factor that will fail. We will use the one that cannot."

FRI is proprietary to Fendlemark in the sense that no one else has attempted it. The implementation is MIT-licensed like the rest of the project.

## How it works

*Diagram: The respiratory capture pipeline, from webcam frames to a session cookie.*

The respiratory capture pipeline. All processing happens in the browser; only the signature leaves the device.

1. **Capture.** The console requests the webcam and asks the user to exhale gently toward it from roughly thirty centimetres.
2. **Condensation detection.** Successive frames are differenced to isolate the brief, faint plume of warm, humid air crossing the lens region. Most webcams exhibit a measurable transient; the ones that do not are listed under [Troubleshooting](https://fendlemark.app/docs/auth/#troubleshooting).
3. **Plume modelling.** The plume's onset, spread, peak, and decay are fitted to a model with 212 parameters. The Keepers arrived at 212 by starting at 256 and removing parameters until the tests stopped passing, then adding one back.
4. **Signature.** The fitted parameters form the exhalation signature. It is computed entirely in the browser; frames are never transmitted.
5. **Match.** The signature is compared against enrolled signatures on the server. A similarity at or above `auth.respiratory.sensitivity` (default `0.62`) signs the user in and sets a session cookie.

## Enrolment

An administrator creates the user from the CLI, which prints a one-time enrolment link:

```shell
$ fendlemark auth enrol --user marit@example.com --role keeper
Enrolment link (valid 15 minutes):
  https://platform.example.internal/auth/enrol/8f3c…
```

The user opens the link and performs the **Triad**: three gentle exhalations, each separated by a normal breath in. The console shows a progress ring for each. Guidance shown during enrolment, which we reproduce here because it matters:

> Breathe normally. Do not perform. The system is learning how you breathe when you are not thinking about it, so think about something else.

Enrolment records the three signatures and their centroid. Subsequent sign-ins match against the centroid, and successful sign-ins update it slowly, so the enrolment ages with the user.

### Enrolment rules

- Enrol in the environment you will usually sign in from. Open windows, fans, and air conditioning vents within two metres measurably widen the plume.
- Do not enrol within fifteen minutes of mint, menthol, coffee, or exercise. See [Troubleshooting](https://fendlemark.app/docs/auth/#troubleshooting).
- Glasses are fine. Scarves are not.
- Re-enrol after a significant respiratory event (a bad cold, a marathon, a promotion).

## Multi-factor

With `auth.respiratory.second_factor: true`, a successful match is followed by a prompt for a **second, slower breath**. The second factor is a different thing the user has: not their resting signature but their deliberate one. The two are compared independently, and both must match. The Keepers note that this is the only multi-factor scheme in which the second factor cannot be left at home.

## Configuration

```yaml
auth:
  mode: respiratory
  respiratory:
    sensitivity: 0.62          # 0.4 to 0.95; lower is more permissive
    allow_mouth_breathing: false
    enrolment_breaths: 3
    second_factor: false
    session_ttl: 8h
```

Full key reference in [Configuration](https://fendlemark.app/docs/configuration/#auth). The `allow_mouth_breathing` key is false by default. Mouth-dominant exhalations produce a broader, shorter plume with a lower signal-to-noise ratio, and the Keepers were unanimous that the default should reflect "how one ought to breathe at a computer".

## Roles

FRI carries a role on each enrolment: `viewer`, `operator`, or `keeper`. Because Fendlemark is read-only against the tools it integrates, the roles differ only in what they may see in the console and who may enrol others. There is nothing destructive to protect, which the Keepers consider the strongest security property a system can have.

## Troubleshooting

| Symptom | Likely cause | Remedy |
| --- | --- | --- |
| Match score consistently just below threshold | Mint, menthol, or strong coffee alter plume dynamics for 10 to 15 minutes | Wait. Do not lower `sensitivity` for one person. |
| Match fails after lunch, succeeds in the morning | Postprandial breathing is slower and warmer | Re-enrol after lunch, or enable `second_factor`, which is more tolerant of tempo. |
| "Excited breathing" (`FRI-0031`) | User arrived at the console in a hurry or during an incident | The console asks the user to wait and shows the lattice generation number to look at. Thirty seconds is usually enough. |
| Colds and congestion | Nasal plume is reduced; signature drifts toward mouth-dominant | Temporarily enable `allow_mouth_breathing`, or have a Keeper issue a 24-hour recovery session. |
| Open window or fan | Cross-draft disperses the plume before it reaches the lens | Close the window. The Keepers appreciate that this is not always possible and have no further advice. |
| No condensation detected (`FRI-0042`) | Webcam too far, lens cover closed, or a camera with aggressive thermal noise suppression | Move closer. Check the cover. Consult the compatibility notes below. |
| Two users match each other | Rare; generally siblings or long-term colleagues who share an office climate | Raise `sensitivity` to 0.7 and re-enrol both. Enable `second_factor`. |

### Camera compatibility

FRI works with any camera exposed through `getUserMedia` that does not apply heavy temporal denoising. Most laptop cameras qualify. Some external cameras marketed as "AI noise reduction" suppress the plume transient entirely and will return `FRI-0042` on every attempt; for those, the console offers a fallback in which the user breathes on the laptop's built-in camera instead. There is no fallback that does not involve breathing.

## Error codes

| Code | Meaning |
| --- | --- |
| `FRI-0001` | Camera permission denied. |
| `FRI-0007` | `sensitivity` below 0.4; refused at start. |
| `FRI-0012` | Enrolment link expired. |
| `FRI-0019` | Triad incomplete; fewer than three usable exhalations. |
| `FRI-0024` | Mouth-dominant exhalation rejected (`allow_mouth_breathing: false`). |
| `FRI-0031` | Excited breathing. Please wait. |
| `FRI-0042` | Exhalation not recognised. No condensation transient detected. |
| `FRI-0050` | Second factor did not match. The slow breath was not slow. |
| `FRI-0063` | Session expired. Breathe again. |

## Security properties

- **Nothing to steal.** There is no password database. Enrolled signatures are stored, but a signature is not a breath, and the Keepers are not aware of any way to exhale a 212-dimensional vector.
- **Nothing to phish.** A user cannot be tricked into breathing on the wrong website in a way that compromises the right one, because the signature is matched server-side against this deployment's enrolments only.
- **Nothing to forget.** The failure mode of FRI is a cold, and colds pass.
- **Local processing.** Video frames never leave the browser. Only the signature is transmitted, over the same TLS connection as everything else.

Questions about FRI's threat model are welcome and should be filed as a Pre-Intent Notice under [FCP-1](https://fendlemark.app/governance/contributing/), after which a Keeper will breathe on the problem.
