# Phase 1 qualification procedure

The automated contract suite and recorded Spark CPU probes are useful evidence;
installed microphone/speaker and human acceptance are separate gates. Do not mark
those gates passed from generated silence, mocked playback, or source inspection.

## Record the candidate

Record Zamu, MISTT, Narrate and Zati source revisions, installed binary identity,
Mac audio devices/OS, local English detector availability, runtime image/model and
Anna asset hashes, Spark resource allocation and competing workloads. Record the
exact OpenRouter reasoning model/profile, provider route, current pricing and
actual usage. Do not record credentials or raw private speech by default.

Run the root Go race suite and the upstream native, synthesis and host checks.
Use each repository's pinned asset preparation and Spark manifest tools. Check
service authentication and health before admitting microphone capture. Keep
unrelated Spark workloads running; CPU speech qualification does not require
reserving their GPU.

## Scripted scenarios

Use an explicitly consented test conversation and non-sensitive phrases.

1. Enable Hey Zati. Confirm visible armed state and that no microphone PCM reaches
   either speech service before activation. Say the wake phrase followed directly
   by a command; verify the command is retained but the wake phrase is excluded.
2. Start by click, mute, unmute, stop, revoke microphone permission, switch the
   selected conversation and close the window. Check there is one capture owner,
   fresh recognition identity after unmute, and no stale command after recovery.
3. Speak the 40 phrases/pause cases in `detector_test.go`. Add natural speech and
   intentional 1–3 second pauses; compare submitted turns with the intended text.
4. While Anna speaks, say a standalone acknowledgement. Verify local pause and
   resumed audio from the acknowledged cursor. Repeat with “yeah, but change the
   date,” an answer to a yes/no question, and “stop.” No instruction may disappear
   as a backchannel, and clear interruption must not replay stale audio.
5. Repeat at normal speaker volume, different distances, and with background
   sound. Measure false interruptions and verify playback echo does not become a
   user turn. Headphone success does not qualify speakers.
6. Interrupt while a governed task is running. Speech stops promptly while the
   task remains governed by ordinary host policy. Explicitly cancel the task by
   instruction and verify the normal host cancellation/approval path.
7. Disconnect Spark, stall recognition, truncate synthesis, overload playback,
   inject a transcript gap, and exhaust the configured reasoning limit. Voice
   must report the failure, text chat must remain available, and no hosted speech
   fallback or automatic replay may occur. Restore services and explicitly start
   fresh.
8. Run a two-hour session with periodic turns, acknowledgements, interruptions,
   mute cycles and idle periods. Record process/queue/resource growth, session
   renewal behavior and bounded shutdown. A virtual-clock expiry test alone does
   not qualify endurance.

## Measurements and verdict

Use capture sample offsets and actual playback receipts. Record speech onset to
local pause/flush, acoustic end to first audible response, final recognition
coverage latency, first PCM, maximum delivery gap, token usage/cost and cleanup
latency. Report the sample count and distribution, not only the best result.
Compare against RFC section 12's thresholds. Heard text requires alignment;
otherwise report rendered samples and uncertain word boundaries.

The owner listens to greeting, disagreement, uncertainty, tool progress, error and
interruption recovery exchanges before approving friendly craft. Record failed,
skipped and unobserved scenarios explicitly. Until those checks run, describe the
result as a source candidate with measured component behavior, not a qualified
daily-use voice release.

## Configure and operate the candidate

Follow Zati's `docs/implementation/local-voice.md` for its protected
`ZATI_LOCAL_VOICE_CONFIG`, separate governed reasoning profile and source
snapshot provenance. Choose English and the pinned Anna assets. Keep STT/TTS
credential references separate from the OpenRouter reasoning profile. Do not put
tokens in command-line arguments, the RFC, application logs or conversation text.

MISTT's `dgx/conversation/README.md` and Narrate's
`services/pocket/README.md` define the pinned service builds, asset preparation,
health/authentication and rollback. Production images and persistent storage must
exist before enabling voice; the completed ephemeral probes do not establish
that installation. Use the tracked Spark API, retain unrelated workloads, verify
both service health checks, and then explicitly start a fresh Mac voice session.

Wake is opt-in. Start admits the selected conversation; mute disables wake and
capture, and stop ends the active admission. Re-enable explicitly after lock,
logout, permission loss or an unavailable service. Text chat remains available.
For diagnosis retain timing, queue bounds, generation/sequence and error metadata;
raw audio and transcripts are not diagnostic defaults. Roll back the application
and speech manifests together to their recorded compatible revisions.
