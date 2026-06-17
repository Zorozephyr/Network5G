# DPU-Resident Closed-Loop AI for Network Slicing: System Architecture (v3)

**Date:** June 2026 | **Hardware Target:** NVIDIA BlueField-3 (BF3)  
**Revision:** v3 — Incorporates peer review feedback: CQR uncertainty replacement, latency claim separation, ARM memory model fixes, training data specification, DOCA Flow actuation correction, phased research plan

---

## 1. Design Thesis

The fastest published closed-loop control for RAN slicing operates at **450 µs** (Lacava et al., Computer Networks 2025). For URLLC slices with 0.125 ms slot timing (numerology µ=3), this means the control system cannot react within a single scheduling slot. Every existing system — from the centralized NWDAF (seconds) to co-located dApps (450 µs) — is bottlenecked by the same physics: PCIe bus transfers, OS context switches, and shared-CPU scheduling contention.

This architecture eliminates **every** software-imposed latency source by collapsing telemetry collection, AI inference, and QoS enforcement into a single DPU chip. The system achieves:

- **The slice IS a hardware object** — enforcement lives in eSwitch ASIC registers, not software config files
- **The AI runs on physically isolated silicon** — DPU ARM cores share nothing with the host CPU or baseband
- **A new standardizable interface (SHAL)** bridges the management plane and the hardware enforcement plane
- **Cross-slice preemption** executes at hardware speed when high-priority slices demand bandwidth from low-priority ones

### Latency Architecture (Two Distinct Claims)

This architecture makes **two separable latency claims** that must not be conflated:

| Metric | Value | Definition |
|---|---|---|
| **Control loop period** | Configurable: 1–10 ms (Δt) | How often new telemetry drives a new AI inference and decision |
| **Actuation latency** | Target: <20 µs | Time from decision trigger to eSwitch meter enforcement |

The **actuation latency** is the novel contribution — the time from when inference completes to when the eSwitch enforces the new rate. The **control loop period** is Δt, the telemetry aggregation interval. Compared to dApps (450 µs actuation within a ~1 ms loop), this architecture targets **22× faster actuation** within a comparable loop period.

### Comparison Against Current SOTA

| System | Loop Period | Actuation Latency | AI Venue | Host CPU |
|---|---|---|---|---|
| NWDAF (3GPP R17+) | Seconds–minutes | Seconds | Central server | 100% |
| Non-RT RIC rApps | > 1 second | > 1 second | Remote SMO | 100% |
| Near-RT RIC xApps | 10 ms – 1 s | 10–100 ms | Edge server | 100% |
| Simão 2025 (P4+SDN) | 1–10 ms | 1–10 ms | SDN controller | 100% |
| **dApps (Lacava 2025)** | **~1 ms** | **~450 µs** | **DU co-located CPU** | **Shared** |
| **This Architecture** | **1–10 ms** | **<20 µs (target)** | **DPU ARM cores** | **0%** |

### Worst-Case Floor Argument

Even if TCN inference takes **80 µs** (8× slower than best-case estimate), total actuation latency would be ~85 µs — still **5.3× faster than dApp SOTA**. The architecture's value proposition survives across a wide range of ARM inference performance:

| Scenario | TCN Inference | Total Actuation | Speedup vs. dApps |
|---|---|---|---|
| Best case (warm L1, no contention) | ~7 µs | ~12 µs | 37× |
| Expected case | ~15–25 µs | ~20–30 µs | 15–22× |
| Worst case (cold L1, DMA contention) | ~80 µs | ~85 µs | 5.3× |

---

## 2. Hardware Platform: What Is Actually "Hardware" vs. "Software"

The BlueField-3 DPU contains three distinct compute components on one chip:

| Component | Has OS? | Nature | Role in This System |
|---|---|---|---|
| **eSwitch + ConnectX-7 ASIC** | No. Pure silicon logic. | **True hardware** | Per-packet flow classification, counter updates, meter enforcement at 400 Gbps wire speed |
| **16× ARM Cortex-A78 cores** | Yes — Ubuntu Linux 24.04 | **Software on dedicated HW** | Runs AI inference (TCN), decision engine, SHAL API server, model lifecycle |
| **DPA (Data Path Accelerator)** | No. Programmable microthreads. | **Programmable hardware** | 16 cores / 256 threads for custom packet processing (future use) |

**Precise claim:** AI-driven slice decisions execute on dedicated ARM cores **physically isolated** from the host CPU, and the resulting QoS policies are enforced by the eSwitch ASIC via pre-allocated meter objects at hardware line-rate, with **zero host CPU involvement** in the telemetry-inference-actuation loop. If ARM cores crash, the eSwitch continues enforcing the last-written meter parameters — graceful degradation.

---

## 3. The Hardware Slice Abstraction

### 3.1 Core Concept: The Slice IS a Hardware Object

In current production 5G, a "network slice" is a software abstraction — QoS rules enforced by `tc`, OpenFlow, or DPDK traffic shapers on a host CPU. In this architecture, **a slice is a set of eSwitch registers:**

| Property | Software Slice (Current SOTA) | Hardware Slice (This Architecture) |
|---|---|---|
| Enforcement determinism | Probabilistic — depends on OS scheduler | Deterministic — eSwitch enforces per-packet regardless of ARM state |
| Isolation guarantee | Best-effort — shared CPU/memory | Physical — isolated hardware registers per slice |
| Failure independence | Host OS crash → all slices fail | ARM crash → eSwitch continues enforcing last-written policy |
| Latency floor | ~100 µs (Linux `tc` on PREEMPT_RT) | <100 ns (eSwitch meter → packet scheduling) |
| Scalability | Each slice adds CPU overhead | O(1) hardware lookup regardless of slice count |

### 3.2 Hardware Slice Data Structure

```c
// Per-slice hardware state (lives in eSwitch SRAM)
// CONCURRENCY MODEL: ARM cores access via atomic load/store with
// acquire/release semantics (see Section 7.3 for memory ordering protocol)
struct hw_slice {
    // Identity (set at slice creation via SHAL)
    uint32_t  slice_id;            // S-NSSAI mapping
    uint8_t   qfi;                 // QoS Flow Identifier
    uint8_t   fiveqi;              // 5QI class (e.g., 82=URLLC, 9=eMBB)
    uint8_t   priority;            // Strict priority level (0 = highest)

    // Meter parameters (ENFORCEMENT — written by ARM, enforced by eSwitch)
    // Writes use __atomic_store_n(..., __ATOMIC_RELEASE)
    _Atomic uint64_t  committed_rate;      // CIR in bytes/sec
    _Atomic uint64_t  peak_rate;           // PIR in bytes/sec
    uint32_t  burst_size;          // CBS in bytes
    uint64_t  min_guaranteed;      // Absolute floor — never go below this

    // Hardware counters (TELEMETRY — written by eSwitch per-packet, read by ARM)
    // Reads use __atomic_load_n(..., __ATOMIC_ACQUIRE)
    _Atomic uint64_t  byte_count;          // Monotonic byte counter
    _Atomic uint64_t  pkt_count;           // Monotonic packet counter
    _Atomic uint64_t  drop_count;          // Packets dropped by meter
    _Atomic uint64_t  last_arrival_ns;     // Hardware timestamp of last packet
    _Atomic uint16_t  queue_depth;         // Current queue occupancy

    // AI state (written by ARM decision engine)
    uint64_t  ai_predicted_demand; // ŷ(t+T) from TCN
    double    ai_lower_quantile;   // q̂_α/2 from CQR (replaces σ²)
    double    ai_upper_quantile;   // q̂_{1-α/2} from CQR
    uint8_t   ai_action;           // RESERVE=0 / HOLD=1 / FORGO=2 / PREEMPTED=3

    // SLA constraints (loaded from management plane via SHAL)
    double    per_threshold;       // Packet Error Rate from 5QI table
    uint32_t  pdb_ms;              // Packet Delay Budget in ms
};
```

**This struct IS the slice.** No software process, no container, no Linux namespace. The eSwitch enforces `committed_rate` and `peak_rate` on every packet autonomously via pre-allocated DOCA Flow meter objects.

---

## 4. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        NVIDIA BlueField-3 DPU                            │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  LAYER 1: eSwitch ASIC (True Hardware — No OS, Per-Packet)        │  │
│  │  ┌──────────┐  ┌──────────┐  ┌───────────┐  ┌─────────────────┐  │  │
│  │  │ GTP-U    │→ │ QFI/5QI  │→ │ Per-Slice │→ │ Pre-allocated   │  │  │
│  │  │ Decap &  │  │ Flow     │  │ HW Counter│  │ Meter Enforce   │  │  │
│  │  │ PSC Match│  │ Classify │  │ Update    │  │ (per-packet)    │  │  │
│  │  └──────────┘  └──────────┘  └───────────┘  └─────────────────┘  │  │
│  └─────────────────────┬──────────────────────────┬─────────────────┘  │
│                        │ counter snapshots         │ meter param update │
│                        │ (DMA, every Δt)           │ (register write)   │
│                        ▼                           ▲                    │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  LAYER 2: Telemetry Aggregation (ARM + DMA, ~1 µs)               │  │
│  │  Per-slice sliding window: W=32 intervals × 5 features           │  │
│  │  [throughput, pkt_rate, avg_pkt_size, queue_depth, IAT_variance]  │  │
│  └─────────────────────┬──────────────────────────────────────────┘  │
│                        │ feature tensor [N×32×5]                      │
│                        ▼                                              │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  LAYER 3: AI Inference + Decision Engine (ARM Cores)              │  │
│  │  ┌──────────────┐  ┌───────────────┐  ┌────────────────────────┐ │  │
│  │  │ Quantized    │→ │ Reserve/Forgo │→ │ Preemption Engine      │ │  │
│  │  │ TCN (INT8)   │  │ per-slice     │  │ (cross-slice priority  │ │  │
│  │  │ ŷ + CQR     │  │ decision      │  │  reallocation)         │ │  │
│  │  │ quantiles    │  │               │  │                        │ │  │
│  │  └──────────────┘  └───────────────┘  └────────────────────────┘ │  │
│  └─────────────────────┬──────────────────────────────────────────┘  │
│                        │                                              │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │  LAYER 4: SHAL Interface + Lifecycle (ARM Cores, Background)     │  │
│  │  • SHAL API server (gRPC/Protobuf) — vendor-neutral northbound   │  │
│  │  • 5QI constraint table (loaded from management plane)            │  │
│  │  • Model drift detector (EMA of prediction error)                 │  │
│  │  • Shadow model hot-swap (atomic pointer swap)                    │  │
│  │  • Preemption audit log (exportable via SHAL)                     │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
         │                                          ▲
         │ GTP-U packets (400 Gbps)                 │ SHAL Interface
         ▼                                          │ (gRPC/Protobuf)
    ┌──────────┐                             ┌──────────────┐
    │ 5G UPF / │                             │ Non-RT RIC / │
    │ gNB      │                             │ SMO / MANO   │
    └──────────┘                             └──────────────┘
```

---

## 5. End-to-End Packet Flow & Closed Loop

### Phase A: Packet Ingress & Telemetry (eSwitch ASIC, ~100-500 ns)

```
1. GTP-U packet arrives at DPU 400GbE port
2. eSwitch pipeline (6 hardware stages, zero software):
   Stage 1: Outer header match (MAC, VLAN, IP)
   Stage 2: GTP-U decapsulation — extract TEID + QFI from PDU Session
            Container extension header (type 0x85, per 3GPP TS 38.415)
            ✓ VERIFIED: BF-3 supports RTE_FLOW_ITEM_TYPE_GTP_PSC at line
            rate via ConnectX-7 hardware steering (NVIDIA DOCA Flow docs,
            DPDK mlx5 driver documentation)
   Stage 3: Flow classification — QFI → 5QI → slice_id via lookup table
   Stage 4: Atomic counter update in eSwitch SRAM:
            hw_slice[slice_id].byte_count += pkt_len
            hw_slice[slice_id].pkt_count  += 1
            hw_slice[slice_id].queue_depth = current_occupancy
            hw_slice[slice_id].last_arrival_ns = hw_clock
   Stage 5: Meter check — pass/drop based on committed_rate
   Stage 6: Forward to destination (host PCIe, network port, or ARM)
3. ARM core involvement: ZERO. Host CPU involvement: ZERO.
```

### Phase B: Telemetry Aggregation (ARM + DMA, ~1 µs)

```
4. Every Δt interval (configurable: 1-10 ms), a pinned ARM core:
   - Takes consistent snapshot via __atomic_load_n(..., __ATOMIC_ACQUIRE)
     on all hw_slice[] counter fields (prevents torn 64-bit reads on
     ARM's weakly-ordered memory model — see Section 7.3)
   - Computes delta from previous snapshot
   - Derives 5 features per slice:
     throughput      = bytes_delta / Δt          (Mbps — macro demand trend)
     pkt_rate        = pkts_delta / Δt           (burstiness signal)
     avg_pkt_size    = bytes_delta / pkts_delta   (traffic type fingerprint)
     queue_occupancy = direct read                (earliest congestion signal)
     iat_variance    = variance(timestamps)       (predictability indicator)
   - Appends to per-slice sliding window buffer (last W=32 intervals)
```

### Phase C: AI Inference + Decision Engine (ARM Cores)

```
5. Dedicated ARM core reads sliding windows for all active slices
6. Single batched forward pass through multi-head TCN:

   Input:  [N_slices × 32 × 5]  (e.g., 3 slices × 32 timesteps × 5 features)

   Shared Temporal Encoder (Quantized TCN, INT8, ARM NEON):
   • 3 causal conv1d layers (kernel=3, dilation=1,2,4)
   • Channels: 32 → 32 → 16, ReLU activation
   • ~5,000 parameters (fits entirely in L1 cache)

   Per-Slice Output Heads (one MLP per slice type):
   • Head_URLLC: Linear(16→8) → Linear(8→3) → [ŷ, q̂_lo, q̂_hi]
   • Head_eMBB:  Linear(16→8) → Linear(8→3) → [ŷ, q̂_lo, q̂_hi]
   • Head_mIoT:  Linear(16→8) → Linear(8→3) → [ŷ, q̂_lo, q̂_hi]

   Uncertainty via Conformalized Quantile Regression (CQR):
   • Training: pinball loss at quantiles α/2 and 1-α/2 (replaces Gaussian NLL)
   • Calibration: offline conformal calibration on held-out set adjusts
     quantile width to guarantee marginal coverage ≥ 1-α
   • Why CQR over Gaussian: 5G user-plane traffic exhibits heavy-tailed
     distributions (Pareto, self-similar aggregations). Gaussian NLL assumes
     exponential tail decay → underestimates burst probability during calm
     phases → unsafe FORGO triggers. CQR provides distribution-free coverage
     guarantees without distributional assumptions.
     (Romano et al., NeurIPS 2019; Angelopoulos & Bates, FnTML 2023)
   • Time-series adaptation: online recalibration of conformity scores using
     sliding residual window (EnbPI-style, Xu & Xie, ICML 2021) to handle
     autocorrelation without exchangeability assumption.

7. Per-slice decision (Reserve / Hold / Forgo):

   FOR each slice s:
     PER_s = hw_slice[s].per_threshold     // from 5QI table
     
     // CQR prediction interval: [q̂_lo, q̂_hi] with coverage ≥ 1-α
     interval_width = q̂_hi_s - q̂_lo_s
     
     // RESERVE (demand rising — use upper quantile as safety bound)
     IF q̂_hi_s > current_alloc × θ_reserve:
       new_alloc = q̂_hi_s                  // upper quantile IS the margin
       WRITE hw_slice[s].committed_rate = new_alloc  // via RELEASE store
     
     // FORGO (demand falling — distribution-free safety guarantee)
     ELSE IF q̂_hi_s < current_alloc × θ_forgo:
       // CQR guarantees: P(y_actual > q̂_hi) ≤ α
       // FORGO is safe iff α < PER_s (the 5QI packet error rate)
       IF α < PER_s AND interval_width < width_threshold:
         WRITE hw_slice[s].committed_rate = q̂_hi_s + margin
         released_bw += (current_alloc - new_alloc)

8. Cross-slice preemption check (see Section 7)
```

### Phase D: Hardware Actuation (Meter Parameter Update)

```
9. ARM core updates pre-allocated DOCA Flow meter parameters:
   - Meters are created at slice instantiation (one per hw_slice)
   - Actuation modifies meter CIR/PIR via mlx5_flow_meter_modify()
   - This is a memory-mapped register update to existing meter objects,
     NOT a flow rule insertion (which can take 100–305 µs per rule)
   - Estimated meter parameter update: ~1–5 µs (register write + firmware ack)
   
   NOTE: Full DOCA Flow rule insertion (mlx5dv_dr_action_create) measured
   at 100–305 µs in published benchmarks. This system avoids per-decision
   rule insertion by pre-allocating all flow rules at slice creation time
   and modifying only meter rate parameters at runtime.

10. eSwitch immediately enforces new meter limits on the NEXT packet
    — no OS involvement, no PCIe round-trip, no host CPU
11. Counters continue accumulating; loop repeats at next Δt
```

### Latency Budget (Actuation Path Only)

| Phase | Component | Latency |
|---|---|---|
| B | DMA snapshot + feature vector assembly | ~1 µs |
| C | TCN inference (INT8, ~5K params, NEON) | ~7–80 µs (see worst-case table §1) |
| C | Decision engine + preemption check | ~1–2 µs |
| D | Meter parameter update (pre-allocated) | ~1–5 µs |
| **Total** | **Inference → Decision → Actuation** | **~10–88 µs** |

**Conservative paper target: <100 µs actuation.** Even worst-case (88 µs) is **5× faster** than dApp SOTA (450 µs). Best case (~12 µs) delivers **37× speedup**.

> **⚠ Validation Requirement:** The TCN inference estimate (7–80 µs range) is bounded by embedded ML benchmarks on comparable ARM Cortex-A78 platforms, NOT measured on BlueField-3. No public BF-3 ARM inference benchmarks exist. Hardware microbenchmarking is **mandatory** before paper submission (see Phase 2, Section 13).

---

## 6. SHAL: Slice Hardware Abstraction Layer (New Interface Proposal)

### 6.1 The Interface Gap

The O-RAN and 3GPP stacks define well-specified software-to-software interfaces:

```
Non-RT RIC ←─ A1 ──→ Near-RT RIC ←─ E2 ──→ O-DU/O-CU ←─ E3 ──→ dApps
```

**There is no standardized interface between the management plane and a hardware enforcement device.** Each DPU vendor (NVIDIA DOCA, Marvell OCTEON, AMD Pensando) uses a proprietary SDK. SHAL fills this gap as a **vendor-neutral abstraction** for hardware-resident slice management, designed to meet O-RAN WG interface requirements with architecture suitable for future standardization consideration.

### 6.2 SHAL Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Management Plane                      │
│  (Non-RT RIC / SMO / MANO / Operator Portal)            │
└────────────────────────┬────────────────────────────────┘
                         │
                    SHAL Interface
                    (gRPC / Protobuf over TLS)
                         │
        ┌────────────────┼────────────────────┐
        │                │                    │
        ▼                ▼                    ▼
   ┌─────────┐     ┌──────────┐      ┌────────────┐
   │ NVIDIA   │     │ Marvell  │      │ AMD        │
   │ BF-3/4   │     │ OCTEON   │      │ Pensando   │
   │ (DOCA)   │     │ (SDK)    │      │ (P4)       │
   └─────────┘     └──────────┘      └────────────┘
   
   Each vendor implements the SHAL API server on its ARM/control cores.
   The management plane sends identical messages regardless of vendor.
```

### 6.3 SHAL Operations

#### Slice Lifecycle

| Operation | Direction | Description |
|---|---|---|
| `CreateSlice(slice_spec)` | Mgmt → DPU | Instantiate `hw_slice` struct in eSwitch SRAM, create DOCA Flow rules + pre-allocated meter objects, set 5QI mapping and priority |
| `DeleteSlice(slice_id)` | Mgmt → DPU | Remove flow table entry + meter + counters. Released bandwidth returns to shared pool |
| `UpdateSlicePolicy(slice_id, params)` | Mgmt → DPU | Modify SLA constraints (PER threshold, PDB, min_guaranteed) that the AI uses as decision boundaries |

#### Telemetry & Auditability

| Operation | Direction | Description |
|---|---|---|
| `GetSliceTelemetry(slice_id)` | DPU → Mgmt | Export aggregated telemetry (throughput, drops, queue depth). Raw per-packet data stays on-card |
| `GetAIDecisionLog(slice_id, window)` | DPU → Mgmt | Export recent reserve/forgo/preempt decisions with CQR interval widths — operator auditability |
| `GetPreemptionLog(window)` | DPU → Mgmt | Export cross-slice preemption events with victim/preemptor IDs, amounts, and SLA impact |

#### AI Model Lifecycle

| Operation | Direction | Description |
|---|---|---|
| `PushModel(model_binary, metadata)` | Mgmt → DPU | Deploy new quantized model to shadow buffer for validation |
| `PushCalibrationSet(scores)` | Mgmt → DPU | Update CQR conformity score thresholds from latest calibration run |
| `GetModelHealth()` | DPU → Mgmt | Report active model MAPE, CQR coverage rate, drift metrics, last retrain timestamp |

#### Operator Overrides

| Operation | Direction | Description |
|---|---|---|
| `PreemptSlice(victim_id, preemptor_id)` | Mgmt → DPU | Explicit preemption command from operator |
| `ForceForgo(slice_id, amount)` | Mgmt → DPU | Override AI: force-release bandwidth (manual intervention) |
| `LockSlice(slice_id)` | Mgmt → DPU | Freeze a slice's allocation — AI cannot modify it until unlocked |

### 6.4 Why SHAL Is a Strong Paper Contribution

1. **Standards bodies value interfaces.** Designed to meet O-RAN WG interface requirements, with architecture suitable for future standardization consideration.
2. **Vendor-neutral framing** makes the paper applicable across NVIDIA, Marvell, AMD, Intel ecosystems.
3. **Separates AI from systems contribution.** The SHAL interface is publishable even with a simple ML model — the interface is the architectural novelty; the AI is a pluggable component behind it.
4. **Independently publishable.** SHAL alone merits a HotNets position paper, establishing priority on the interface concept while hardware validation proceeds.
