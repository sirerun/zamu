# OpenRouter reasoning-model qualification (bounded provider probe)

Date (UTC): 2026-11-13. This receipt records the bounded provider-measurement
slice of [plan.md](../plan.md) T5.4 (select and configure the voice reasoning
model). It is a measurement record and recommendation only; it does not mark
T5.4 complete and does not claim a qualified daily-use voice release.

## Authorization and spend

The owner approved a $5 spend cap for this measurement. Actual spend was about
$0.00012 total, far below the cap: the full batch account-usage delta was
$0.000036 and the cost computed from token pricing was $0.000101. No billing
account or spending setting was changed. Only the measurement's own spend
delta is recorded here; absolute account balance and usage totals are not.

## Endpoint and credentials

All calls targeted the real official OpenRouter API at
`https://openrouter.ai/api/v1`; no proxy was used. The reasoning credential
(`OPENROUTER_API_KEY`) was used for reasoning calls. Speech credentials were
not used as reasoning credentials. The key value was never recorded.

## Method

Three low-cost tool-capable models were probed. Per model:

- 6 latency samples with the prompt "Reply with exactly the word OK and
  nothing else.", `max_tokens` 64, `temperature` 0.
- 3 tool-call scenarios (`get_weather` for Paris, Tokyo and London;
  `tool_choice` auto; `max_tokens` 256; `temperature` 0).

Cost was computed as `prompt_tokens*prompt_price +
completion_tokens*completion_price` using `/models` pricing and cross-checked
against the account usage delta.

## Results

| model | median latency (n=6) | range | tool fidelity | price/M prompt | price/M completion | ctx |
| --- | --- | --- | --- | --- | --- | --- |
| mistralai/mistral-nemo | 405 ms | 384-1165 ms | 3/3 | $0.029 | $0.030 | 131072 |
| meta-llama/llama-3.1-8b-instruct | 452 ms | 307-832 ms | 3/3 | $0.020 | $0.040 | 131072 |
| openai/gpt-oss-20b | 735 ms | 668-1143 ms | 3/3 | $0.030 | $0.130 | 131072 |

## Recommendation

Recommended model: **mistralai/mistral-nemo**. Basis, in order: full tool
fidelity first, then lowest median latency, then lowest completion price.

Alternative: meta-llama/llama-3.1-8b-instruct — cheaper prompt price for
history-heavy voice turns and lower worst-case latency, at a slightly higher
median. openai/gpt-oss-20b is more capable but slower and pricier; it is a
reasoning model (the short-output probe consumed tokens on reasoning).

## Limits and pending integration

This is a bounded provider probe, NOT a full conversation-level qualification.
Full T5.4 integration remains pending: pin `voice.reasoning_model` in Zati's
governed profile through Settings on the installed Mac candidate, then select
it in Talk to Zati (blocked on installation and owner Keychain approval).
Worker/chat profiles must remain unchanged, and cost/token limits must stay
exposed as T5.4 requires.

Full runtime qualification per [qualification.md](../qualification.md) is a
separate gate; [delivery.md](../delivery.md) already lists the OpenRouter
profile measurement as outstanding installed-runtime work. This receipt
supplies that measurement and recommendation only. No installed daily-use
acceptance is claimed.

Structured data: [openrouter-reasoning-qualification.json](openrouter-reasoning-qualification.json).
