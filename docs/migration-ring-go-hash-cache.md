# Migration: `signer-context-cache` → ring-go `hash-cache` signing API

Guidance for downstream consumers (notably **PATH**) currently building against
the SDK's `perf/signer-context-cache` branch, moving to the new
`perf/ring-go-hash-cache` branch.

## TL;DR

- `Signer.Sign` no longer performs the off-chain optimization. It is now the
  **deterministic, consensus-safe** default (ring-go `Ring.Sign`), matching the
  SDK's original `main` behavior.
- The off-chain fast path moved to **new named methods**: `SignOffChain` and
  `SignOffChainWithRing`.
- If you leave your `Sign` call sites unchanged, **nothing breaks** — signatures
  are still valid and verify identically. You simply lose the speedup until you
  switch to the off-chain methods.
- Requires the **Go 1.26** toolchain (propagates from the SDK).
- ⚠️ The off-chain context cache is **unbounded** — call `ClearSignerContextCache()`
  on session rollover or it leaks slowly. See step 3.

## Why it changed

On `signer-context-cache`, `Signer.Sign` was silently the cached off-chain path.
That overloaded a single method with off-chain-only behavior (skipped self-checks,
non-deterministic per-signature work). The new design mirrors ring-go itself:
`Sign` stays the safe deterministic default, and the optimization is an explicit,
named opt-in — so nothing on a consensus-critical path can pick it up by accident.

## API mapping

| `signer-context-cache` (old)     | `hash-cache` (new)                     | Notes |
|----------------------------------|----------------------------------------|-------|
| `signer.Sign(ctx, req, appRing)` | `signer.SignOffChain(ctx, req, appRing)` | to keep the speedup on off-chain relay signing |
| `signer.SignWithRing(ctx, req, ring)` | `signer.SignOffChainWithRing(ctx, req, ring)` | external/session-cached ring |
| `signer.ClearSignerContextCache()` | `signer.ClearSignerContextCache()`   | unchanged |
| `signer.Sign(...)` (want deterministic) | `signer.Sign(ctx, req, appRing)`   | now genuinely deterministic — use on consensus-critical paths |

`NewSignerFromHex` is unchanged.

## Steps

### 1. Bump the SDK dependency

```sh
go get github.com/pokt-network/shannon-sdk@perf/ring-go-hash-cache
go mod tidy
```

> The SDK pins ring-go to the `perf/hash-cache` branch commit (not a tagged
> release yet) and requires **Go 1.26**. Make sure CI and every downstream of
> PATH has the 1.26 toolchain before merging. (ring-go also bumped go-ethereum to
> v1.17.4 for CVEs — only active with the optional `ethereum_secp256k1` cgo tag.)

### 2. Switch off-chain relay/session signing to the new methods

```go
// before (signer-context-cache): Sign WAS the cached off-chain path
signedReq, err := signer.Sign(ctx, relayRequest, appRing)

// after (hash-cache): opt into the off-chain fast path explicitly
signedReq, err := signer.SignOffChain(ctx, relayRequest, appRing)
```

```go
// before
signedReq, err := signer.SignWithRing(ctx, relayRequest, sessionRing)
// after
signedReq, err := signer.SignOffChainWithRing(ctx, relayRequest, sessionRing)
```

Only apply this on the **off-chain** relay/session signing path. Keep plain
`Sign` for anything consensus-critical / gas-metered.

### 3. Reuse the signer and ring for cache hits

The `SignerContext` is cached per **ring pointer**. To hit the cache across many
signs, reuse:

- the same `*Signer` instance, and
- the same ring pointer — prefer `SignOffChainWithRing` with a ring you cache by
  session, so rebuilding the ring per message does not discard the precompute.

> ### ⚠️ You MUST evict the cache on session rollover
>
> The off-chain `signerContextCache` is keyed by **ring pointer** and is
> **unbounded** — it grows one entry per distinct ring (i.e. per session),
> alongside your own ring cache. It is **not evicted automatically**. If you never
> clear it, it leaks slowly (memory climbs over days).
>
> Call **`ClearSignerContextCache()` on session rollover**, when the old rings are
> no longer used. This clears the whole cache; rebuild lazily on the next sign.
> (PATH wires this into its existing per-session rollover eviction.)

### 4. Validate

- Your unit tests should pass unchanged after the call-site edit.
- End-to-end: measure per-relay signing latency + throughput before/after under
  load, and report the ring/session size — gains scale with it.

## Notes

- `SkipSelfCheck` is already enabled inside the off-chain path; no action needed.
  It is safe here because the context is always signed with the same private key
  it was built from.
- Both `Sign` and `SignOffChain*` produce signatures that **verify identically**;
  on-chain verification is unchanged.
