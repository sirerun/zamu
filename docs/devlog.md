# Development log

## 2026-10-13: Phase 1 local conversation implementation

**Type:** investigation
**Tags:** voice, recognition, cancellation, native-audio

**Problem:** Existing paired capture and batch synthesis did not supply a reusable,
interruptible conversation path. Zati's old voice flow used fixed silence and
busy-drop behavior.

**Fix:** Added a standalone coordinator and explicit streaming adapters. MISTT
adds microphone admission, finalized recognition coverage and one native duplex
media owner. Narrate adds persistent cancellable Anna PCM. Zati integrates these
through its normal governed message, permission and accounting route.

**Review findings:** Fixed post-commit transcript revisions, a completed-response
backchannel race, stranded pauses, outstanding text limits, cleanup error loss,
and pre-mute transcript isolation. Native silent checkpoints must follow VAD
classification; issuing a checkpoint from an audio callback before onset can
finalize and lose the first speech frame. Zati must skip zero-sample phrase
markers and isolate reply readers and admission by turn sequence.

**Impact:** Existing paired transcription and batch narration remain separate.
Local detector capability, speaker echo behavior and owner listening acceptance
remain explicit qualification gates. Source tests do not establish audio quality.

## 2026-10-13: Persistent Anna CPU qualification

**Type:** benchmark
**Tags:** spark, pocket, anna

**Problem:** Installed Pocket 3.0.2 lacks the cancellable streaming API.

**Fix:** Pin upstream revision b94db177bc0f77517d9b124843e25605d6f9aed8,
verify production Anna/model/tokenizer hashes, stage a fully local configuration,
and reject incompatible runtimes at startup.

**Evidence:** Narrate a8ad488 contains
`services/pocket/qualification/runtime-receipt.json`. On a tracked Spark pod with
1 CPU, 2 GiB and no GPU: first PCM 97/76 ms for two phrases, maximum frame gap
34 ms, cancellation 11 ms, and successful post-cancel synthesis. Probe pods were
removed; unrelated workloads were preserved. Runtime TCP attempts beyond loopback
were denied by the probe's Python guard after preparation; this is not a
system-wide firewall claim.

**Limits:** Two phrases do not establish p95, concurrent recognition, a two-hour
soak, Mac speaker behavior or human acceptance.
