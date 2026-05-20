# Variant C: Hybrid Architecture — DPU Hardware + Host Wasm

**Target:** Tier-1 Operators with Heavy DPI / ML-Based Inspection Needs  
**Platform:** BlueField-3 DPU (eSwitch) + Host Intel Xeon / GNR-D (Wasm compute)  
**Status:** All individual components are production-ready. End-to-end integration is novel and unvalidated. Recommended as future work.

---

## 1. Why This Variant Exists

Variant A places Wasm on the DPU's ARM cores — but ARM Cortex-A78 cores are "wimpy." They cannot handle compute-heavy DPI workloads like ML-based anomaly detection, complex regex pattern matching, or encrypted traffic analysis.

Variant C solves this by splitting responsibilities:
- **DPU eSwitch:** Handles 99.9% forwarding in hardware (hairpin)
- **DPU ARM cores:** Receive exception packets, perform lightweight classification, then DMA to host
- **Host x86 P-Cores:** Run WasmEdge with full CPU power (AVX-512, AMX, QAT) for heavy DPI

This gives the best of both worlds: hardware line-rate forwarding AND full x86 compute for inspection.

### Target Deployments

| Deployment | Example | Why This Variant Specifically |
|:---|:---|:---|
| **Encrypted traffic DPI** | Enterprise requiring TLS inspection | QAT on host decrypts → Wasm inspects → QAT re-encrypts |
| **ML anomaly detection** | Operator-level IDS/IPS | AMX matrix ops inside Wasm for inference at scale |
| **High-volume DPI** | Tier-1 core with millions of sessions | ARM cores insufficient for throughput; x86 scales with DLB |
| **Regulatory compliance** | Financial services, healthcare | Deep content inspection with audit trail per tenant |

---

## 2. End-to-End Architecture

### 2.1 Kubernetes Cluster Layout

```
┌──────────────────────────────────────────────────────────────┐
│              Host Kubernetes Cluster (x86)                     │
│                                                               │
│   ┌──────────────────────────────────────────────────────┐    │
│   │           WasmPlugin Controller (Go)                  │    │
│   │           Watches WasmPlugin CRDs                     │    │
│   │           Coordinates host sidecar + DPU agent        │    │
│   └───────────────────┬──────────────────────────────────┘    │
│                       │                                       │
│   ┌───────────────────▼──────────────────────────────────┐    │
│   │                    UPF Pod (Host)                      │    │
│   │                                                       │    │
│   │   ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │    │
│   │   │ UPF-CP   │  │ DPU Mgmt │  │ Wasm Sidecar     │   │    │
│   │   │ (PFCP)   │  │ Agent    │  │ (Rust daemon)    │   │    │
│   │   │          │  │          │  │                   │   │    │
│   │   │ N4 term  │  │ Programs │  │ WasmEdge x86 AOT │   │    │
│   │   │ Session  │  │ DOCA Flow│  │ DLB eventdev     │   │    │
│   │   │ state    │  │ via gRPC │  │ QAT cryptodev    │   │    │
│   │   └──────────┘  └──────────┘  └──────────────────┘   │    │
│   │                                                       │    │
│   │   Shared Memory Region (hugepages):                   │    │
│   │   ┌──────────────────────────────────────────────┐    │    │
│   │   │ DPU writes exception packets here via DMA    │    │    │
│   │   │ Wasm sidecar reads from here                 │    │    │
│   │   │ Zero-copy: DPU DMA → host hugepage → Wasm    │    │    │
│   │   └──────────────────────────────────────────────┘    │    │
│   └──────────────────────────────────────────────────────┘    │
│                                                               │
│ ═══════════════════════ PCIe Bus ════════════════════════════ │
│                                                               │
│   ┌──────────────────────────────────────────────────────┐    │
│   │              BlueField-3 DPU                          │    │
│   │                                                       │    │
│   │   ARM Cores: Exception classifier + DMA engine        │    │
│   │   ┌──────────────────────────────────────────────┐    │    │
│   │   │ Lightweight classifier (ARM):                 │    │    │
│   │   │  • Receives exception pkts from eSwitch       │    │    │
│   │   │  • Quick header check (is DPI needed?)        │    │    │
│   │   │  • DMA payload to host shared memory          │    │    │
│   │   │  • Receives verdict from host                 │    │    │
│   │   │  • Applies verdict (reinject or drop)         │    │    │
│   │   └──────────────────────────────────────────────┘    │    │
│   │                                                       │    │
│   │   eSwitch (ASAP² Hardware):                           │    │
│   │   ┌──────────────────────────────────────────────┐    │    │
│   │   │ Normal TEIDs → HAIRPIN (no CPU)               │    │    │
│   │   │ Exception TEIDs → repr port → ARM classifier  │    │    │
│   │   └──────────────────────────────────────────────┘    │    │
│   │                                                       │    │
│   │   ┌────────┐  ┌────────┐                              │    │
│   │   │ Port 0 │  │ Port 1 │                              │    │
│   │   │ N3     │  │ N6     │                              │    │
│   │   └────────┘  └────────┘                              │    │
│   └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 Data Plane Flow

```
    Packet from RAN (N3)
         │
         ▼
    ┌─────────────────────────┐
    │  eSwitch Match-Action    │
    │                          │
    │  TEID normal → HAIRPIN ──┼──→ Port 1 (N6)  [Bulk Traffic, zero CPU]
    │                          │
    │  TEID exception ─────────┼──→ ARM Core (repr port)
    └─────────────────────────┘
                                         │
                                         ▼
                                ┌────────────────────┐
                                │ ARM Classifier      │
                                │                     │
                                │ Quick header check: │
                                │ "Does this need     │
                                │  full DPI?"         │
                                │                     │
                                │ YES → DMA payload   │
                                │   to host memory    │
                                │                     │
                                │ NO  → reinject via  │
                                │   repr port (light  │
                                │   verdict: TAG only)│
                                └────────┬────────────┘
                                         │
                              PCIe DMA (~1-2μs)
                                         │
                                         ▼
                                ┌────────────────────┐
                                │ Host x86 Wasm Layer │
                                │                     │
                                │ DLB distributes to  │
                                │ Wasm worker cores    │
                                │                     │
                                │ ┌─────────────────┐ │
                                │ │ QAT: decrypt    │ │
                                │ │ (if encrypted)  │ │
                                │ └────────┬────────┘ │
                                │          │          │
                                │ ┌────────▼────────┐ │
                                │ │ WasmEdge (x86)  │ │
                                │ │ AOT + AVX-512   │ │
                                │ │ AMX for ML      │ │
                                │ │                 │ │
                                │ │ Full DPI:       │ │
                                │ │ • Regex scan    │ │
                                │ │ • ML inference  │ │
                                │ │ • Content class │ │
                                │ └────────┬────────┘ │
                                │          │          │
                                │ ┌────────▼────────┐ │
                                │ │ QAT: re-encrypt │ │
                                │ │ (if was encrypted)│
                                │ └────────┬────────┘ │
                                │          │          │
                                │ Verdict + modified  │
                                │ packet written to   │
                                │ shared memory       │
                                └────────┬────────────┘
                                         │
                              PCIe DMA (~1-2μs)
                                         │
                                         ▼
                                ┌────────────────────┐
                                │ ARM Core            │
                                │ Reads verdict       │
                                │                     │
                                │ PASS: reinject via  │
                                │   repr → eSwitch    │
                                │   → Port 1 (N6)    │
                                │                     │
                                │ DROP: free buffers  │
                                └─────────────────────┘
```

---

## 3. Key Design Decisions

### 3.1 Why Two-Stage (ARM → Host) Instead of Direct Host?

The DPU's ARM cores act as a **gatekeeper**:

```
Without ARM gatekeeper:
  ALL exception packets cross PCIe → host
  PCIe bandwidth becomes bottleneck at high exception rates

With ARM gatekeeper:
  ARM does quick header check (~100ns)
  Only packets needing FULL DPI cross PCIe
  Light operations (TAG, METER) handled on ARM directly
  
  Result: PCIe traffic reduced by 50-80%
  Only truly complex packets reach the host x86 cores
```

### 3.2 Shared Memory Design

```
Host Hugepage Region (mapped by both host app and DPU DMA):

  ┌────────────────────────────────────────────────┐
  │  Ring 0: DPU → Host (exception packets)         │
  │  ┌──────┬──────┬──────┬──────┬───────────────┐  │
  │  │Slot 0│Slot 1│Slot 2│ ...  │ Slot N        │  │
  │  │pkt   │pkt   │pkt   │      │               │  │
  │  └──────┴──────┴──────┴──────┴───────────────┘  │
  │                                                  │
  │  Ring 1: Host → DPU (verdicts + modified pkts)   │
  │  ┌──────┬──────┬──────┬──────┬───────────────┐  │
  │  │Slot 0│Slot 1│Slot 2│ ...  │ Slot N        │  │
  │  │verd  │verd  │verd  │      │               │  │
  │  └──────┴──────┴──────┴──────┴───────────────┘  │
  │                                                  │
  │  Doorbell registers for notification             │
  └────────────────────────────────────────────────┘

  Both rings are DPDK rte_ring compatible.
  DPU writes via DOCA DMA engine (not ARM CPU copy).
  Host reads via standard memory access (hugepage-backed).
```

---

## 4. Hitless Plugin Upgrade

Combines DOCA Port State swap (DPU side) with host-side Wasm instance swap.

```
T0: v1 running. ARM classifier → DMA → Host Wasm v1 → verdict.

T1: kubectl apply -f dpi-v2.yaml

T2: Host sidecar:
    • AOT compiles v2 for x86
    • Creates new DLB queue for v2 workers
    • v2 ready but not receiving traffic

T3: Coordinated swap:
    • DPU: DOCA Port State → ACTIVE_READY_TO_SWAP
    • Host: DLB switches new exception packets to v2 queue
    • Both transitions happen within same control message

T4: Drain (100ms):
    • v1 workers finish processing in-flight packets
    • Shared memory rings drain naturally
    • No new packets arrive at v1

T5: Cleanup. Unload v1 on host. Free shared memory slots.
```

---

## 5. Kubernetes Integration

### 5.1 WasmPlugin CRD

```yaml
apiVersion: naas.network/v1alpha1
kind: WasmPlugin
metadata:
  name: ml-anomaly-detector-v3
spec:
  targetSelector:
    matchLabels:
      app: upf-dataplane
      tenant: enterprise-security

  ociRef: registry.example.com/plugins/ml-anomaly@sha256:def456

  # This variant uses host compute
  executionTarget: hybrid       # dpu-arm | host-cpu | hybrid

  # ARM classifier config (lightweight check before DMA)
  armClassifier:
    enabled: true
    lightVerdicts: [TAG, METER]   # ARM handles these directly
    dmaThreshold: "l7-inspection" # Only DMA when L7 needed

  # Host Wasm config
  hostExecution:
    workerCores: 4
    dlbScheduling: ordered
    qatDecrypt: true              # Decrypt before Wasm inspect
    qatReencrypt: true            # Re-encrypt after inspect

  # Resource limits
  fuelBudget: 1000000            # Higher budget for ML workloads
  verdictTimeout: 2ms            # More time for ML inference
  memoryLimit: 8Mi               # Larger for ML model weights

  upgrade:
    strategy: CoordinatedSwap
    drainWindowMs: 200
```

---

## 6. Use Cases

### 6.1 Encrypted Traffic Intelligence

**Scenario:** Enterprise operator must inspect TLS-encrypted traffic for compliance (PCI-DSS, HIPAA) without breaking end-to-end encryption for end users.

```
1. DPU eSwitch identifies exception TEIDs (compliance-flagged subscribers)
2. ARM classifier confirms: encrypted payload, needs L7 inspection
3. DMA to host
4. QAT decrypts TLS (host holds enterprise CA certificate)
5. Wasm plugin scans decrypted payload for:
   • Credit card numbers (PCI-DSS)
   • Patient health records (HIPAA)
   • Data exfiltration patterns
6. QAT re-encrypts
7. Verdict: PASS (clean) or TAG (flag for audit) or DROP (violation)
8. DMA verdict back to DPU → reinject or drop
```

### 6.2 ML-Based Anomaly Detection

**Scenario:** Operator deploys ML model inside Wasm to detect DDoS, botnet C2, or anomalous traffic patterns.

- Wasm plugin loads pre-trained model weights into linear memory (up to 8MB)
- AMX executes INT8 matrix operations for inference (~2000 ops/cycle)
- Model classifies traffic patterns in real-time
- `TAG` verdict with threat-level score attached to packet metadata
- Downstream systems (SIEM, SOC dashboard) consume tagged traffic

### 6.3 Multi-Layer Financial Compliance

**Scenario:** Banking institution running private 5G for trading floor, with regulatory requirement for full traffic audit.

- All trading-related TEIDs marked as exception
- QAT decrypts → Wasm inspects for insider trading patterns, market manipulation signals
- Every packet verdict logged to immutable audit store via URR METER counters
- Plugin updated quarterly for new regulatory rules — zero trading interruption

---

## 7. Latency Budget

```
Component                          Latency
──────────────────────────────────────────────
eSwitch → ARM repr port:          ~0.5μs
ARM classifier (header check):    ~0.1μs
PCIe DMA to host:                 ~1-2μs
DLB dequeue:                      ~0.1μs
QAT decrypt (if needed):          ~2-5μs
Wasm plugin execution:            ~5-20μs
QAT re-encrypt (if needed):       ~2-5μs
PCIe DMA back to DPU:             ~1-2μs
ARM reinject via repr port:       ~0.5μs
──────────────────────────────────────────────
TOTAL (with crypto):              ~12-36μs
TOTAL (without crypto):           ~8-26μs

For comparison:
  Fast-path hairpin:              ~0.5μs
  DPDK software UPF:              ~5-10μs
  
This is acceptable because:
  • DPI inherently requires processing time
  • Alternative (no DPI) means zero inspection
  • Even if exception ratio scales to 10%+, PCIe Gen4 x16 handles ~250Gbps easily.
```

---

## 8. Scalability Under High Inspection Ratios

Unlike Variant A, Variant C can handle high inspection ratios (e.g., 10% to 50% of traffic).
Because the DPU uses DMA to push packets across the PCIe bus, the bottleneck shifts from the wimpy ARM cores to the PCIe bus bandwidth and host CPU.
* A PCIe Gen 4 x16 link can sustain ~250 Gbps. A 100Gbps network pipe where 20% of traffic is sent to the exception path only requires 20 Gbps of PCIe bandwidth — well within limits.
* The host x86 CPUs scale horizontally via DLB to process the incoming DMA streams.

---

## 9. Limitations (Honest)

| Limitation | Impact | Mitigation |
|:---|:---|:---|
| **PCIe round-trip adds ~3-4μs** | Higher total latency than Variant A or B | Only for heavy DPI that ARM can't handle. Light operations stay on ARM |
| **Shared memory coordination** | Complex ring buffer management between DPU and host | Use DPDK rte_ring semantics — well-understood, production-proven |
| **Two failure domains** | DPU or host failure independently affects pipeline | Fail-open: if host unreachable, ARM falls back to PASS verdict |
| **Most complex to deploy** | Requires DPU K8s cluster + host sidecar + shared memory setup | DPF Operator automates most of this. Still most operationally complex variant |
| **Not yet validated E2E** | No published system combines all these components | Individual pieces (DOCA DMA, WasmEdge, DLB, QAT) are all production. Integration is the contribution |

---

## 10. Relationship to Other Variants

```
Complexity:     Variant B (GNR-D) < Variant A (DPU) < Variant C (Hybrid)
Compute Power:  Variant A (ARM)   < Variant B (x86) = Variant C (x86)
Throughput:     Variant B (DPDK)  < Variant A (HW)  = Variant C (HW)
DPI Capability: Variant A (light) < Variant B (full) < Variant C (full + crypto)

Recommendation:
  • Start with Variant B for edge/private 5G (simplest, strongest)
  • Use Variant A for central core when light DPI is sufficient
  • Use Variant C only when encrypted traffic DPI or ML inference is required
  • Original HLD (XDP) remains the research prototype for validation
```
