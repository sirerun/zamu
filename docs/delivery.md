# Phase 1 delivery receipt

The Phase 1 source candidate implements the coordinator, reusable demo, local
speech services and Mac host integration. Code completion and installed daily-use
acceptance are separate. Hosted speech and avatars remain deferred.

## Source and review

| Component | Reviewed candidate | Delivery |
| --- | --- | --- |
| MISTT | `132460d785e7ffa40735e75a50245637e4466837`, base `4a66e95117cb2ae24fd25731bf717eee1679f0b4` | [PR 27](https://github.com/sirerun/MISTT/pull/27), landed `fef56673e2ac3c8a59a647a5069212081a9ae279`; identical tree and target reachability verified |
| Narrate | `a8ad48879a4960f88810b2d8bb11989dc009c4bc`, base `29bc740031c8b26ca68a2d79e9702d5d91e7bb84` | [PR 3](https://github.com/narrate-it/narrate/pull/3), landed `5bcca2d357038a1b88b0c883dd7eabcb16cbd4c0`; identical tree and target reachability verified |
| Zamu | `0ac91abece62c97c71390a7e364a3e80f2bb98fd`, base `71ddc205e6fe97e76cc0fd55792c789722d11c22` | [PR 2](https://github.com/sirerun/zamu/pull/2); independent final source review clean |
| Zati | `a719d3ba6b5cf2ea3b88cac03abce64be981262a`, base `e46e1a5ded57346c62f87664960c8b517c3aa274` | [PR 26](https://github.com/zati-run/zati/pull/26), draft candidate reviewed clean; self-contained Mac debug build passes; CI/landing pending |

Narrate's implementation owner independently reviewed Zamu and Zati; the Zamu
owner independently reviewed Narrate, MISTT and the public demo. Blocking findings
were fixed and re-reviewed, including Zati failure/disposal shutdown receipts and
source-specific voice profile selection. MISTT and Narrate landed by rebase.

## Executed checks

- Zamu: all six packages pass `GOWORK=off GOPROXY=off GOSUMDB=off GOMAXPROCS=2 go test -p 1 -race -timeout 120s ./...`; vet, build, golangci-lint and actionlint pass. Offline here means dependencies already cached plus the checked-in MISTT source snapshot; no private module fetch is needed.
- MISTT: Go race/vet, legacy desktop and preflight regressions, native conversation tests/CLI build and packaging syntax/tests pass. Actual Whisper CPU decode, coverage, cancellation and recovery probes are recorded upstream.
- Narrate: Go race/vet (109 test/subtest results, one pre-existing Darwin skip), six Python tests and actual Anna streaming/cancellation/recovery probe pass. PR CI build passed.
- Zati: full Mac debug application build, 34 voice test/subtest race cases, selected registry and execution-context cases, two LocalIO cases, 14 Flutter tests, analyzer and controller build pass and after final source-snapshot packaging. All 23 vendored files match upstream source; offline Go 1.26.2 checks and 42 specification-generator tests pass. The final self-contained Mac build ran with private GitHub URL fetch disabled, and all 14 Flutter tests passed again at the final candidate. The broader 237-case execution race suite exceeded 600 seconds and is not counted as passing.

Zamu also passed all six race-tested packages, vet and build in a fresh tracked
Linux/ARM64 container with Go 1.26.2, two CPUs, 3 GiB and no GPU. Its exact source
archive, immutable container digest and output are in
[the runtime receipt](receipts/zamu-linux-0ac91ab.json). The pod was deleted after
verification. This is independent Linux source evidence, not a GitHub CI pass.

## CI availability

Zamu's push run `38517432027` and PR run `38517466943` report
`startup_failure`, synthetic path `BuildFailed`, zero jobs and no logs/check runs.
The exact committed workflow passes actionlint; referenced action revisions exist.
Actions is enabled and allows all actions. The account/backend cause is not known;
no billing or policy setting was changed. No CI success or merge is claimed.
Keep PR 2 open until the repository can execute its checks.

Zati's pre-existing run `38455051476`, check `115495513701`, has an explicit
annotation that recent account payments failed or its spending limit must be
increased. Its job did not start. This establishes Zati's existing billing/spend
availability problem, not the exact cause of Zamu's separate startup failure.
Payment credentials and account spending policy were not changed. The new PR 26
run `38518850264`, check `115703672847`, failed before execution with the same
billing/spending annotation. Its [candidate PR](https://github.com/zati-run/zati/pull/26)
therefore remains draft and unmerged despite passing local source verification.

## Remaining installed-runtime acceptance

The tracked Spark probes were removed after measurements. They did not install
persistent production services. Whisper persistent model storage and service image
activation still need a supported deployment route. Follow MISTT's
`dgx/conversation/README.md` and Narrate's `services/pocket/README.md` for pinned
assets, authenticated health, bounded resources and rollback. Existing unrelated
Spark workloads remain untouched.

No qualified live OpenRouter profile was accessible through Zati's normal product
context. The separate governed profile is implemented, but its model/provider
choice, latency, tool fidelity and actual cost still need measurement. Speech
credentials must not substitute for reasoning credentials.

Run [qualification.md](qualification.md) on the installed Mac candidate for
speaker echo, Hey Zati/self-wake, interruption, natural pauses, backchannels,
recovery, external speech-egress enforcement and two-hour endurance. The owner
must listen to Anna and accept persona/craft. No mocked or source-only result
satisfies those gates. The broad authorization removes a permission blocker; it
does not create missing runtime configuration or substitute for a sleeping user's
listening acceptance.
