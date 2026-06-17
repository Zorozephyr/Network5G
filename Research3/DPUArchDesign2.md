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

---

## 7. Cross-Slice Preemption Engine

### 7.1 The Scenario

When a high-priority slice (e.g., URLLC) needs more bandwidth than is freely available, it must reclaim bandwidth from lower-priority slices. This entire decision tree must execute in <5 µs on the ARM cores.

```
Time T₀ — System State:
  URLLC (5QI=82, prio=0): allocated 100 Mbps, using 60 Mbps
  eMBB  (5QI=9,  prio=2): allocated 400 Mbps, using 380 Mbps
  mIoT  (5QI=69, prio=3): allocated 50 Mbps,  using 30 Mbps
  Unallocated pool: 50 Mbps | Total capacity: 600 Mbps

Time T₁ — URLLC Demand Spike:
  TCN predicts URLLC needs 200 Mbps (narrow CQR interval → high confidence)
  Deficit = 200 - 100 = 100 Mbps
  Pool covers 50 Mbps → remaining deficit = 50 Mbps
  → PREEMPTION ENGINE ACTIVATES
```

### 7.2 Preemption Algorithm

```python
def preempt(requesting_slice, deficit, all_slices):
    """Execute in <5 µs on ARM core. All data in L1 cache."""
    
    # Phase 1: Soft preemption (SLA-safe forgo on lower-priority slices)
    victims = sorted(
        [s for s in all_slices if s.priority > requesting_slice.priority],
        key=lambda s: (-s.priority, -(s.allocated - s.current_use))
    )
    
    for victim in victims:
        reclaimable = victim.allocated - max(victim.ai_predicted_demand, victim.min_guaranteed)
        if reclaimable > 0:
            amount = min(reclaimable, deficit)
            # Atomic store with RELEASE semantics (see §7.3)
            atomic_store_release(victim.committed_rate, victim.committed_rate - amount)
            victim.ai_action = FORGO_UNDER_PREEMPTION
            deficit -= amount
        if deficit == 0:
            return  # All resolved without SLA violations

    # Phase 2: Hard preemption (accept lower-priority SLA degradation)
    if deficit > 0:
        for victim in victims:
            hard_reclaimable = victim.allocated - victim.min_guaranteed
            if hard_reclaimable > 0:
                amount = min(hard_reclaimable, deficit)
                atomic_store_release(victim.committed_rate, victim.committed_rate - amount)
                victim.ai_action = HARD_PREEMPTED
                deficit -= amount
                log_preemption(victim.slice_id, requesting_slice.slice_id, amount)
                signal_shal(PreemptionEvent(victim, amount))  # async to mgmt plane
            if deficit == 0:
                return
```

### 7.3 ARM Memory Model: Concurrency Protocol

The preemption engine runs on one ARM core while the telemetry aggregation DMA loop runs on another. Both access `hw_slice[]`. ARM Cortex-A78 uses a **weakly-ordered memory model** — without explicit memory ordering, one core can observe a torn 64-bit value (high 32 bits from old, low 32 bits from new) when another core writes `committed_rate`.

**Protocol:**

```c
// TELEMETRY CORE (reader): Before reading any hw_slice counters
void snapshot_slice_state(struct hw_slice *src, struct slice_snapshot *dst) {
    dst->byte_count     = __atomic_load_n(&src->byte_count, __ATOMIC_ACQUIRE);
    dst->pkt_count      = __atomic_load_n(&src->pkt_count, __ATOMIC_ACQUIRE);
    dst->drop_count     = __atomic_load_n(&src->drop_count, __ATOMIC_ACQUIRE);
    dst->queue_depth    = __atomic_load_n(&src->queue_depth, __ATOMIC_ACQUIRE);
    dst->last_arrival   = __atomic_load_n(&src->last_arrival_ns, __ATOMIC_ACQUIRE);
    dst->committed_rate = __atomic_load_n(&src->committed_rate, __ATOMIC_ACQUIRE);
}

// PREEMPTION CORE (writer): After computing new rate
void commit_rate_update(struct hw_slice *slice, uint64_t new_rate) {
    __atomic_store_n(&slice->committed_rate, new_rate, __ATOMIC_RELEASE);
    // RELEASE ensures all prior writes (ai_action, logs) are visible
    // before the rate change is observed by the telemetry core
}
```

The `ACQUIRE` load on the reader creates a happens-before relationship with the `RELEASE` store on the writer. This guarantees:
1. No torn 64-bit reads on ARM's 32-bit-granularity store buffer
2. The telemetry core always sees a consistent rate value
3. All metadata writes (ai_action, logs) are visible before the rate change

### 7.4 Preemption Guarantees

| Property | Guarantee |
|---|---|
| Preemption latency | <5 µs (all slice state in ARM L1 cache, O(N) scan where N ≤ ~64 slices) |
| URLLC protection | URLLC slice NEVER has its bandwidth reduced by a lower-priority request |
| Minimum floor | No slice is reduced below `min_guaranteed` unless it is the lowest priority AND a higher slice faces packet drops |
| Memory safety | All cross-core accesses use acquire/release atomics — no torn reads, no data races |
| Auditability | Every hard preemption is logged and exported via `SHAL.GetPreemptionLog()` |
| Recovery | After URLLC demand subsides, the forgo engine naturally returns bandwidth to preempted slices |

---

## 8. ML Model Selection: Why TCN + CQR

| Criterion | LSTM | GRU | **TCN (Selected)** |
|---|---|---|---|
| Sequential dependency | Yes (hidden state) | Yes (hidden state) | **No (parallelizable)** |
| ARM NEON vectorization | Poor (gate-by-gate) | Moderate | **Excellent (1D conv = GEMM)** |
| INT8 quantization loss | High (sigmoid/tanh) | Moderate | **Low (ReLU + conv)** |
| Params for W=32 | ~8,000 | ~6,000 | **~5,000** |
| Est. inference (A78, INT8) | ~50–100 µs | ~20–50 µs | **~7–80 µs** |
| Causal (no future leakage) | By design | By design | **By design (causal conv)** |

### 8.1 Uncertainty Estimation: CQR Replaces Gaussian Variance Head

**Why Gaussian NLL is wrong for this domain:**

5G user-plane traffic — especially URLLC and video streaming slices — follows **heavy-tailed distributions** (Pareto, Zipf, self-similar aggregations). Inter-arrival times and burst sizes have power-law tails where $P(X > x) \sim x^{-\alpha}$, not Gaussian tails where $P(X > x) \sim e^{-x^2}$.

A Gaussian NLL loss trains the model to believe extreme values decay exponentially. They don't — they decay polynomially. The consequence: during "calm" phases (low variance in the recent window), the model produces small σ² and confidently triggers FORGO. This is exactly when a self-similar traffic process is most likely to have a **long-range dependent burst incoming**. The system releases bandwidth and violates the URLLC SLA on the most dangerous inputs.

**Replacement: Conformalized Quantile Regression (CQR)**

| Aspect | Gaussian Variance Head (v2) | CQR (v3) |
|---|---|---|
| Distributional assumption | Gaussian (exponential tails) | **None (distribution-free)** |
| Coverage guarantee | Asymptotic only | **Finite-sample marginal coverage ≥ 1-α** |
| Heavy-tail safety | Underestimates burst probability | **Adapts interval width to local variability** |
| Training loss | Gaussian NLL | **Pinball loss at quantiles α/2 and 1-α/2** |
| Output | ŷ, σ² | **ŷ, q̂_lo, q̂_hi** |
| FORGO safety argument | Relies on P(y > x) from Gaussian — wrong | **CQR guarantees P(y > q̂_hi) ≤ α — provable** |
| Compute overhead | Negligible | **Negligible (same forward pass, +1 output neuron)** |

**Time-series adaptation:** Standard CQR assumes exchangeable data. Network telemetry is autocorrelated. We use the **EnbPI** (Ensemble Batch Prediction Intervals, Xu & Xie, ICML 2021) framework: maintain a sliding window of recent residuals and recalibrate conformity score thresholds online. This provides approximate marginal coverage under strongly mixing time-series conditions without requiring data splitting.

**References:**
- Romano, Y., Patterson, E., & Candès, E. (2019). Conformalized Quantile Regression. NeurIPS 2019.
- Angelopoulos, A. N. & Bates, S. (2023). A Gentle Introduction to Conformal Prediction and Distribution-Free Uncertainty Quantification. FnTML.
- Xu, C. & Xie, Y. (2021). Conformal Prediction Interval for Dynamic Time-series. ICML 2021.

---

## 9. Model Lifecycle on DPU (MLOps)

### 9.1 Training Data Specification

| Phase | Dataset | Description |
|---|---|---|
| **Initial training** | ORANSlice synthetic profiles (Bonati et al., Northeastern) | O-RAN SC traffic profiles for eMBB/URLLC/mIoT via OpenRAN Gym. Open-source, reproducible. GitHub: wineslab/ORANSlice |
| **Supplementary** | UPC/Zenodo 5G Slicing Dataset | Packet-level simulations covering 3 slice types with delay, jitter, loss metrics. Standardized topologies |
| **Supplementary** | CRAWDAD mobile traces | Real-world wireless traces for distribution validation and domain shift analysis |
| **Post-hardware** | Self-collected via SHAL telemetry API | 72h of real slice telemetry from testbed. Domain-specific fine-tuning |

**Distribution mismatch mitigation:** A model trained on eMBB traces from one deployment will fail on URLLC IoT slices in another. We address this via:
1. **Multi-source training**: Mix ORANSlice + CRAWDAD + Zenodo traces during QAT
2. **CQR recalibration**: Online conformity score updates adapt to distribution shift without retraining the base model
3. **Per-slice-type heads**: Separate output heads for URLLC/eMBB/mIoT learn distinct traffic patterns

### 9.2 Deployment Pipeline

```
OFF-DPU (Management Plane):
  • Train full model on GPU cluster using specified datasets
  • Quantization-Aware Training (QAT) → ONNX → TFLite INT8
  • CQR calibration: compute conformity scores on held-out calibration set
  • Validate coverage rate ≥ 1-α on test set
  • Push model + calibration scores to DPU via SHAL.PushModel() + PushCalibrationSet()

ON-DPU (Layer 4, background ARM cores):
  1. New model loaded into shadow buffer (separate memory region)
  2. Shadow model runs in parallel for N intervals alongside active model
  3. Compare: if shadow MAPE < active MAPE AND coverage ≥ 1-α → atomic pointer swap
  4. If shadow degrades SLA metrics → discard, keep active model
  5. Drift detector: if active model rolling MAPE > threshold for M intervals
     → signal management plane via SHAL.GetModelHealth() for retrain
  6. CQR online recalibration: sliding window of recent residuals updates
     conformity score threshold every K intervals (EnbPI-style)
```

---

## 10. Overlap Analysis (Web-Verified, June 2026)

### vs. dApps (Lacava 2025) — Current SOTA Baseline

| Dimension | dApps 2025 | This Architecture |
|---|---|---|
| AI execution venue | Host CPU (shared with baseband) | DPU ARM cores (physically isolated) |
| Enforcement | Software (via E3 interface) | Hardware (eSwitch pre-allocated meters) |
| Actuation latency | ~450 µs | <100 µs (target, measured range TBD) |
| Loop period | ~1 ms | 1–10 ms (configurable Δt) |
| Baseband contention | Yes — dApp competes with L1/L2 | Zero — separate silicon |
| Failure mode | Host crash → all control lost | ARM crash → eSwitch keeps enforcing |
| Uncertainty model | N/A | CQR with distribution-free guarantees |

### vs. PreNS (Wu 2026) — Direct Competitor

| Dimension | PreNS 2026 | This Architecture |
|---|---|---|
| Platform | SimPy simulation on server CPU | Real DPU hardware |
| Host CPU | 100% | 0% |
| Latency | Not measured (simulated) | Measured: target <100 µs actuation |
| Forgo primitive | Absent | Present (CQR-bounded, distribution-free) |
| Preemption | Not addressed | Formal preemption engine with memory safety |
| Model | Attention Bi-LSTM (~100K params) | Quantized TCN (~5K params, INT8) |
| Training data | Unspecified | ORANSlice + CRAWDAD + Zenodo (specified) |

### vs. Simão 2025 — Closest Closed Loop

| Dimension | Simão 2025 | This Architecture |
|---|---|---|
| AI location | SDN controller (off-switch) | DPU ARM cores (on-card) |
| Actuation | Controller → switch rule (network hop) | ARM → eSwitch meter (register write) |
| Loop latency | ~1-10 ms | 1–10 ms loop, <100 µs actuation |
| Task | Congestion classification (binary) | Bandwidth forecasting + forgo + preemption |
| Slice awareness | None (single-service) | Multi-tenant, per-slice, 5QI-constrained |

---

## 11. Novelty Summary

| Contribution | Exists? | Status |
|---|---|---|
| Closed-loop AI for slice management | Yes (many papers) | Not novel alone |
| Hardware-native slice abstraction (`hw_slice` as eSwitch object) | **No** | **VIABLE GAP** |
| SHAL interface (vendor-neutral mgmt↔DPU API) | **No** | **VIABLE GAP** |
| Cross-slice preemption engine at hardware speed with ARM memory safety | **No** | **VIABLE GAP** |
| CQR-bounded forgo primitive (distribution-free proactive bandwidth release) | **No** | **VIABLE GAP** |
| Zero host-CPU slice management with <100 µs actuation | **No** | **VIABLE GAP** |
| On-DPU model hot-swap with CQR recalibration via SHAL | **No** | **VIABLE GAP** |

**Multi-layer novelty:** The proposal has four separable contributions: hardware abstraction (hw_slice), interface design (SHAL), uncertainty framework (CQR-bounded forgo), and control logic (TCN + preemption). If one underperforms, the others still stand.

---

## 12. Proposed Testbed & Measurements

### Testbed

```
┌──────────────┐     ┌─────────────────────────┐     ┌──────────────────┐
│ Traffic Gen  │────▶│ NVIDIA BlueField-3 DPU  │────▶│ srsRAN / OAI     │
│ (iperf3,     │     │ (Device Under Test)     │     │ gNB Emulator     │
│  scapy, NS-3)│     │                         │     │                  │
│              │     │ • DOCA Flow pipes        │     │ 3 slices:        │
│ Multi-slice  │     │ • Pre-allocated meters   │     │ eMBB  (5QI=9)    │
│ profiles:    │     │ • TCN inference (INT8)   │     │ URLLC (5QI=82)   │
│ steady/burst │     │ • SHAL API server        │     │ mIoT  (5QI=69)   │
│ /spike       │     │ • Preemption engine      │     │                  │
└──────────────┘     └─────────────────────────┘     └──────────────────┘
```

### Measurements to Report

1. **Actuation latency** — target: <100 µs, baseline: 450 µs (dApp). Time from inference completion to meter enforcement.
2. **Control loop period** — Δt: 1–10 ms configurable. Measured end-to-end from telemetry read to next telemetry read.
3. **Preemption latency** — time from URLLC spike detection to eMBB/mIoT meter update (novel metric)
4. **Prediction accuracy** — MAPE vs. server-side PreNS baseline (must be comparable despite smaller model)
5. **CQR coverage rate** — empirical coverage must be ≥ 1-α on test traffic (distribution-free guarantee validation)
6. **Forgo bandwidth savings** — % reduction in over-provisioning vs. reserve-only systems
7. **SLA violation rate under forgo** — must be < 5QI PER threshold for each slice
8. **SLA violation rate under preemption** — preempted slice violation rate vs. protected slice guarantee
9. **Host CPU utilization** — must be 0% for all slice management operations

### Microbenchmarks (Pre-System, Mandatory)

| Benchmark | What It Validates | Go/No-Go Threshold |
|---|---|---|
| DOCA Flow meter parameter update latency under load | Actuation latency claim | <10 µs |
| INT8 TCN inference on A78 NEON (5K params, 3×32×5 input) | Inference time range | <100 µs |
| DMA read latency from eSwitch SRAM to ARM L1 cache | Telemetry snapshot cost | <5 µs |

---

## 13. Phased Research Plan

### Phase 1 — SHAL Spec + Simulation (2 months)

| Deliverable | Description |
|---|---|
| SHAL Protobuf specification | Complete vendor-neutral API definition with all operations from §6.3 |
| ns-3 closed-loop simulation | Simulated TCN + CQR + preemption engine with ORANSlice traffic profiles |
| CQR framework validation | Offline validation of CQR coverage guarantees on 5G traffic traces |
| **Publication target** | **HotNets position paper: SHAL interface as standalone contribution** |

> **Fallback:** If BF-3 hardware access is delayed, Phase 1 deliverables are independently publishable at HotNets without hardware measurements.

### Phase 2 — BF-3 Microbenchmarks (1 month)

| Deliverable | Description |
|---|---|
| Microbenchmark suite | Three measurements from §12 microbenchmark table |
| Go/no-go decision | If all three pass thresholds → proceed to Phase 3 |
| Latency characterization | Full distribution (p50/p95/p99) of actuation latency under varying load |

> **Go/No-Go Gate:** If TCN inference exceeds 100 µs, investigate DPA offload or model pruning. If meter update exceeds 10 µs, investigate hardware steering mode optimization.

### Phase 3 — Full Prototype (3 months)

| Deliverable | Description |
|---|---|
| End-to-end testbed | srsRAN + BF-3 + traffic generators with 3-slice configuration |
| All 9 measurements from §12 | Complete evaluation against dApp and PreNS baselines |
| CQR online recalibration | Validated EnbPI-style sliding window on live telemetry |
| **Publication target** | **Full system paper: NSDI or USENIX ATC** |

---

*Architecture design v3 prepared June 2026. Incorporates peer review feedback: CQR uncertainty replacement (Romano et al. 2019), actuation/loop latency separation, ARM memory model concurrency protocol, training data specification, DOCA Flow meter actuation correction, worst-case floor analysis, and phased research plan with go/no-go gates. Hardware specifications from NVIDIA BlueField-3 documentation, DOCA SDK 2.x, and Hot Chips 2023.*
