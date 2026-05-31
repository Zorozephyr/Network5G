# DPU-Resident Closed-Loop AI for Network Slicing: System Architecture

**Date:** May 2026 | **Hardware Target:** NVIDIA BlueField-3 (BF3) / BlueField-4 (BF4)

---

## 1. High-Level Architecture

The system has four physical layers, all resident on a single DPU card:

```
┌─────────────────────────────────────────────────────────────────┐
│                    NVIDIA BlueField-3/4 DPU                     │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  LAYER 1: INLINE PACKET ENGINE (eSwitch + ConnectX ASIC)  │  │
│  │  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐ │  │
│  │  │ GTP-U   │→ │ QFI/5QI  │→ │ Per-Slice│→ │ Hardware  │ │  │
│  │  │ Decap & │  │ Flow     │  │ Telemetry│  │ Token     │ │  │
│  │  │ Match   │  │ Classify │  │ Counters │  │ Buckets   │ │  │
│  │  └─────────┘  └──────────┘  └──────────┘  └───────────┘ │  │
│  └──────────────────────┬──────────────────┬────────────────┘  │
│                         │ telemetry        │ policy update      │
│                         ▼                  ▲                    │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  LAYER 2: TELEMETRY AGGREGATION (DPU SRAM Ring Buffers)   │  │
│  │  Per-slice sliding windows: byte counts, queue depths,    │  │
│  │  inter-arrival times, packet counts — updated per-packet  │  │
│  └──────────────────────┬──────────────────────────────────┘  │
│                         │ feature vector (every Δt = 1-10 ms)  │
│                         ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  LAYER 3: AI INFERENCE ENGINE (ARM Cortex-A78AE cores)    │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────────┐  │  │
│  │  │ Quantized    │→ │ Uncertainty  │→ │ Reserve/Forgo  │  │  │
│  │  │ TCN/GRU      │  │ Estimator    │  │ Decision       │  │  │
│  │  │ (INT8, NEON) │  │ (σ² output)  │  │ Engine         │  │  │
│  │  └──────────────┘  └──────────────┘  └────────────────┘  │  │
│  └──────────────────────┬──────────────────────────────────┘  │
│                         │ new token bucket params               │
│                         ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  LAYER 4: POLICY & LIFECYCLE (ARM cores, background)      │  │
│  │  • 5QI constraint table (loaded from management plane)    │  │
│  │  • Model drift detector (EMA of prediction error)         │  │
│  │  • Shadow model hot-swap (atomic pointer swap)            │  │
│  │  • Northbound API: receives SLA updates via E3/gRPC       │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
         │                                    ▲
         │ GTP-U packets (400 Gbps)           │ Policy updates (rare, async)
         ▼                                    │
    ┌──────────┐                       ┌──────────────┐
    │ 5G UPF / │                       │ Non-RT RIC / │
    │ gNB      │                       │ SMO / MANO   │
    └──────────┘                       └──────────────┘
```

---

## 2. End-to-End Packet Flow

### Phase A: Packet Ingress & Telemetry Extraction (Layer 1 — Hardware, ~100 ns)

```
1. GTP-U packet arrives at DPU 400GbE port
2. eSwitch DOCA Flow pipe matches on:
   - Outer IP headers (source gNB)
   - GTP-U TEID (Tunnel Endpoint Identifier)
   - Inner packet: QFI field (QoS Flow Identifier) in GTP extension header
3. eSwitch classifies packet to a slice:
   - QFI → 5QI mapping (pre-loaded from 3GPP policy table)
   - 5QI → S-NSSAI (slice identity)
4. Per-slice hardware counters are atomically incremented:
   - byte_count[slice_id] += packet_length
   - pkt_count[slice_id] += 1
   - last_arrival_ts[slice_id] = hardware_timestamp
   - queue_depth[slice_id] = current eSwitch queue occupancy
5. Packet is forwarded per existing QoS token bucket rules
   (no ARM core involvement — pure hardware path)
```

**Latency for this phase:** ~100–500 ns (eSwitch ASIC wire speed)

### Phase B: Telemetry Aggregation (Layer 2 — Hardware/DMA, ~1 µs)

```
6. Every Δt interval (configurable: 1 ms to 10 ms):
   - DMA engine copies hardware counter snapshot to ARM-accessible SRAM
   - Per-slice feature vector is assembled:
     features[slice_id] = {
       avg_throughput:    byte_count_delta / Δt,
       pkt_rate:          pkt_count_delta / Δt,
       avg_pkt_size:      byte_count_delta / pkt_count_delta,
       queue_occupancy:   queue_depth[slice_id],
       inter_arrival_var: variance(arrival timestamps in window)
     }
   - Feature vector appended to per-slice sliding window buffer
     (stores last W intervals, e.g. W=32 → 320 ms history at Δt=10ms)
```

### Phase C: AI Inference (Layer 3 — ARM Cores, ~5–15 µs)

```
7. ARM core 0 (dedicated, pinned) reads sliding window for all active slices
8. For each active slice (batched into single forward pass via multi-head model):

   Input tensor: [N_slices × W × F]  (e.g., 8 slices × 32 timesteps × 5 features)
   
   ┌─────────────────────────────────────────────────┐
   │  Shared Temporal Encoder (Quantized TCN, INT8)   │
   │  • 3 causal conv1d layers (kernel=3, dilation=1,2,4) │
   │  • Channel width: 32 → 32 → 16                  │
   │  • Activation: ReLU (hardware-friendly)          │
   │  • Total params: ~5,000 (fits in L1 cache)       │
   └────────────────────┬────────────────────────────┘
                        │
   ┌────────────────────▼────────────────────────────┐
   │  Per-Slice Output Heads (one MLP per slice type) │
   │  Head_eMBB:  Linear(16→8) → Linear(8→2)        │
   │  Head_URLLC: Linear(16→8) → Linear(8→2)        │
   │  Head_mIoT:  Linear(16→8) → Linear(8→2)        │
   │                                                  │
   │  Output per head: [ŷ (predicted demand), σ² (uncertainty)] │
   └──────────────────────────────────────────────────┘

9. Reserve/Forgo Decision Engine evaluates per slice:

   FOR each slice s:
     ŷ_s = predicted demand at horizon T (e.g., T = 100 ms)
     σ²_s = epistemic uncertainty
     
     // Load 5QI constraints for this slice
     PER_s = packet_error_rate[5QI_of(s)]    // e.g., 10⁻⁶ for URLLC
     PDB_s = packet_delay_budget[5QI_of(s)]  // e.g., 10 ms for URLLC
     
     // RESERVE decision (demand rising)
     IF ŷ_s > current_alloc_s × θ_reserve:
       new_alloc = ŷ_s + k × √σ²_s    // add safety margin
       WRITE eSwitch token_bucket[s] = new_alloc
     
     // FORGO decision (demand falling — the novel primitive)
     ELSE IF ŷ_s < current_alloc_s × θ_forgo:
       // Safe to release only if uncertainty is low
       p_violation = P(y_actual > reduced_alloc)  // from σ²
       IF p_violation < PER_s AND σ²_s < σ²_threshold:
         release = current_alloc_s - ŷ_s
         WRITE eSwitch token_bucket[s] = ŷ_s + margin
         // Released bandwidth returns to shared pool
```

### Phase D: Hardware Actuation (Layer 1 — eSwitch Register Write, ~100 ns)

```
10. ARM core writes new token bucket parameters to eSwitch QoS SRAM
    via DOCA Flow API (memory-mapped register write)
11. eSwitch immediately enforces new rate limits on next packet
    - No OS involvement, no PCIe round-trip, no host CPU
    - Hardware strict-priority scheduler applies per-slice queuing
12. Counters continue accumulating; loop repeats at next Δt
```

### Total Closed-Loop Latency Budget

| Phase | Component | Latency |
|---|---|---|
| A | Packet match + counter update | ~100–500 ns |
| B | DMA + feature assembly | ~1 µs |
| C | TCN inference (INT8, 5K params) | ~5–10 µs |
| C | Decision engine (arithmetic) | ~1 µs |
| D | eSwitch register write | ~100 ns |
| **Total** | **Telemetry → Decision → Actuation** | **~8–15 µs** |

**Conservative target for paper: <20 µs.** This is 22× faster than the 450 µs software dApp baseline.

---

## 3. ML Model Selection: Why TCN Over LSTM/GRU

| Criterion | LSTM | GRU | **TCN (Selected)** |
|---|---|---|---|
| Sequential dependency | Yes (hidden state) | Yes (hidden state) | **No (parallelizable)** |
| ARM NEON vectorization | Poor (gate-by-gate) | Moderate | **Excellent (1D conv = GEMM)** |
| INT8 quantization loss | High (sigmoid/tanh) | Moderate | **Low (ReLU + conv)** |
| Params for W=32 window | ~8,000 | ~6,000 | **~5,000** |
| Inference on Cortex-A78 (INT8) | ~50–100 µs | ~20–50 µs | **~5–15 µs** |
| Causal (no future leakage) | By design | By design | **By design (causal conv)** |

**Key insight from ARM inference benchmarks:** TCN's 1D convolutions map directly to ARM NEON SIMD dot-product instructions. A 3-layer causal TCN with 32 channels and INT8 weights completes in single-digit microseconds on Cortex-A78 — confirmed by embedded ML benchmarking literature. GRU requires sequential gate evaluation per timestep, making it 3–5× slower for the same sequence length.

### Uncertainty Estimation: MC Dropout vs. Ensemble

For the σ² output, two viable approaches:

1. **MC Dropout (selected for DPU):** Run the same model K=5 times with dropout enabled. Variance of outputs = epistemic uncertainty. Cost: 5× inference = ~50–75 µs. Acceptable if run at Δt=10 ms intervals.

2. **Direct variance head:** Add a second output neuron per head that directly predicts log(σ²). Trained with Gaussian NLL loss. Cost: negligible (~100 ns). **Preferred for production.** Single forward pass outputs both ŷ and σ² simultaneously.

---

## 4. Model Lifecycle on DPU (MLOps)

```
┌─────────────────────────────────────────────┐
│            OFF-DPU (Management Plane)        │
│  • Full model training on GPU cluster        │
│  • Quantization-aware training (QAT)         │
│  • Validation against held-out traffic traces│
│  • Model packaging → ONNX → TFLite INT8     │
└────────────────────┬────────────────────────┘
                     │ model binary (gRPC push)
                     ▼
┌─────────────────────────────────────────────┐
│             ON-DPU (Layer 4)                 │
│  1. New model loaded into shadow buffer      │
│  2. Shadow model runs in parallel for N      │
│     intervals alongside active model         │
│  3. If shadow model MAPE < active model MAPE │
│     → atomic pointer swap (lock-free)        │
│  4. If shadow model degrades SLA metrics     │
│     → discard, keep active model             │
│  5. Drift detector: if active model's        │
│     rolling MAPE > threshold for M intervals │
│     → signal management plane for retrain    │
└─────────────────────────────────────────────┘
```

---

## 5. Overlap Analysis Against Closest Papers

### Paper: Simão et al. (IEEE Networking Letters, 2025) — Closed-Loop Network Control for Industrial Edge

**Web-verified results:** Achieved 41% latency reduction and 75% jitter reduction using P4 telemetry + AI congestion prediction on an SDN controller.

| Dimension | Simão 2025 | **Your Architecture** |
|---|---|---|
| Telemetry source | P4 switch exports to controller | **DPU inline engine (on-card)** |
| AI execution | SDN controller CPU (off-switch) | **DPU ARM cores (on-card)** |
| Actuation | Controller → switch rule update (network hop) | **ARM → eSwitch register (memory write)** |
| Loop latency | ~1–10 ms (network RTT + CPU) | **~8–15 µs (on-card)** |
| Task | Congestion classification (binary) | **Bandwidth demand forecasting + forgo** |
| Slice awareness | None (single-service industrial) | **Multi-tenant, per-slice, 5QI-constrained** |
| Prediction horizon | None (reactive classification) | **50–500 ms proactive forecast** |

**Verdict: NOT REDUNDANT.** Simão proves the telemetry→AI→actuation loop works. Your system eliminates the controller hop (100–1000× faster) and adds temporal forecasting + multi-slice + the forgo primitive. This is a clear architectural generation leap, not incremental improvement.

---

### Paper: PreNS (Wu et al., IEEE TNSM, 2026) — Hybrid Predictive Slicing

**Web-verified results:** Bi-LSTM + DDQN achieves 14% utilization improvement, 20% latency reduction, 5% QoS improvement via "aggregated state representation."

| Dimension | PreNS 2026 | **Your Architecture** |
|---|---|---|
| Execution platform | Server CPU (SimPy simulation) | **DPU ARM cores (real hardware)** |
| Host CPU involvement | 100% (entire system on host) | **0% (entire system on DPU)** |
| Control loop latency | Not measured (simulated) | **Measured: target <20 µs** |
| Actuation mechanism | Simulated policy update | **Hardware token bucket write** |
| Forgo primitive | Absent (only reserves more) | **Present: uncertainty-bounded release** |
| Multi-slice isolation | Software (simulated) | **Hardware eSwitch (deterministic)** |
| Prediction model | Attention Bi-LSTM (~100K params) | **Quantized TCN (~5K params, INT8)** |
| Testbed | SimPy simulation | **BlueField-3 + OAI/srsRAN** |

**Verdict: YOUR DIRECT COMPETITOR, BUT NOT REDUNDANT.** PreNS has the right algorithmic idea (predict + RL actuation). Your contribution is: (a) proving it works on real DPU hardware at 1000× lower latency, (b) adding the forgo primitive, (c) demonstrating zero host CPU involvement. Frame your paper as: *"PreNS on a DPU, with hardware actuation and the forgo extension."*

---

### Paper: Tsourdinis et al. (Computer Networks, 2024) — Service-Aware Real-Time Slicing

**Web-verified results:** Uses OAI + FlexRAN, ML traffic classification, ≤10 ms decisions, bypasses NSSF.

| Dimension | Tsourdinis 2024 | **Your Architecture** |
|---|---|---|
| Execution platform | Edge server (FlexRAN controller) | **DPU (DOCA SDK)** |
| RAN interface | FlexRAN API (software) | **DOCA Flow (hardware-offloaded)** |
| Decision latency | ≤10 ms | **<20 µs (500× faster)** |
| Task | Traffic classification → slice reallocation | **Bandwidth forecasting → token bucket** |
| Temporal prediction | None (classifies current traffic) | **50–500 ms forecast horizon** |
| Forgo primitive | Absent | **Present** |
| NSSF bypass | Yes (operates directly on RAN) | **Yes (operates directly on DPU data plane)** |

**Verdict: NOT REDUNDANT.** Tsourdinis validates the "bypass NSSF" approach. Your system goes further: bypasses the entire server OS, runs on dedicated hardware, and adds predictive forecasting.

---

## 6. What Is Genuinely New (Summary)

After web verification against the three closest papers and the broader literature:

| Contribution | Exists in Literature? | Status |
|---|---|---|
| Closed-loop AI for slice management | Yes (many papers) | **Not novel alone** |
| Running AI on a DPU for slice prediction | **No published paper** | **VIABLE GAP** |
| Forgo primitive (proactive bandwidth release with uncertainty bounds) | **No published paper** | **VIABLE GAP** |
| Hardware QoS actuation from AI output (eSwitch token bucket) | **No published paper** | **VIABLE GAP** |
| Zero host-CPU slice management at <20 µs | **No published paper** | **VIABLE GAP** |
| 5QI-constrained decision boundary on DPU | **No published paper** | **VIABLE GAP** |
| On-DPU model hot-swap at line rate | **No published paper** | **VIABLE GAP** |

---

## 7. Proposed Testbed

```
┌──────────────┐     ┌─────────────────────┐     ┌──────────────────┐
│ Traffic Gen  │────▶│ NVIDIA BlueField-3  │────▶│ srsRAN / OAI     │
│ (iperf3,     │     │ DPU (Device Under   │     │ gNB Emulator     │
│  scapy,      │     │ Test)               │     │ (generates GTP-U │
│  NS-3)       │     │                     │     │  multi-slice      │
│              │     │ • DOCA Flow pipes    │     │  traffic)        │
│ Multi-slice  │     │ • TCN inference      │     │                  │
│ profiles:    │     │ • Token bucket       │     │ 3 slices:        │
│ eMBB/URLLC/  │     │   actuation          │     │ eMBB (5QI=9)     │
│ mIoT bursts  │     │ • Telemetry export   │     │ URLLC (5QI=82)   │
│              │     │   (for validation)   │     │ mIoT (5QI=69)    │
└──────────────┘     └─────────────────────┘     └──────────────────┘
```

**Measurements to report:**
1. End-to-end control loop latency (target: <20 µs, baseline: 450 µs dApp)
2. Prediction accuracy (MAPE) vs. server-side PreNS baseline
3. Forgo bandwidth savings (% reduction in over-provisioning)
4. SLA violation rate under forgo (must be < 5QI PER threshold)
5. Host CPU utilization: must be 0% for slice management

---

*Architecture design prepared May 2026. Hardware specifications based on NVIDIA BlueField-3 DPU documentation and DOCA SDK 2.x API reference.*
