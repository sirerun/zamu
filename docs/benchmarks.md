# Voice runtime measurements

| Date | Component / revision | Resources | Observed result | Limits |
| --- | --- | --- | --- | --- |
| 2026-10-13 | Narrate a8ad488, Pocket b94db177, pinned Anna | Spark 1 CPU / 2 GiB / no GPU | First PCM 97/76 ms; max frame gap 34 ms; cancellation 11 ms; recovery passed | Two phrases, CPU probe; no p95, speaker or simultaneous STT claim |

| 2026-10-13 | MISTT Whisper base.en 911407f4, pinned model | Spark 4 CPU / 4 GiB / no GPU | Six decodes 510–649 ms; simultaneous Anna/STT 1.065 s; cancellation cleanup 434 ms; exact fixture words and sample coverage | Three synthesized phrases, two repeats; not real microphone or p95 |

Exact model and asset hashes, commands and runtime provenance are recorded in
Narrate's `services/pocket/qualification/runtime-receipt.json`.
