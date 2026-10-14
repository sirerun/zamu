# Zamu delivery plan — Phase 1 natural voice

Status: Implementation-ready task decomposition; runtime and contract preflight remain prerequisites. All implementation tasks are pending. This document is the maintained delivery source; `docs/rfc/rfc-001.md` owns architecture. `~/Desktop/Zamu-Complete-Plan.md` is a regenerated combined snapshot.

## Accepted scope

Phase 1: English conversation in Zati Mac through speakers, local MISTT STT and Narrate/Pocket Anna TTS on Spark, a separately configurable low-cost OpenRouter reasoning model, optional “Hey Zati” activation, explicit start/mute/stop, thinking-pause tolerance, reversible backchannel pauses, and a standalone reusable demo. Interruptions stop speech; tasks continue unless the user's words change or cancel them. Spark failure pauses voice while text remains available. Enhance existing libraries compatibly. No avatar implementation in Phase 1; E9 and B7–B8 are retained later-phase backlog, pending separate owner scope. E8/B6 are hosted speech, not the required OpenRouter reasoning route.

## Source and qualification

See RFC section 15 for inspected source revisions and concrete entrypoints. Inspection found a stale primary MISTT checkout, paired-only capture, no recognition completeness watermark, Zati's destructive interruption/fixed silence/busy-drop path, and Narrate's batch Pocket pipeline. Anna was explicitly selected for the owner's Zati videos. Native capture ownership and AEC reference transport must be designed together.

Executor: direct engineering in isolated repository worktrees, inheriting the active session selection. No remote execution route or runtime lifecycle is admitted by this plan. Task owners are responsibility roles, not assigned workers. Estimates remain TBD until preflight. Batches retain at most three coherent implementation members per session by default; larger scopes split along repository/test boundaries while preserving IDs and gates. Each changed repository gets independent exact-revision review and its own landing evidence.

Ordinary source checks: Zamu Go formatting, unit/contract tests and race tests when implemented; MISTT existing Go/gRPC tests and `desktop/Scripts/test.sh` plus native build under its build lease; Narrate Go tests and legacy CLI/paragraph regression checks; Zati authored spec regeneration/check, Go host tests, Flutter voice tests, and native Mac audio scenarios. Confirm exact current commands in T1.1. Hardware, model, speaker, and owner listening checks cannot be replaced by mocks. No heavy builds, model calls, installation or application implementation ran while authoring this plan.

## Use-case coverage

| ID | Outcome | Implementation and acceptance |
| --- | --- | --- |
| ZV-01 | Start by Hey Zati or click; mute/stop safely | T1.4, T3.3, T6.4, T7.4 |
| ZV-02 | Natural English Anna conversation over speakers | T3.1–T4.2, T5.1–T6.3, T7.1, T7.3 |
| ZV-03 | Thinking pauses and backchannels; clear interruption | T2.1–T2.3, T6.2, T7.4 |
| ZV-04 | Governed, separately priced voice reasoning | T5.4, T6.1, T7.2 |
| ZV-05 | Safe transcript completeness and recovery | T2.1, T2.3, T3.2, T7.2 |
| ZV-06 | Reuse outside Zati | T2.4, T19.1–T19.4 |

## Reconciliation of the previous Desktop plan

All 63 original task IDs, pending statuses and role owners are retained. Owner decisions change E5 from mandatory local LLM to local speech plus OpenRouter; add wake/tolerance requirements; choose Anna; and explicitly defer avatar work. New IDs: T1.4, T1.5, T2.4, T3.3, T5.4, T6.4, T7.4 and T19.1–T19.4. Existing batch gates expand to cover their new members; demo gets B9. The 74 current tasks remain pending. Source snapshot and exact original task rows are preserved in `docs/plans/archive/zamu-complete-plan-original.md` as non-authoritative history. No prior content approval applies to this revision.

## Delivery checklist

All rows are pending. Dependencies indicate required outcomes. Cross-repository adapter development and candidate integration may use an explicitly pinned upstream candidate or stable fixture; this does not establish landed integration acceptance. A multi-repository batch reviews and lands each repository separately in upstream-first order, and release waits until all referenced dependencies are landed and reverified. Candidate pins must be replaced with landed versions before release.

### E1 — Contract and design freeze

fidelity: executable

- [ ] T1.1 Pin source revisions and reconcile interfaces  Owner: Integration owner  Est: TBD  kind: agent  stage: preflight  blocked-by: []  acc: [MISTT, Narrate, Zati and Zamu revisions are recorded; actual public APIs and current native behavior are inspected; the RFC source baseline is reconciled; existing paired capture, hosted speech and Narrate batch compatibility tests are identified]
- [ ] T1.2 Freeze conversation and media contracts  Owner: Integration owner  Est: TBD  kind: agent  stage: author  blocked-by: [T1.1, T1.4, T1.5]  acc: [Listener, turn, agent, synthesis, wake and player contracts define ordering, finalized coverage, provisional pause tokens, generations, timing, queue limits, errors and cancellation; future avatar consumers only reuse playback events]
- [ ] T1.3 Agree friendly persona and voice craft handoff  Owner: Voice craft owner  Est: TBD  kind: agent  stage: author  blocked-by: [T1.1]  acc: [Parallel prompt work supplies a versioned style contract and sample exchanges; Zati remains the only persona authority]

- [ ] T1.4 Qualify wake, urgent-stop and tolerant detector candidates  Owner: Audio owner  Est: TBD  kind: agent  stage: preflight  blocked-by: [T1.1]  acc: [Pin licensed Mac-compatible sherpa-onnx English keyword-spotter candidate, Hey Zati tokenization and local stop-word path; record bounded backchannel classification design and fixture thresholds; failure requires a replacement candidate before contract freeze]

- [ ] T1.5 Qualify speech feasibility and Anna asset identity  Owner: Audio owner  Est: TBD  kind: agent  stage: preflight  blocked-by: [T1.1]  acc: [Record concrete Anna preset/model asset hashes from the production receipt, license and persistent streaming feasibility on ARM64; probe candidate STT finalized-through coverage and conversational latency; reject batch-only assumptions and select a feasible backend before freezing contracts; bounded probes are distinct from full T5.3 acceptance]

### E2 — Reusable Zamu coordinator

fidelity: executable

- [ ] T2.1 Implement sessions, turn assembly and generation fences  Owner: Zamu implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T1.2]  acc: [Deterministic tests demonstrate partial transcript replacement, exactly-once turn submission, candidate endpoint revocation, bounded queues, finalized recognition barrier, gap quarantine, and rejection of stale output]
- [ ] T2.2 Implement interruption and playback receipts  Owner: Zamu implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T2.1]  acc: [Interruption flushes playback and cancels obsolete response work; provisional pauses retain valid generations; backchannels resume from the acknowledged cursor without duplicate audio; clear interruptions or timeout invalidate; played progress and uncertain alignment are represented; task cancellation remains separate]
- [ ] T2.3 Implement capability negotiation and lifecycle recovery  Owner: Zamu implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T2.2]  acc: [Unsupported formats fail explicitly; disconnect, gaps, timeout and close paths terminate boundedly without silently replaying user turns]

- [ ] T2.4 Deliver standalone reusable conversation demo  Owner: Zamu implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T2.3, T3.2, T3.3, T4.2, T5.4, T6.2]  acc: [examples/conversation uses public interfaces and a tool-free OpenRouter host adapter; real wake/start, speech, backchannel resume, interruption and Spark-loss behavior work without Zati internal imports or avatar dependencies]

### E3 — MISTT conversational input

fidelity: executable

- [ ] T3.1 Add explicit microphone-only admission and speech activity events  Owner: MISTT implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T1.2]  acc: [Real subscription exposes ordered speech-start, speech-stop and RecognitionProgress finalized-through coverage; microphone-only mode is explicit; delayed finals and gaps are tested and existing paired behavior remains tested]
- [ ] T3.2 Implement Zamu MISTT adapter  Owner: Zamu implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T3.1, T2.1]  acc: [Adapter handles source filtering, segment revisions, RecognitionProgress finalized-through coverage, gaps and terminal errors through the real gRPC contract; delayed-final tests prevent premature commitment and no second microphone is opened]

- [ ] T3.3 Implement local wake and duplex capture modes  Owner: MISTT implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T3.1, T1.4]  acc: [Single Mac capture owner supports armed-only local keyword detection, Hey Zati span and two-second volatile command buffer, authenticated activation, native activity/stop events and AEC playback reference; armed audio never leaves Mac; incompatible active paired sessions produce explicit busy without hijacking other clients; existing paired clients remain compatible]

### E4 — Narrate conversational synthesis

fidelity: executable

- [ ] T4.1 Add public incremental synthesis API and local backend  Owner: Narrate implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T1.2]  acc: [Stable text chunks produce incremental PCM with format and generation identity; cancellation works; warm service avoids per-turn job and file-transfer startup]
- [ ] T4.2 Integrate voice craft and synthesis adapter  Owner: Narrate implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T4.1, T1.3, T2.2]  acc: [Anna preset/model assets are pinned from the Zati video receipt and voice/style are pinned per response; phrasing stays natural; unsupported prosody is explicit; factual and verbatim speech text remains unchanged]

### E5 — Spark speech and OpenRouter reasoning qualification

fidelity: executable

- [ ] T5.1 Inspect capacity and select bounded model candidates  Owner: Runtime owner  Est: TBD  kind: agent  stage: preflight  blocked-by: [T1.1]  acc: [Current Spark availability, competing workloads, model licenses, ARM64 runtime support and resource limits are recorded without displacing unrelated workloads]
- [ ] T5.2 Prepare reproducible local runtime manifests  Owner: Runtime owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T5.1, T4.1]  acc: [Pinned local STT and Anna TTS configurations declare health, auth, warmup, limits, lifecycle and rollback using the approved workload mechanism]
- [ ] T5.3 Qualify simultaneous local inference  Owner: Runtime owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T5.2, T5.4, T3.1]  acc: [Authorized bounded qualification measures cold and warm latency with local STT/TTS active alongside the configured OpenRouter route; exact revisions, resources and failures are recorded]

- [ ] T5.4 Select and configure the voice reasoning model  Owner: Zati implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T1.2]  acc: [Separate voice.reasoning_model selects the ordinary governed OpenRouter route, preserves worker/chat profiles, exposes cost and token limits, and pins a measured low-cost model/provider with tool fidelity; no speech credentials are misused as LLM configuration]

### E6 — First Zati voice integration

fidelity: executable

- [ ] T6.1 Integrate host agent and conversation adapter  Owner: Zati implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T2.3, T3.2, T4.2, T5.4]  acc: [Voice turns use the existing governed message/reply stream and separate reasoning profile without changing worker models; busy utterances are queued or explicitly rejected; local speech adapters preserve admission, credentials, accounting, approvals and provisional semantics; authored Zati specs are regenerated]
- [ ] T6.2 Integrate native playback and immediate interruption  Owner: Zati implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T6.1]  acc: [Native streaming PCM playback acknowledges rendered progress and pause/resume/flush; MISTT is the sole capture owner with a verified native playback echo reference; local speech pauses sound without waiting for the controller; destructive cancellation follows confirmed interruption]
- [ ] T6.3 Qualify real microphone-to-speaker conversation  Owner: Acceptance owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T6.2, T6.4, T5.3]  acc: [Installed candidate completes a local-speech/OpenRouter conversation and repeated mid-response interruptions with no stale audio; source and installed revision are identified]

- [ ] T6.4 Wire wake activation and session controls in Zati Mac  Owner: Zati implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T6.2, T3.3]  acc: [Hey Zati and click admit the selected actor/conversation; mute disables wake and recognition; stop returns to armed/off; lock/logout require explicit rearming; focus loss does not stop opted-in listening; inline commands, idle timeout, permission loss and Spark unavailable states match RFC and text chat stays usable]

### E7 — Personal daily-use acceptance

fidelity: executable

- [ ] T7.1 Qualify turn detection and speakerphone echo handling  Owner: Audio owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T6.3]  acc: [Recorded pause, noise and unfinished-sentence cases meet agreed cutoff thresholds; Mac speakers pass echo, wake and interruption acceptance; headphone results are diagnostic only]
- [ ] T7.2 Qualify endurance, privacy and recovery  Owner: Acceptance owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T6.3]  acc: [Two-hour soak covers rollover, interruption and model loss; buffers remain bounded; STT/TTS external egress is blocked while the explicit OpenRouter reasoning route works; blocking that route fails honestly; recovery creates no duplicate turn or action]
- [ ] T7.3 Accept friendly conversational delivery  Owner: Owner  Est: TBD  kind: human  stage: verify  blocked-by: [T1.3, T7.1, T7.4]  acc: [Owner hears and accepts greetings, interruptions, disagreement, errors and tool-progress examples for warmth, pace, clarity and persona consistency]

- [ ] T7.4 Qualify wake and backchannel tolerance  Owner: Audio owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T6.3]  acc: [RFC fixture counts and thresholds pass for Hey Zati, assistant self-wake, 1–3 second thinking pauses, backchannels, clear stops, ambiguous timeout, yes-as-approval and mixed acknowledgement-plus-command input; traces prove no repeated audio or accidental turn/tool submission]

### E8 — Later option: hosted speech adapters (outside Phase 1)

fidelity: executable

- [ ] T8.1 Add explicitly selected OpenRouter speech profiles  Owner: Adapter implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T4.2, T6.3]  acc: [Hosted configuration declares capabilities and credentials; utterance transcription is not called continuous recognition; local speech mode cannot silently fall back]
- [ ] T8.2 Qualify hosted interruption and provider uncertainty  Owner: Acceptance owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T8.1]  acc: [Authorized provider checks report actual latency and cancellation behavior; uncertain execution is not blindly retried or reported as free]

### E9 — Future talking Zati avatar (outside Phase 1; retained backlog)

fidelity: executable

- [ ] T9.1 Choose avatar style and renderer within resource budget  Owner: Avatar owner  Est: TBD  kind: agent  stage: author  blocked-by: [T7.3]  acc: [Owner accepts a visual direction and renderer choice; identity, reduced-motion behavior and voice-only fallback are specified]
- [ ] T9.2 Implement stateful avatar presentation  Owner: Avatar implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T9.1, T6.2]  acc: [Listening, thinking and speaking reflect actual state; failure does not block audio; state animation is not represented as lip sync]
- [ ] T9.3 Implement audio-aligned mouth movement  Owner: Avatar implementer  Est: TBD  kind: agent  stage: implement  blocked-by: [T9.2]  acc: [Alignment or audio-to-viseme cues follow playback sample position and generation; flush, pause and resume preserve synchronization]
- [ ] T9.4 Qualify avatar realism and interruption  Owner: Owner  Est: TBD  kind: human  stage: verify  blocked-by: [T9.3]  acc: [Owner watches and hears actual output; lip alignment, immediate listening transition, stale-cue rejection, resource contention and fallback pass recorded acceptance]

## Review and delivery gates for coding batches

These gates are obligations for future code delivery, not Git stages for this Desktop document. Each changed repository gets its own exact-head review and landing receipt, even when a batch spans repositories. Implementation members per batch are bounded by ownership and accepted contracts. A single reviewer may cover a coherent batch only if every changed revision is identified. Review findings require stable fix, affected-verification and re-review rows before merging.

### B1 — Zamu core

Scope: T2.1, T2.2, T2.3.

- [ ] T11.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T2.1, T2.2, T2.3]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T11.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T11.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T11.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T11.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T11.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T11.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B2 — MISTT and input adapter

Scope: T3.1, T3.2, T3.3.

- [ ] T12.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T3.1, T3.2, T3.3]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T12.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T12.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T12.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T12.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T12.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T12.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B3 — Narrate and craft

Scope: T4.1, T4.2.

- [ ] T13.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T4.1, T4.2]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T13.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T13.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T13.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T13.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T13.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T13.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B4 — Spark manifests

Scope: T5.2.

- [ ] T14.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T5.2, T5.4, T3.1]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T14.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T14.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T14.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T14.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T14.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T14.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B5 — Zati integration

Scope: T5.4, T6.1, T6.2, T6.4.

- [ ] T15.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T5.4, T6.1, T6.2, T6.4]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T15.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T15.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T15.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T15.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T15.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T15.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B6 — Hosted adapters

Scope: T8.1.

- [ ] T16.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T8.1]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T16.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T16.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T16.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T16.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T16.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T16.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B7 — Avatar states

Scope: T9.2.

- [ ] T17.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T9.2]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T17.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T17.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T17.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T17.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T17.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T17.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B8 — Avatar alignment

Scope: T9.3.

- [ ] T18.1 Verify candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T9.3]  acc: [Changed-behavior tests, formatting, race checks where relevant and required repository gates pass on each exact candidate revision]
- [ ] T18.2 Independent code review  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T18.1]  acc: [Independent reviewer records base and head SHAs, covered scope and findings; all blocking findings are resolved and re-reviewed]
- [ ] T18.3 Merge reviewed candidates  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T18.2]  acc: [Only exact approved heads merge under repository policy and required checks; landed SHAs are recorded]
- [ ] T18.4 Verify landed revisions  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T18.3]  acc: [Target branches contain the reviewed changes and focused checks pass on the landed revisions]

### B9 — Standalone demo

Scope: T2.4.

- [ ] T19.1 Verify demo candidate  Owner: Integration owner  Est: TBD  kind: agent  stage: verify  blocked-by: [T2.4, T5.3]  acc: [Public API build plus real Mac/Spark/OpenRouter demo scenarios pass; exact candidate and provider cost recorded]
- [ ] T19.2 Independently review demo  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T19.1]  acc: [Independent reviewer records base/head SHA, T2.4 coverage and resolved findings]
- [ ] T19.3 Merge reviewed demo  Owner: Integration owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T19.2]  acc: [Exact approved head merges under required repository checks; landed SHA recorded]
- [ ] T19.4 Verify landed demo  Owner: Integration owner  Est: TBD  kind: agent  stage: verify-landed  blocked-by: [T19.3]  acc: [Landed public example builds and focused scenarios pass]

## Release, installation and operation

These steps remain subject to explicit release/deployment scope and existing host policy. Writing this plan does not start them. Native installation uses existing approved signing identities and preserves credentials and configuration. Optional hosted and avatar releases do not block the first local voice release.

- [ ] T20.1 Package local voice release candidates  Owner: Release owner  Est: TBD  kind: agent  stage: implement  blocked-by: [T11.4, T12.4, T13.4, T14.4, T15.4, T19.4]  acc: [Pinned library versions and reproducible application/runtime artifacts identify all landed revisions and include migration and rollback instructions]
- [ ] T20.2 Independently review release packaging  Owner: Independent reviewer  Est: TBD  kind: agent  stage: review  blocked-by: [T20.1]  acc: [Exact packaging diff and artifact provenance reviewed; any code changes pass required checks and resolve blocking findings]
- [ ] T20.3 Land and verify packaging changes  Owner: Release owner  Est: TBD  kind: agent  stage: merge  blocked-by: [T20.2]  acc: [Reviewed packaging revisions are merged under repository policy and target reachability and artifact correspondence are verified]
- [ ] T20.4 Install authorized local candidate  Owner: Release owner  Est: TBD  kind: agent  stage: deploy  blocked-by: [T20.3]  acc: [Explicit installation authority recorded; correct native binary and Spark services activated with preserved settings and tested rollback]  blocked: Requires installation authority and qualified deployment route
- [ ] T20.5 Accept installed daily-use release  Owner: Owner  Est: TBD  kind: human  stage: verify  blocked-by: [T20.4, T7.1, T7.2, T7.3, T7.4]  acc: [Installed exact revision passes real conversation, Hey Zati, backchannel resume, interruption, speech locality, recovery and owner listening acceptance]
- [ ] T20.6 Complete operational handoff  Owner: Operations owner  Est: TBD  kind: agent  stage: author  blocked-by: [T20.5]  acc: [Runbook explains wake opt-in/off, start, stop, mute, privacy boundaries, OpenRouter costs, health, model versions, latency diagnostics, retention, resource limits, renewal and rollback without secrets]

## Milestones

| Milestone | Exit evidence |
| --- | --- |
| M1: Contracts agreed | E1 completed; interfaces and persona handoff recorded |
| M2: Reusable voice components | B1–B3 landed with contract and interruption tests |
| M3: Zati speaks with local STT/TTS and OpenRouter | E5 and E6 real audio acceptance; B4–B5 landed |
| M4: Phase 1 daily personal use | T20.5 and T20.6; measured latency and endurance, accepted friendly craft |
| M5: Later talking avatar (deferred) | E9 accepted on actual audio/video; B7–B8 landed; separately authorized install |

No calendar estimates are committed before runtime and contract preflight. Hosted speech and fully local reasoning are later options; OpenRouter reasoning is required in Phase 1.

## Main risks and mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Existing MISTT recognition emits too slowly | Conversation feels delayed | Measure actual first partial and final latency before selecting the backend; keep recognition adapter replaceable |
| Spark contention | Delayed answers or audio underruns | Qualify local STT and TTS together with representative Spark contention; reserve speech capacity; cap hosted context/tokens and report LLM cost |
| Silence detector cuts off thinking pauses | User feels talked over | Provisional endpoints, configurable timing, semantic detector qualification and owner speech samples |
| Assistant hears itself | Repeated self-interruption | Speaker acceptance is a Phase 1 release gate; retain native echo reference under single-owner MISTT capture |
| Concurrent persona work diverges | Inconsistent Zati personality | Versioned shared style contract; no secondary voice persona prompt |
| Tool continues after spoken interruption | User assumes work stopped | Distinct playback and task cancellation; grounded host progress reporting |
| Avatar timing follows generation rather than playback | Unconvincing lip sync | Sample-clock synchronization, flush acknowledgement and generation fences |
| Unavailable alignment | Overstated heard text or mouth precision | Declare phrase-level uncertainty; qualify audio-to-viseme adapter separately |
| Cloud fallback leaks local conversation | Privacy expectation violated | Local speech profile, service-scoped egress checks, explicit OpenRouter text disclosure, no paid speech fallback |

## Handoff and evidence ledger

For every task completion record task ID, source revision, check command or procedure, result, limitations and evidence location. Keep unit tests, provider checks, Spark execution, installed binary identity and owner-visible acceptance separate. Do not mark a task complete because an interface exists or a model is listed in a catalog.

Before implementation, reconcile current branches and ongoing work in MISTT, Narrate and Zati. Preserve the separate friendly-prompt session's changes. This document does not assign that session new work or send it a message.

The companion RFC is `docs/rfc/rfc-001.md`; it is included in full in the Desktop snapshot. Reconcile the snapshot with repository sources before executing any task. Model choices and measured performance remain open. No implementation work has been performed as part of writing this plan.

## Historical document review record (prior Desktop draft only)

Independent content review approved this standalone planning draft after two corrections: explicit recognition finalized-through coverage in the MISTT tasks, and pinned candidate integration with upstream-first landing to avoid implicit cross-repository dependency cycles. No remaining content blockers were reported. Review covered requirement coverage, architecture consistency and delivery sequencing, not source compatibility or runtime performance. Local checks confirmed 63 unique task IDs, resolved dependencies, no dependency cycles, required task metadata and balanced code fences. No portable Wazi conformance or execution admission claimed.

## Current document validation and review

Independent content review identified three issues: backchannel suppression could consume the start of a command; a legacy table still required a local LLM; and speech feasibility/Anna identification did not precede contract freeze. All three were corrected and independently re-reviewed; the reviewer found no remaining substantive blockers after an explicit standalone-backchannel wording clarification, which is applied. Previous Desktop approval remains historical only.

The full Wazi reader/schema/semantic gate was attempted with the installed Wazi checkout and failed because `wazi-contract` is unavailable. This document is a checked engineering plan draft, not an admitted executable lifecycle or a portable-conformance claim. Local checks passed: 74 unique pending tasks, all 63 original IDs and role owners preserved, all dependencies resolved, no dependency cycles, required task metadata and balanced Markdown fences. These checks are separate from the blocked conformance gate. Runtime, installation and human acceptance remain future work.
