# Zamu

Zamu is an interruptible voice-conversation coordinator for Go hosts. It assembles
finalized microphone transcripts into turns and streams stable response phrases
through speech synthesis and acknowledged playback. Hosts retain their own chat
history, permissions, tools, accounting and persona.

Phase 1 uses MISTT microphone recognition, Narrate's persistent local Pocket Anna
service, and one native Mac media owner for capture, echo processing, local
“Hey Zati” detection and playback. Zati supplies its governed reasoning route.
The [standalone demo](examples/conversation/README.md) supplies a tool-free
OpenRouter host.

## Integration

Construct a `Session` with explicit identity and admission binding, a `Listener`,
`Agent`, `Synthesizer`, and `Player`. Construction opens no devices or network
connections. Run it with a cancellable context and continuously consume `Events()`;
a stalled event consumer fails explicitly. `Close(ctx)` waits for bounded cleanup
and returns cleanup failures. `Interrupt(ctx)` stops speech production/playback;
it does not cancel host tasks.

See [the RFC](docs/rfc/rfc-001.md) for behavior and
[the delivery plan](docs/plan.md) for implementation and qualification evidence.

The adapter packages expose:

- `adapters/mistt`: authenticated microphone-only gRPC, finalized capture coverage,
  and ordered native controls. The host supplies already captured PCM.
- `adapters/narrate`: stable phrase input and bounded 24 kHz PCM output from the
  persistent Anna service. No per-turn jobs or model downloads.
- `adapters/native`: bounded JSONL transport to MISTT's `mistt-media` executable.
- `adapters/openrouter`: explicit model and credentials, streaming plain speech,
  bounded history and usage reporting, without tool execution. Zati uses its own
  governed host instead.

## Contract details

Capture is 16 kHz mono PCM16. Each fresh recognition stream has its own identity,
sequence and zero-based capture timeline. Speech activity cannot certify that
recognition is complete: only a final coverage watermark permits turn submission.
Transcript gaps require fresh admission and a repeat, never reconstructed words.

Mute affects microphone input. Unmute rotates the recognition stream and capture
baseline under the same admission. Its first event is `CaptureReset` with the same
binding and a fresh stream identity; the prior session must be active and muted.
This discards pre-mute partial text while preserving the active playback generation.
Ordinary `Activated` starts a new admission and invalidates prior output.

Audio generations, frame sequences, offsets and formats are checked independently.
Phrase boundaries are synthesis markers, not PCM or proof of words heard.
Playback receipts report rendered sample progress. Only aligned evidence can
establish heard text. Acknowledged provisional pauses can resume the same
generation and cursor; confirmed interruptions invalidate it.

At most four host response calls may remain outstanding, including canceled calls
that have not returned. Further turns produce an explicit `turn_rejected` event.
Hosts must promptly admit or explicitly reject each `Turn.ID`, deduplicate it, and
keep already admitted tasks independent from response-production cancellation.
They must also reject canceled or superseded calls before replacing current reply
routing. This library does not automatically replay a failed turn.

## Verification

```sh
go test -race ./...
go vet ./...
```

Tests cover deterministic endpoint and interruption races, real adapter wire
contracts, local process protocol handling, cancellation and stream failures.
Hardware wake accuracy, speaker echo performance, long-session latency and owner
voice acceptance require the separate qualification procedure. Passing these tests
does not establish those results.

Private upstream source dependencies are included under [third_party](third_party/README.md)
with licenses and exact revision receipts. Build and test a fresh checkout with
`GOWORK=off go test ./...`; no sibling checkout or private dependency token is needed.
