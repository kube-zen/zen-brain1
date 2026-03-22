# llama.cpp vs Ollama: 2×2 matrix (stack × quantization)

Generated: 2026-03-22T08:10:56-04:00

## Environment

- Host threads (logical): 20
- Thread cap: 16 (llama `-t`, Ollama `GGML_NUM_THREADS` / `OMP_NUM_THREADS`)
- Task: single-turn Go coding (ParseSemver), `max_tokens` / `num_predict`: 384
- Qwen3 thinking disabled: llama-server `--reasoning off`, Ollama `think: false`
- **Main `ollama` container is not stopped** (other services may use CPU/RAM).
- **Ollama Q4**: Hub has no separate Q4 tag for 0.8b; we `ollama create` from the **same GGUF** as llama Q4 (`FROM /model/q4.gguf` in a bench container).
- **llama Q8**: GGUF from [unsloth/Qwen3.5-0.8B-GGUF](https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF) `Q8_0` (downloaded once to `/home/neves/git/ai/Qwen3.5-0.8B-Q8_0.gguf` unless present).
- **Ollama Q8**: `qwen3.5:0.8b` (typical Hub default **Q8_0**).

| | Q4_K_M | Q8_0 |
|--|--------|------|
| **llama.cpp** | `Qwen3.5-0.8B-Q4_K_M.gguf` | `Qwen3.5-0.8B-Q8_0.gguf` |
| **Ollama** | `bench-qwen35-0.8b-q4km` (from same Q4 file) | `qwen3.5:0.8b` |

### Main `ollama` snapshot (start of run)

- `docker stats`: **Mem=1.857GiB / 62.57GiB CPU=0.00%**
- Memory limit: **unlimited**


## 2×2 matrix: stack × quantization

Same Go task for all cells (`ParseSemver`), `think: false` / `--reasoning off`, `max_tokens`/`num_predict`=384.

| Cell | Stack | Quant | Gen tok/s (median) | Gen time ms (median) | Prompt ms (median) | VmRSS peak (llama) / Docker after runs |
|------|-------|-------|----------------------|----------------------|---------------------|----------------------------------------|
| llama q4 | llama.cpp | Q4_K_M | 29.88 (min 29.47, max 31.09) | 12851.4 (min 12352.5, max 13029.2) | 38.6 (min 37.0, max 64.7) | 4342020 kB |
| llama q8 | llama.cpp | Q8_0 | 26.03 (min 25.42, max 26.26) | 14751.9 (min 13749.3, max 15103.8) | 44.2 (min 42.0, max 47.1) | 4449692 kB |
| ollama q4 | Ollama (Docker) | Q4_K_M | 20.57 (min 20.38, max 20.71) | 18672.4 (min 18540.0, max 18841.7) | 722.1 (min 691.3, max 736.8) | 1.551GiB / 62.57GiB (2.48%) |
| ollama q8 | Ollama (Docker) | Q8_0 | 18.42 (min 16.84, max 18.72) | 20515.7 (min 16981.5, max 22803.0) | 1112.4 (min 1034.6, max 1176.5) | 2.118GiB / 62.57GiB (3.39%) |

### Per-cell memory (detail)

#### `llama_q4`

- Artifact: `/home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf` (532517120 bytes)
- VmRSS idle → peak: 3965580 → 4342020 kB
- smaps peak: Rss_rollup=4342020 Pss_rollup=4338043 kB

#### `llama_q8`

- Artifact: `/home/neves/git/ai/Qwen3.5-0.8B-Q8_0.gguf` (811843840 bytes)
- VmRSS idle → peak: 4073516 → 4449692 kB
- smaps peak: Rss_rollup=4449692 Pss_rollup=4445715 kB

#### `ollama_q4`

- Model: `bench-qwen35-0.8b-q4km`
- Docker after load: 48.02MiB / 62.57GiB (0.07%)
- Docker after runs: 1.551GiB / 62.57GiB (2.48%)
- Memory limit: unlimited

#### `ollama_q8`

- Model: `qwen3.5:0.8b`
- API size: 1036046583 bytes
- Docker after load: 37.09MiB / 62.57GiB (0.06%)
- Docker after runs: 2.118GiB / 62.57GiB (3.39%)
- Memory limit: unlimited

### Sample output (first timed run, truncated per cell)

**llama_q4** (`Q4_K_M`)

```
```go
package semantic

import (
	"errors"
	"strings"
)

// ParseSemver parses a semantic version string into a struct with major, minor, and patch fields.
// It also validates the input string and returns an error if parsing fails.
//
// Examples:
//   ParseSemver("1.2.3")   -> {Major: 1, Minor: 2, Patch: 3}
//   ParseSemver("v2.0.1")  -> {Major: 2, Minor: 0, Patch: 1}
//   ParseSemver("10.20.30") -> {Major: 10, Minor: 20, Patch: 30}
//   ParseSemver("1.2.3-alpha") -> {Major: 1, Minor: 2, Patch: 3, Alpha: true}
//   ParseSemver("invalid")  -> errors.New("invalid semantic version")
//   ParseSemver("1.2.3.4")  -> errors.New("semantic version must be 3 parts")
//   ParseSemver("1.2.3.4.5") -> errors.New("semantic version must be 3 parts")
//   ParseSemver("1.2.3.4.5.6") -> errors.New("semantic version must be 3 parts")
//   ParseSemver("1.2.3.4.5.6.7") -> errors.New("semantic version must
... [truncated]
```

**llama_q8** (`Q8_0`)

```
```go
package semantic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ParseSemver parses a semantic version string into a struct with major, minor, and patch fields.
// It also handles the "v" prefix and returns an error for invalid input.
func ParseSemver(s string) (int, int, int, error) {
	// Strip leading "v" if present
	if s == "v" {
		s = s[1:]
	}

	// Split by dot
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("invalid semantic version: expected format 'major.minor.patch'")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, errors.New("invalid major version: must be a non-negative integer")
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, errors.New("invalid minor version: must be a non-negative integer")
	}

	// Ensure minor is non-negative
	if minor < 0 {
		return 0, 0, 0, errors.New
... [truncated]
```

**ollama_q4** (`Q4_K_M`)

```
```go
package semantic

import (
	"errors"
	"fmt"
	"strings"
)

// ParseSemver parses a semantic version string into a struct with major, minor, and patch fields.
// It supports the following formats:
// - "1.2.3" (major, minor, patch)
// - "v2.0.1" (major, minor, patch)
// - "10.20.30" (major, minor, patch)
// It also handles the "err" field for error cases.
//
// Note: The "v" prefix is stripped if present.
//
// Args:
//   s string: The semantic version string to parse.
//
// Returns:
//   struct {
//     Major int
//     Minor int
//     Patch int
//     Err error
//   }
//
// Error:
//   - "invalid version" if the input string is not a valid semantic version.
//   - "invalid version" if the input string is not a valid semantic version.
//
// Example:
//   fmt.Println(ParseSemver("1.2.3"))
//   fmt.Println(ParseSemver("v2.0.1"))
//   fmt.Println(ParseSemver("10.20.30"))
//   fmt.Prin
... [truncated]
```

**ollama_q8** (`Q8_0`)

```
```go
package semanticver

import (
	"errors"
	"fmt"
)

// ParseSemver parses a semantic version string into a struct with major, minor, and patch fields.
// It also handles the "v" prefix for versions like "v1.2.3".
// Returns an error if the input is invalid or contains non-numeric characters.
func ParseSemver(s string) (int, int, int, error) {
	if s == "" {
		return 0, 0, 0, errors.New("version string cannot be empty")
	}

	// Strip leading "v" if present
	s = s[1:]

	var major, minor, patch int
	var err error

	// Split by dot and validate each part
	parts := strings.Split(s, ".")
	for i, part := range parts {
		if len(part) == 0 {
			return 0, 0, 0, errors.New("version string contains empty parts")
		}

		// Check for non-numeric characters in the current part
		if !isNumeric(part) {
			return 0, 0, 0, fmt.Errorf("invalid character '%c' at position %d",
				part[0], i+1)
		}

		// P
... [truncated]
```

## Versions

- llama.cpp: `version: 8467 (990e4d969)`
- Report helper: `/tmp/bench_matrix_cells.py`
- Ollama image: `sha256:8318e7716a2d43ce66a3cfb8670153a4ca850fb757be45021370756938c15140`

## Method

1. Four sequential runs: llama Q4 → llama Q8 → Ollama Q4 (Modelfile from mounted GGUF) → Ollama Q8.
2. Each: 3 warmup requests, 5 timed requests.
3. Per-cell JSON was written under a temporary directory during the run (removed on exit); aggregates are in this report.

## Cross-reference: zen-brain1 Qwen 0.8B findings

This section ties **local CPU** measurements in this file to **product and ops** analysis under `~/zen/zen-brain1` (sources below; those documents were not re-executed for this update).

**Sources:**

- `docs/03-DESIGN/SMALL_MODEL_STRATEGY.md` — canonical small-model / local CPU strategy (ZB-023).
- `deploy/README.md` — certified local path: **only** `qwen3.5:0.8b` via **host Docker Ollama** (not in-cluster for the active local lane).
- `docs/05-OPERATIONS/WARMUP_FULL_REPORT.md` — cold vs warm behavior for **0.8B** in Ollama.
- `ZB-026F_SUCCESS.md` — end-to-end proof on **qwen3.5:0.8b** (CPU).

### Policy and deployment (ZB-023)

- **Certified local CPU model:** `qwen3.5:0.8b` **only**; other local models (e.g. 14B, llama*, mistral*) are **out of policy** unless the operator explicitly overrides.
- **Supported inference path for that lane:** **host Docker Ollama** (e.g. `http://host.k3d.internal:11434` in k3d), with enforcement via policy + CI gates as described in those docs.

### Warmup and latency expectations (`WARMUP_FULL_REPORT.md`)

- **Cold:** first load after start or unload often **30–120+ seconds** for **0.8B**, hardware-dependent.
- **Warm:** follow-on requests typically **a few seconds** for **0.8B** once resident.
- This benchmark used **discarded warmup** requests; production should still run **apiserver/gateway warmup** so the first real task does not pay cold-start alone.

### Strategy assumptions (`SMALL_MODEL_STRATEGY.md`)

- **CPU-first:** reference profile **14 cores / 64 GiB**; cold warmup **~30–60 s**; short warm requests **~3–5 s**; **multi-step** work often **~10–15 minutes** on CPU.
- **Throughput planning:** **~96–144 tasks/hour** under **certified local** assumptions via **many parallel workers**, not single-request tok/s alone.

### Live proof (`ZB-026F_SUCCESS.md`)

- **Model:** `qwen3.5:0.8b` (Ollama, CPU).
- **One real BrainTask:** **~10 m 7 s** for a **simple** implementation task (two Go files), within the **45 m** timeout profile.
- **Single warm API interaction** in logs: **~362 ms** (not comparable to full multi-minute generation).

### How the 2×2 matrix relates

| zen-brain1 theme | What this benchmark adds |
|------------------|---------------------------|
| **Default stack = Ollama + Hub `qwen3.5:0.8b` (Q8-class)** | The matrix separates **runtime** (llama.cpp vs Ollama) from **quantization** (Q4_K_M vs Q8_0). Same Go prompt everywhere. |
| **Real tasks are minutes, not one curl** | Aligns with **ZB-026F** and with **large Ollama prompt-eval** times measured here (Docker + server path overhead, not only matmul). |
| **Parallelism over single-stream speed** | This run used **single-slot** servers (`--parallel 1` / `OLLAMA_NUM_PARALLEL=1`). Production scales **workers** and queue depth. |

**Takeaway:** Zen-brain remains aligned with **Ollama + `qwen3.5:0.8b`** for the **certified** local lane. These numbers show **where** an optional **llama.cpp + GGUF** path can win on **raw generation speed** (especially **Q4**) and **where** **RSS / cgroup** differ—any adoption beside policy needs an **explicit operator decision**, not only bench results.

