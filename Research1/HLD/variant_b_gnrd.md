# Variant B: Wasm Plugin Architecture on Intel GNR-D (Xeon 6 SoC)

**Target:** Far-Edge / MEC / Private 5G / Enterprise Appliances  
**Platform:** Intel Xeon 6 SoC (Granite Rapids-D)  
**Status:** All APIs production-ready. Strongest variant.

---

## 1. Why This Variant Exists

In far-edge and private 5G deployments, the UPF runs on compact 1U servers powered by Intel Xeon 6 SoC (GNR-D). These SoCs integrate Ethernet (up to 200GbE), QAT, DLB, DSA, and AMX directly on-die — eliminating the need for external SmartNICs or DPUs.

The UPF data plane typically uses DPDK/VPP on the host CPU. Unlike central-core DPU deployments where packets hairpin in hardware, **here the packets DO touch the CPU** — they flow through DPDK poll-mode drivers. This means there IS a software interception point for Wasm plugins.

### Target Deployments

| Deployment | Example | Why Wasm Plugins Add Value |
|:---|:---|:---|
| **Private 5G campus** | Factory, port, hospital, stadium | Per-tenant DPI rules without vendor involvement |
| **Far-edge MEC** | Street-level cabinets, cell towers | Lightweight custom traffic steering per slice |
| **Enterprise branch** | SD-WAN edge, branch firewall | Hot-swappable IDS/IPS signatures, zero downtime |
| **Open-source 5G** | Open5GS, Free5GC, Aether deployments | Production-grade extensibility for open stacks |

---

## 2. End-to-End Architecture

### 2.1 Kubernetes Cluster Layout

```
┌──────────────────────────────────────────────────────────────┐
│                 Kubernetes Cluster (OpenShift / K3s)           │
│                                                               │
│   ┌─────────────────────────────────────────────────────┐     │
│   │              WasmPlugin Controller (Go)              │     │
│   │                                                      │     │
│   │   Watches: WasmPlugin CRDs                           │     │
│   │   Actions: OCI pull → validate → push to sidecar     │     │
│   │   Runs as: Deployment (1 replica, leader-elected)    │     │
│   └──────────────────────┬──────────────────────────────┘     │
│                          │ gRPC                               │
│                          ▼                                    │
│   ┌─────────────────────────────────────────────────────┐     │
│   │                    UPF Pod                           │     │
│   │                                                      │     │
│   │   Annotations:                                       │     │
│   │     k8s.v1.cni.cncf.io/networks: sriov-n3, sriov-n6│     │
│   │                                                      │     │
│   │   ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │     │
│   │   │Container │  │Container │  │ Container        │  │     │
│   │   │UPF-CP    │  │UPF-DP    │  │ Wasm Sidecar     │  │     │
│   │   │(PFCP)    │  │(DPDK/VPP)│  │ (Rust daemon)    │  │     │
│   │   └──────────┘  └────┬─────┘  └───────┬──────────┘  │     │
│   │                      │                 │              │     │
│   │              Shared hugepage memory region             │     │
│   │                                                      │     │
│   │   Network Interfaces (via Multus + SR-IOV):          │     │
│   │   ┌──────┐  ┌──────┐  ┌──────┐                      │     │
│   │   │ eth0 │  │ net1 │  │ net2 │                      │     │
│   │   │Mgmt  │  │ N3   │  │ N6   │                      │     │
│   │   │(CNI) │  │SR-IOV│  │SR-IOV│                      │     │
│   │   └──────┘  └──────┘  └──────┘                      │     │
│   └─────────────────────────────────────────────────────┘     │
│                                                               │
│   Resources per UPF Pod:                                      │
│   • hugepages-1Gi: 4Gi (DPDK)                                │
│   • openshift.io/sriov_n3: 1 (SR-IOV VF for N3)             │
│   • openshift.io/sriov_n6: 1 (SR-IOV VF for N6)             │
│   • cpu: 8-16 cores (isolated via CPU Manager)               │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 Data Plane Architecture

```
                    GNR-D XEON 6 SoC (SINGLE CHIP)
    ════════════════════════════════════════════════════

    ┌──────────────────────────────────────────────────┐
    │        Integrated NIC (200GbE Ethernet)           │
    │        DDP Profile: GTP-U loaded                  │
    │                                                   │
    │   rte_flow Hardware Classification:               │
    │   ┌──────────────────────────────────────────┐    │
    │   │  Rule 1: TEID ∈ {normal set}             │    │
    │   │          → RSS hash on inner 5-tuple     │    │
    │   │          → Queues 0-7 (UPF DPDK workers) │    │
    │   │                                          │    │
    │   │  Rule 2: TEID ∈ {exception set}          │    │
    │   │          → Queue 8 (Wasm intake)          │    │
    │   │                                          │    │
    │   │  Default: No TEID match                  │    │
    │   │          → Queue 9 (first-pkt / PFCP)    │    │
    │   └──────────────────────────────────────────┘    │
    └───────────────┬──────────────┬────────────────────┘
                    │              │
        ┌───────────▼──────┐  ┌───▼─────────────────────┐
        │  UPF DPDK/VPP    │  │  Wasm Plugin Layer       │
        │  Workers (C0-C7) │  │                          │
        │                  │  │  Queue 8 → DLB eventdev  │
        │  • GTP-U decap   │  │       │                  │
        │  • PDR matching  │  │  ┌────▼────┐ ┌────────┐ │
        │  • FAR forwarding│  │  │Wasm W0  │ │Wasm W1 │ │
        │  • QER policing  │  │  │(Core 8) │ │(Core 9)│ │
        │  • URR counting  │  │  └────┬────┘ └───┬────┘ │
        │                  │  │       └─────┬─────┘      │
        │  99.9% of pkts   │  │             │            │
        │                  │  │      Verdict handler     │
        └──────┬───────────┘  │             │            │
               │              │  PASS → reinject to VPP  │
               │              │  DROP → free mbuf        │
               │              │  TAG  → set QoS mark     │
               │              │  METER→ increment ctr    │
               ▼              └──────────┬───────────────┘
           N6 egress                     │
           (Internet)        Reinject via DPDK internal
                             ring (zero-copy, no veth)
```

---

## 3. Packet Flow (Step by Step)

### 3.1 Fast Path (Bulk Traffic — no Wasm involvement)

```
1. Packet arrives on N3 SR-IOV VF
2. GNR-D integrated NIC parses GTP-U header (DDP profile)
3. rte_flow Rule 1 matches TEID → RSS to Queue 0-7
4. DPDK poll-mode driver on Core 0-7 picks up packet
5. VPP pipeline: GTP-U decap → PDR match → QER → FAR → NAT
6. Packet sent out N6 SR-IOV VF
7. Wasm layer never sees this packet
```

### 3.2 Exception Path (Targeted Traffic — Wasm plugin processing)

```
1. SMF sends PFCP Session Modification:
   "TEID 5001 needs DPI inspection (Enterprise-A policy)"

2. UPF-CP container receives PFCP, translates to:
   rte_flow_create(pattern={GTPU, teid=5001},
                   action={QUEUE, index=8})

3. Subsequent packets with TEID 5001 arrive at Queue 8

4. DLB (hardware) dequeues from Queue 8, distributes to
   available Wasm worker core via eventdev atomic scheduling

5. Wasm worker core:
   a. Receives DPDK mbuf
   b. DSA async-copies payload into Wasm linear memory
      (bounded, hardware-accelerated, non-blocking)
   c. Calls Wasm plugin: inspect(payload_ptr, payload_len)
   d. Plugin executes AOT-compiled code (WasmEdge x86)
   e. Plugin returns verdict (PASS / DROP / TAG / METER)

6. Verdict handler:
   PASS      → reinject mbuf into VPP input node (DPDK ring)
   PASS_MOD  → apply modifications, reinject
   DROP      → rte_pktmbuf_free()
   TAG       → set mbuf ol_flags / QoS mark, reinject
   METER     → increment per-TEID counter, reinject

7. VPP continues normal forwarding pipeline for reinjected pkts
```

### 3.3 First-Packet / Unknown TEID (standard UPF behavior)

```
1. Packet with unknown TEID arrives → no rte_flow match
2. Falls to Queue 9 (default exception queue)
3. UPF-CP does PFCP session lookup, determines rules
4. Programs rte_flow for this TEID (fast-path or exception)
5. Subsequent packets handled by hardware classification
```

---

## 4. Hitless Plugin Upgrade Mechanism

The hitless upgrade replaces BPF map pointer swap with **rte_flow rule update + Wasm instance swap**.

```
Timeline:
═════════════════════════════════════════════════════════

T0: Plugin v1 running on Wasm workers, handling exception TEIDs
    rte_flow rules point exception TEIDs → Queue 8 → DLB → v1

T1: Operator applies: kubectl apply -f dpi-plugin-v2.yaml
    Controller pulls OCI image, validates .wasm binary

T2: Sidecar loads v2 into WasmEdge (AOT compile for x86)
    v2 instance ready but NOT receiving traffic yet

T3: DLB reconfiguration:
    • New eventdev queue created for v2 workers
    • DLB atomic scheduling ensures per-flow ordering

T4: rte_flow update (atomic):
    • rte_flow_destroy(old_rule)
    • rte_flow_create(new_rule → Queue 8 → DLB → v2)
    • These two operations execute in sequence
    • Packets in-flight on old queue drain naturally

T5: Drain window (configurable, default 100ms):
    • v1 continues processing any packets already dequeued
    • No new packets arrive at v1 (rte_flow points to v2)
    • DLB guarantees no reordering during transition

T6: v1 drain complete. Unload v1 Wasm instance.
    Memory freed. Upgrade complete.

Total disruption: Zero packets dropped.
In-flight packets complete on v1, new packets go to v2.
```

---

## 5. GNR-D Integrated Accelerators Used

| Accelerator | Role in This Architecture | API |
|:---|:---|:---|
| **Integrated Ethernet** | GTP-U parsing + flow classification in hardware | DDP profile + `rte_flow` |
| **DLB** (Dynamic Load Balancer) | Distributes exception packets to Wasm workers with hardware-guaranteed per-flow atomic ordering | DPDK `eventdev` PMD |
| **DSA** (Data Streaming Accelerator) | Async payload copy from DPDK mbuf → Wasm linear memory. CPU doesn't block | DPDK `dmadev` / `rte_ioat` |
| **QAT** (QuickAssist Technology) | IPsec decrypt BEFORE Wasm inspection, re-encrypt after. Zero CPU cost for crypto | DPDK `cryptodev` PMD |
| **AMX** (Advanced Matrix Extensions) | ML inference inside Wasm plugins (anomaly detection, traffic classification) | WASI-NN or host function |

---

## 6. Kubernetes Integration Details

### 6.1 WasmPlugin CRD

```yaml
apiVersion: naas.network/v1alpha1
kind: WasmPlugin
metadata:
  name: enterprise-dpi-v2
  namespace: upf-system
spec:
  # Target selection
  targetSelector:
    matchLabels:
      app: upf-dataplane
      tenant: enterprise-A

  # Plugin binary (OCI artifact)
  ociRef: registry.example.com/plugins/dpi-scanner@sha256:abc123

  # Resource limits
  fuelBudget: 500000          # Wasm fuel units per invocation
  verdictTimeout: 500us       # Max wall-clock per packet
  memoryLimit: 2Mi            # Wasm linear memory cap

  # Upgrade behavior
  upgrade:
    strategy: HitlessDrain
    drainWindowMs: 100        # Time for in-flight packets to complete
    rollbackOnFailure: true   # Auto-rollback if health check fails

  # Scheduling
  workerCores: 2              # Number of DLB-distributed workers
  dlbScheduling: atomic       # atomic | ordered | parallel
```

### 6.2 Controller → Sidecar Flow

```
kubectl apply -f dpi-v2.yaml
        │
        ▼
┌───────────────────┐
│ WasmPlugin        │   1. Validates CRD spec
│ Controller (Go)   │   2. Pulls OCI artifact from registry
│                   │   3. Verifies .wasm signature (cosign)
│                   │   4. Resolves target UPF pods
└────────┬──────────┘
         │ gRPC: LoadPlugin(wasm_bytes, config)
         ▼
┌───────────────────┐
│ Wasm Sidecar      │   5. AOT-compiles .wasm for x86
│ (Rust daemon)     │   6. Instantiates WasmEdge module
│ in UPF Pod        │   7. Creates new DLB eventdev queue
│                   │   8. Updates rte_flow rules (atomic)
│                   │   9. Drains old plugin instance
│                   │  10. Reports status back to controller
└───────────────────┘
         │
         ▼
    Controller updates CRD status:
    status:
      phase: Running
      activeVersion: v2
      switchoverLatency: 0.3ms
      lastUpgrade: 2026-05-20T17:00:00Z
```

---

## 7. Use Cases

### 7.1 Private 5G Campus — Factory Floor

**Scenario:** Automotive manufacturer runs private 5G for AGV fleet and quality inspection cameras on a single GNR-D edge server.

- **Slice A (AGVs):** Safety-critical. Wasm plugin enforces strict latency bounds — any packet exceeding 5ms RTT triggers `TAG` verdict for priority re-marking
- **Slice B (Cameras):** High bandwidth. Wasm plugin does content-type detection — blocks non-video traffic (`DROP`), meters bandwidth per camera (`METER`)
- **Upgrade:** New AGV firmware requires updated protocol parsing. Security team pushes new `.wasm` → `kubectl apply` → zero AGV downtime

### 7.2 Enterprise Branch Firewall

**Scenario:** Enterprise SD-WAN edge appliance running on GNR-D, inspecting branch traffic for threats.

- **Tenant A (Finance):** Wasm plugin scans for PCI-DSS violations in HTTP payloads after QAT TLS decrypt
- **Tenant B (Engineering):** Wasm plugin detects exfiltration patterns (large outbound transfers to unknown IPs)
- **Zero-day response:** SOC pushes new signature within minutes. No firewall restart. No maintenance window.

### 7.3 Open-Source 5G MEC (Multi-access Edge Computing)

**Scenario:** University or research lab running Open5GS/Free5GC on GNR-D for edge computing research.

- Multiple student/researcher tenants share the UPF
- Each deploys custom Wasm plugins for their experiments
- Isolation guaranteed — one tenant's buggy plugin cannot crash another's session
- Plugins hot-swapped during live experiments without interrupting data collection

---

## 8. Scalability Under High Inspection Ratios

In macro networks, DPI traffic might be <1%. But in Zero-Trust enterprise environments, **up to 100% of traffic on a specific slice** might require Wasm inspection. 

Variant B handles high inspection ratios gracefully because it is CPU-bound rather than hardware-constrained:
- If exception traffic scales to 10% or 50%, **DLB dynamically load-balances** across the x86 cores.
- Kubernetes Horizontal Pod Autoscaler (HPA) or vertical scaling can dedicate more Xeon P-Cores to the Wasm sidecar.
- Because DSA handles the memory copies asynchronously, the CPU cores are not stalled by high packet volumes.

---

## 9. Limitations (Honest)

| Limitation | Impact | Mitigation |
|:---|:---|:---|
| **Requires control of UPF software** | Cannot inject into proprietary Ericsson/Nokia stacks | Target open-source UPFs, private 5G, enterprise appliances |
| **rte_flow update is not truly atomic** | Brief window (~microseconds) between destroy and create | Use rte_flow `transfer` attribute where supported; drain window covers gap |
| **Wasm adds latency to exception path** | ~5-20μs per packet (AOT Wasm + DSA copy + verdict) | Fast-path unaffected. Latency is acceptable for deep inspection payloads |
| **DLB availability** | DLB is GNR-D / Sapphire Rapids+ only | Fallback to software ring distribution on older hardware |
| **Multi-tenant core isolation** | Wasm workers share CPU with UPF DPDK workers | CPU Manager static policy isolates Wasm cores; cgroup v2 enforcement |
