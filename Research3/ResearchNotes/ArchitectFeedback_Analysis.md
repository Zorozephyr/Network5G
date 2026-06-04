# Architect Feedback: Decoded Directives & Gap Analysis

**Date:** June 2026  
**Source:** Lead architect verbal feedback, reconstructed from notes  
**Status:** Directives decoded — none formalized into architecture yet

---

## Directive 1: "Slices in Hardware, Slice Policies Directly in Hardware"

### What the Architect Is Actually Saying

The architect is pushing you to make **hardware-native slice policy enforcement** the central architectural claim — not just a feature of the system, but the defining contribution. The distinction:

| Current Framing (Your Docs) | Architect's Framing |
|---|---|
| "We run AI on the DPU and it updates QoS tables" | "The slice **is** the hardware. The policy **lives** in silicon, not in software config files" |
| DPU as an AI execution venue | DPU as a **slice enforcement primitive** — the hardware IS the slice boundary |

This is a subtle but critical reframing. Today, a "network slice" is a software abstraction — a set of QoS rules enforced by `tc`, OpenFlow, or DPDK traffic shapers running on a host CPU. The architect is saying: **make the slice a hardware object.** A slice becomes a set of eSwitch registers, token bucket parameters, and flow classification rules that exist in silicon — not in a Linux process.

### Concrete Advantages (What Your Paper Must Articulate)

| Advantage | Software Slice (Current SOTA) | Hardware Slice (Your Architecture) |
|---|---|---|
| **Enforcement determinism** | Probabilistic — depends on OS scheduler, CPU load, memory pressure | Deterministic — eSwitch ASIC enforces per-packet regardless of ARM core state |
| **Isolation guarantee** | Best-effort — a misbehaving slice process can consume shared CPU/memory | Physical — each slice's registers are isolated in hardware; one slice cannot corrupt another's state |
| **Failure independence** | If the host OS crashes, all slices lose enforcement | If an ARM core crashes, the eSwitch continues enforcing the **last-written** policy — graceful degradation |
| **Audit trail** | Software logs can be tampered | Hardware counters are read-only from the host — tamper-resistant telemetry |
| **Latency floor** | ~100 µs (Linux `tc` on PREEMPT_RT kernel) | <100 ns (eSwitch register read → packet scheduling) |
| **Scalability** | Each slice adds CPU overhead (more `tc` rules = more CPU cycles per packet) | Each slice is a hardware flow table entry — O(1) lookup regardless of slice count (up to hardware table limits) |

### Use Cases (Where Hardware Slices Are Mandatory, Not Optional)

1. **Industrial Private 5G (URLLC):** Factory automation with 1 ms end-to-end, 99.999% reliability. A software enforcement hiccup of 150 µs violates the SLA. Hardware enforcement is the only path to deterministic guarantees under load.

2. **Multi-Tenant Neutral Host:** A venue (stadium, airport) where multiple operators share the same physical infrastructure. Each operator's slice MUST be isolated at the hardware level — a software isolation failure is a regulatory compliance violation (data sovereignty, SLA contracts).

3. **V2X (Vehicle-to-Everything):** Safety-critical vehicle communication requires that a slice allocated for emergency braking messages cannot be starved by infotainment traffic under any circumstances — including host CPU failure.

4. **Defense/Government Networks:** Classified traffic slices must be provably isolated from unclassified slices. Hardware isolation provides a stronger security argument for certification (e.g., Common Criteria EAL).

### Gap in Your Current Documents

Your `DPUArchDesign.md` describes the eSwitch as an enforcement mechanism (Layer 1, Layer 4) but does not formalize the **slice-as-hardware-object** abstraction. You need a data structure specification:

```c
// Per-slice hardware state (lives in eSwitch SRAM)
struct hw_slice {
    uint32_t  slice_id;           // S-NSSAI mapping
    uint8_t   qfi;                // QoS Flow Identifier
    uint8_t   fiveqi;             // 5QI class
    uint8_t   priority;           // Strict priority level (0 = highest)
    
    // Token bucket parameters (enforcement)
    uint64_t  committed_rate;     // CIR in bytes/sec
    uint64_t  peak_rate;          // PIR in bytes/sec  
    uint32_t  burst_size;         // CBS in bytes
    
    // Telemetry counters (read-only from ARM)
    uint64_t  byte_count;         // Monotonic byte counter
    uint64_t  pkt_count;          // Monotonic packet counter
    uint64_t  drop_count;         // Packets dropped by token bucket
    uint64_t  last_arrival_ns;    // Hardware timestamp of last packet
    uint16_t  queue_depth;        // Current queue occupancy
    
    // AI-managed fields (written by ARM cores)
    uint64_t  ai_predicted_demand;   // ŷ(t+T) from TCN
    uint64_t  ai_uncertainty;        // σ² from variance head
    uint8_t   ai_action;             // RESERVE / HOLD / FORGO
};
```

**This struct IS the slice.** It does not reference any software process. If the ARM cores crash, the eSwitch continues enforcing the `committed_rate` and `peak_rate` values indefinitely — the slice survives a software failure.

---

## Directive 2: "Need to Focus on Speed/Latency Benefits"

### What the Architect Is Actually Saying

The architect is warning you: **do not lead with "AI on DPU."** Lead with **speed.** The AI is the mechanism; the speed is the value proposition. Reviewers and industry audiences care about latency numbers, not about where the neural network runs.

### Reframed Paper Narrative

| Current Narrative (Your Docs) | Architect's Preferred Narrative |
|---|---|
| "We propose a DPU-resident AI system for slice management" | "We demonstrate sub-20 µs closed-loop slice control — 22× faster than the current SOTA — by collapsing telemetry, inference, and enforcement into a single hardware device" |
| Problem → AI Solution → DPU is where AI runs | Latency Problem → Physics-based argument why software can't fix it → DPU architecture that eliminates the physics bottlenecks → AI is the decision engine within that architecture |

### The Latency Argument Hierarchy (What the Paper Should Present)

```
Layer 0: The Physics Floor
├── PCIe Gen5 x16 round-trip: ~200-500 ns (unavoidable for host-CPU systems)
├── Linux kernel context switch: 2-10 µs (PREEMPT_RT best case)  
├── Software interrupt handling: 1-5 µs
├── Memory copy NIC→host RAM: 1-3 µs (DMA + cache miss)
└── TOTAL SOFTWARE FLOOR: ~5-20 µs minimum BEFORE any AI inference
    
Layer 1: Current SOTA Latency Stack
├── E2 interface round-trip (xApp): 10-100 ms
├── dApp co-located (Lacava 2025): ~450 µs
│   ├── Includes: OS scheduling + NIC→CPU telemetry + inference + E2 actuation
│   └── Excludes: the AI inference itself is only ~50-100 µs of that 450 µs
├── Simão (P4+SDN controller): 1-10 ms
└── PreNS (simulated, no real latency): N/A

Layer 2: Your DPU Architecture
├── Telemetry capture (eSwitch → SRAM): ~100-500 ns (NO PCIe, NO host)
├── Feature assembly (DMA to ARM): ~1 µs
├── TCN inference (INT8, ARM NEON): ~5-10 µs
├── Decision engine (arithmetic): ~1 µs
├── Actuation (ARM → eSwitch register): ~100 ns (NO network hop)
└── TOTAL: ~8-15 µs
    
SPEEDUP vs. dApp SOTA: 450 µs / 15 µs = 30×
SPEEDUP vs. xApp: 50 ms / 15 µs = 3,333×
```

### What This Means for Paper Structure

Section 1 (Introduction) should open with the **latency crisis**, not with "AI for network slicing." The hook:

> *"The fastest published closed-loop control for RAN slicing operates at 450 µs. For URLLC slices with 0.125 ms slot timing (numerology µ=3), this means the control system cannot react within a single scheduling slot. We present the first architecture that closes the loop in <20 µs — within a single slot — by eliminating every software-imposed latency source."*

---

## Directive 3: "Wants to Create a New Interface Between Software and Hardware"

### What the Architect Is Actually Saying

This is the architect's most architecturally significant suggestion. They are proposing a **new standardizable interface** — analogous to what the O-RAN Alliance did with E2 (RIC↔RAN) and what Lacava proposed with E3 (RIC↔dApps) — but for the boundary between **software management plane** and **hardware enforcement plane**.

### The Interface Gap

Currently, the O-RAN and 3GPP stacks have well-defined interfaces between software components:

```
Non-RT RIC ←─ A1 ──→ Near-RT RIC ←─ E2 ──→ O-DU/O-CU ←─ E3 ──→ dApps
     │                                                              │
     └── All of these are software-to-software interfaces ──────────┘
```

**There is no standardized interface between the management plane and a hardware enforcement device like a DPU.** Today, each vendor (NVIDIA DOCA, Marvell OCTEON, AMD Pensando) has a proprietary SDK. There is no common abstraction.

### Proposed: The Slice Hardware Abstraction Layer (SHAL)

Your paper can propose a vendor-neutral interface specification — call it **SHAL** (Slice Hardware Abstraction Layer) or **H-Plane Interface** — that defines:

```
┌─────────────────────────────────────────────────────────┐
│                    Management Plane                      │
│  (Non-RT RIC / SMO / MANO / Operator Portal)            │
└────────────────────────┬────────────────────────────────┘
                         │
                    SHAL Interface
                    (gRPC / Protobuf)
                         │
        ┌────────────────┼────────────────────┐
        │                │                    │
        ▼                ▼                    ▼
   ┌─────────┐     ┌──────────┐      ┌────────────┐
   │ NVIDIA   │     │ Marvell  │      │ AMD        │
   │ BF-3/4   │     │ OCTEON   │      │ Pensando   │
   │ (DOCA)   │     │ (SDK)    │      │ (P4)       │
   └─────────┘     └──────────┘      └────────────┘
```

### SHAL Interface Operations (What It Must Support)

| Operation | Direction | Description |
|---|---|---|
| `CreateSlice(slice_spec)` | Mgmt → DPU | Instantiate a hardware slice object with QoS parameters, 5QI mapping, initial token bucket |
| `UpdateSlicePolicy(slice_id, new_params)` | Mgmt → DPU | Modify SLA constraints that the on-DPU AI uses as decision boundaries |
| `GetSliceTelemetry(slice_id)` | DPU → Mgmt | Export aggregated telemetry (not raw per-packet — that stays on-card) |
| `GetAIDecisionLog(slice_id, window)` | DPU → Mgmt | Export the AI's recent reserve/forgo decisions with confidence scores — auditability |
| `PushModel(model_binary, model_metadata)` | Mgmt → DPU | Deploy a new quantized model to the DPU's shadow buffer |
| `GetModelHealth()` | DPU → Mgmt | Report active model's drift metrics, MAPE, last retrain timestamp |
| `PreemptSlice(victim_id, preemptor_id)` | Mgmt → DPU | Explicit preemption command (see Directive 4 below) |
| `ForceForgo(slice_id, amount)` | Mgmt → DPU | Override AI: force-release bandwidth from a slice (operator manual intervention) |

### Why This Is a Strong Paper Contribution

1. **Standards bodies care about interfaces.** If the O-RAN Alliance or ETSI can adopt this interface, the paper's impact extends beyond academia.
2. **Vendor-neutral framing** makes the paper applicable to NVIDIA, Marvell, AMD, and Intel DPU ecosystems — broader reviewer appeal.
3. **It separates the AI contribution from the systems contribution.** The SHAL interface is publishable even if the AI model is a simple linear regression. The interface is the architectural novelty; the AI is a pluggable component behind it.

### Gap in Your Current Documents

Your `DPUArchDesign.md` Layer 4 mentions a "Northbound API: receives SLA updates via E3/gRPC" but does not specify the interface operations, message formats, or semantics. This needs to be formalized into a proper API specification.

---

## Directive 4: "What Happens When a High-Priority Slice Needs Bandwidth from a Low-Priority Slice"

### What the Architect Is Actually Saying

This is the **preemption scenario** — the most operationally critical edge case that your paper MUST address to survive peer review. It is also where the "forgo" primitive meets real-world operational pressure.

### The Scenario in Detail

```
Time T₀: System State
├── URLLC slice (5QI=82, priority=0): allocated 100 Mbps, using 60 Mbps
├── eMBB slice (5QI=9, priority=2):   allocated 400 Mbps, using 380 Mbps  
├── mIoT slice (5QI=69, priority=3):  allocated 50 Mbps, using 30 Mbps
└── Total link capacity: 600 Mbps (50 Mbps unallocated)

Time T₁: URLLC Demand Spike
├── URLLC slice suddenly needs 200 Mbps (factory robot emergency trajectory)
├── Current allocation: 100 Mbps → deficit: 100 Mbps
├── Unallocated pool: 50 Mbps → still short: 50 Mbps
└── MUST preempt lower-priority slices to find 50 Mbps
```

### The Decision Tree (What the DPU Must Execute in <20 µs)

```
1. URLLC demand spike detected by TCN model
   ŷ_URLLC(t+T) = 200 Mbps, σ² = low → high confidence
   
2. RESERVE triggered: need 200 Mbps, have 100 Mbps allocated
   Deficit = 100 Mbps
   
3. Check unallocated pool: 50 Mbps available
   Remaining deficit = 50 Mbps
   
4. PREEMPTION ENGINE activates (priority-ordered victim selection):
   
   Step 4a: Identify lowest-priority slice with reclaimable bandwidth
   └── mIoT (priority=3): allocated 50, using 30 → reclaimable = 20 Mbps
       ├── Check: will reducing mIoT to 30 Mbps violate mIoT SLA?
       │   └── 5QI=69 PER threshold = 10⁻¹ (very tolerant) → SAFE
       └── FORGO-UNDER-PREEMPTION: reduce mIoT allocation 50→30 Mbps
       
   Remaining deficit = 30 Mbps
   
   Step 4b: Next victim — eMBB (priority=2)
   └── eMBB: allocated 400, using 380 → reclaimable = 20 Mbps
       ├── Check: will reducing eMBB to 370 Mbps violate eMBB SLA?
       │   └── 5QI=9 PER threshold = 10⁻³ (moderate) → check σ²_eMBB
       │       └── If eMBB demand is predicted to stay at 380 → UNSAFE
       │       └── If eMBB demand is predicted to drop to 350 → SAFE
       └── Decision depends on eMBB TCN prediction + uncertainty
       
   Step 4c: If deficit remains → HARD PREEMPTION
   └── Force-reduce eMBB allocation regardless of SLA prediction
       ├── Accept temporary eMBB degradation to protect URLLC
       ├── Log the preemption event for operator audit
       └── Signal management plane: "eMBB SLA at risk due to URLLC preemption"

5. Write new token bucket values to eSwitch:
   URLLC: 200 Mbps (increased)
   eMBB:  370 Mbps (reduced — preempted)
   mIoT:  30 Mbps  (reduced — forgo under preemption)
   
6. Total: 600 Mbps (fully utilized, zero waste)
```

### The Formal Preemption Priority Rule

```python
preempt_order = sorted(
    active_slices, 
    key=lambda s: (
        -s.priority,                    # lowest priority first (highest number)
        -(s.allocated - s.current_use), # most unused bandwidth first
        s.sla_sensitivity               # least SLA-sensitive first
    )
)

for victim in preempt_order:
    reclaimable = victim.allocated - max(victim.predicted_demand, victim.min_guaranteed)
    if reclaimable > 0:
        forgo_amount = min(reclaimable, remaining_deficit)
        victim.allocated -= forgo_amount
        remaining_deficit -= forgo_amount
    if remaining_deficit == 0:
        break

if remaining_deficit > 0:
    # HARD PREEMPTION: violate lower-priority SLA to protect higher-priority
    for victim in preempt_order:
        hard_reclaim = min(victim.allocated - victim.absolute_minimum, remaining_deficit)
        victim.allocated -= hard_reclaim
        remaining_deficit -= hard_reclaim
        log_sla_violation(victim.slice_id, hard_reclaim)
        if remaining_deficit == 0:
            break
```

### Why This Scenario Is Critical for the Paper

1. **Reviewers will ask this question.** Any slicing paper that does not address inter-slice priority preemption will be rejected for incompleteness.
2. **It exercises the "forgo" primitive under pressure.** The forgo decision is not just "demand is falling, release bandwidth" — it is "a higher-priority slice needs your bandwidth NOW, and the AI must decide in microseconds whether it is safe to release."
3. **It demonstrates the hardware advantage.** The entire preemption decision tree above must execute in <20 µs. In a software system, this would require multiple `tc` rule updates, kernel scheduler intervention, and potentially a round-trip to the RIC — pushing into milliseconds.
4. **It creates a measurable benchmark.** "Preemption latency" — the time from URLLC demand spike detection to eMBB/mIoT token bucket update — is a concrete, novel metric no existing paper reports.

### Gap in Your Current Documents

Your `ProblemSpaceAnalysis.md` and `DPUArchDesign.md` describe RESERVE and FORGO as independent decisions per slice. The **cross-slice preemption logic** — where one slice's RESERVE triggers another slice's forced FORGO — is completely unaddressed. This is the missing piece.

---

## Summary: Four Directives → Four Paper Contributions

| # | Architect Directive | Paper Contribution | Status in Current Docs |
|---|---|---|---|
| 1 | "Slices in hardware" | Hardware-native slice abstraction (`hw_slice` struct) with failure-independent enforcement | Partially covered — eSwitch mentioned but not formalized as a slice primitive |
| 2 | "Focus on speed" | Sub-20 µs closed-loop with physics-based latency argument hierarchy | Covered in DPUArchDesign.md but not framed as the primary narrative |
| 3 | "New interface between software and hardware" | SHAL (Slice Hardware Abstraction Layer) — vendor-neutral API specification | **NOT covered** — only a vague "gRPC northbound" mention exists |
| 4 | "High-priority preempts low-priority" | Cross-slice priority preemption engine with forgo-under-preemption | **NOT covered** — reserve/forgo are per-slice only, no cross-slice interaction |

> [!IMPORTANT]
> **Directives 3 and 4 are completely absent from the current architecture documents.** These are the two elements the architect is pushing you toward that represent genuinely new work. The SHAL interface and the preemption engine are potentially the strongest contributions — they are systems architecture novelty, not just "we ran AI on different hardware."

---

*Decoded June 2026. Based on verbal architect feedback mapped against DPUArchDesign.md, ProblemSpaceAnalysis.md, and PaperAnalysis.md.*
