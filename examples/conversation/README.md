# Native conversation demo

This tool-free host connects the public Zamu session to MISTT native capture/playback,
authenticated MISTT recognition, persistent Narrate Anna speech, and one explicitly
selected OpenRouter model. macOS microphone and speech-recognition permissions are
required. Build the `mistt-media` executable from MISTT's repository-root Swift package
(`swift build --product mistt-media -j 2`) and place it on PATH, or pass `--media /absolute/path/to/mistt-media`.

Start the authenticated MISTT conversation service and Narrate Pocket service using
their repository deployment instructions first. Narrate must have the pinned Anna
preset and model assets. This demo does not install models or start remote workloads.
Non-loopback MISTT requires verified TLS (`--mistt-ca-file` accepts a private CA);
Narrate requires HTTPS except for an explicitly configured loopback tunnel.

Store each service token in its own regular file with mode 0600. The demo reads only
the three files supplied below; it does not discover credentials from other apps.

```sh
go run ./examples/conversation \
  --mistt-address localhost:50051 \
  --mistt-token-file /private/path/mistt-token \
  --narrate-origin http://127.0.0.1:8091 \
  --narrate-token-file /private/path/narrate-token \
  --openrouter-token-file /private/path/openrouter-token \
  --model YOUR_EXPLICIT_OPENROUTER_MODEL \
  --wake
```

`--wake` arms local “Hey Zati” detection. No recognition stream opens before wake or
an explicit `start`. Without `--wake` or `--start`, capture stays off until a command.
Committed text and conversation history go to OpenRouter; speech audio goes to your
configured local speech services. Generated text never invokes tools. Event metadata
and reported usage are printed; `--show-text` additionally prints conversation text.

Commands:

- `start`: explicitly admit a conversation, including fresh admission after a timeout
  or recognition gap. Discarded incomplete speech is never replayed.
- `mute`: stop microphone capture; current playback may continue.
- `unmute`: join the old recognition stream, create a fresh authenticated identity,
  reset capture state, and restart the microphone while preserving playback.
- `wake-on` / `wake-off`: enable or disable wake detection before the first admission.
  After a conversation has started, use `start` to readmit or relaunch with `--wake`.
- `stop`, `quit`, Ctrl-C, or stdin EOF: close the entire demo and its native process.

A service, transport, permission, or native-process failure closes the session. There
is no cloud speech fallback or automatic replay. Quiet checkpoints come from native
VAD after frame classification; the demo does not guess recognition coverage from a
timer. Spoken interruption and playback receipts use the same public adapters as the
Zati integration.

Validation: targeted tests exercise recognition reset/join ordering, admission kind,
service failure, private credential files, and transport policy. These tests use a
fake recognition stream; they do not claim microphone, speaker echo cancellation,
wake accuracy, or full hardware acceptance. Narrate's separate qualification receipt
records actual pinned Anna service synthesis on Spark CPU.
