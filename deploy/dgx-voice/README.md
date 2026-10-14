# Installed DGX speech runtime, 2026-10-14

The `zati-voice-runtime-20261014-a` Spark pod runs persistent Whisper, Pocket Anna,
and the MISTT gRPC gateway. The existing host controller connects through local
ports 18792 (gRPC) and 18092 (TTS). No GPU or GitHub Actions is used.

Source: MISTT `132460d785e7ffa40735e75a50245637e4466837`, Narrate
`a8ad48879a4960f88810b2d8bb11989dc009c4bc`, Zati controller
`4869f60db7bfd9c7eb3d1a6d40f779e80cb232bb`. MISTT/Narrate source trees match the
landed revisions in the delivery receipt. The controller change avoids opening
runtime voice configuration during descriptor-only catalog assembly. Its catalog
regression and local-voice tests pass locally; the Linux ARM64 build is clean.

## Reproduce

These templates require explicit installation paths; they are not directly
submittable. Render `${VOICE_ROOT}`, `${PREPARED_ROOT}`, `${POCKET_PACKAGES}` and
`${POCKET_HUB}` to dedicated absolute paths. Preserve the immutable NVIDIA ARM64
image digest. Preparation needs 2 CPU / 4 GiB; runtime needs 4 CPU / 6 GiB, no GPU.

1. Populate `VOICE_ROOT/source/mistt` from upstream `dgx/conversation` and
   `VOICE_ROOT/source/narrate` from upstream `services/pocket`. Build upstream
   `cmd/mistt-conversation` for Linux ARM64 and install it in `source`, alongside
   `supervise.py`. Store manifests and source hashes with the deployment.
2. Provision a new, empty `PREPARED_ROOT`. The preparation pod is the sole writer
   to this exact directory. Existing model/package caches mount read-only. Submit
   the rendered preparation manifest to Spark; require `PREPARATION_COMPLETE`.
   This prepares hash-verified Whisper/Anna assets, locked STT dependencies and
   the pinned cancellation-capable Pocket source. Runtime mounts are read-only;
   runtime startup does not install dependencies or download models.
3. Create separate random 32-byte-or-longer tokens in
   `VOICE_ROOT/credentials/{mistt,narrate}/token`, mode 0600, directories 0700.
   Never put values in manifests, arguments, logs or source. The container copies
   them to private owner-only runtime files for its service UID.
4. Install `voice-ingress.nft` and its systemd unit before publishing ports. The
   dedicated nftables prerouting chain blocks non-loopback ingress before DNAT;
   it does not alter unrelated firewall rules. Verify remotely that both ports
   cannot be reached and locally that unauthenticated TTS returns 401.
5. Submit the rendered runtime manifest to Spark. Require authenticated health,
   `VOICE_SERVICES_READY`, actual synthesis/recognition, and zero restarts. Source,
   prepared models/dependencies, existing Pocket dependencies and credentials
   mount read-only. Prepared dependencies take priority over the legacy cache.
6. For trusted local credential provisioning, copy `provision.go.txt` into a
   temporary command directory inside the exact Zati source checkout and build
   for Linux ARM64. It uses Zati's existing encrypted platform SecretStore API,
   adds only two named speech secrets, and writes an exclusive mode-0600
   `VOICE_ROOT/controller.json`. Run with controller-root and voice-root paths;
   never print or copy existing controller secrets. It refuses to overwrite the
   configuration file. This is an operator setup helper, not a public operation.
7. Point the host controller's `ZATI_LOCAL_VOICE_CONFIG` at that file through a
   systemd drop-in and restart. Use controller revision above or newer. Verify
   public voice begin, backend exchange and session end using the normal
   authenticated CLI. Existing text-chat configuration stays in place.

The prepared source/dependency directory is an installation bundle, not a newly
built standalone service OCI image. The base image is immutable. Preserve the
bundle with its manifest; do not modify files under a running deployment.

## Evidence and remaining tests

`verification.json` records a real synthesis/recognition round trip: first PCM
151 ms, recognition 792 ms, and all 62,720 input samples finalized. The public
controller API admitted an existing qualified OpenRouter profile, established
its local backend exchange and closed the test session. No message was sent to
the reasoning model in this verification; no new OpenRouter spend was incurred.
These are bounded checks, not latency distributions or physical audio evidence.

The installed Mac still needs the owner's spoken test, microphone permission,
Anna listening approval, speaker echo/wake-word checks and endurance procedure.
The runtime uses offline local model references. Kernel-level outbound egress
blocking and its all-protocol qualification remain unverified. The supervisor
restarts on child exit; an alive-but-unhealthy model requires operator restart.

## Rollback

Remove only the voice environment/controller-override drop-ins, reload systemd,
and restart the controller to return to its previous binary/configuration.
Stop the owned runtime using Spark DELETE; retain the source, prepared models and
credential directories for recovery. Do not remove unrelated pods or caches.
Restore a compatible recorded runtime manifest before re-enabling voice.

The installed Mac process was relaunched after setup. A process sample shows its
main thread waiting in `SecItemCopyMatching` while SecurityAgent is running; it
has not yet opened a window or connected to DGX. Owner Keychain approval remains
necessary before the physical voice test. The password was not read or bypassed.
