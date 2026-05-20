# Variant A: Wasm Plugin Architecture on NVIDIA BlueField-3 DPU

**Target:** Central Core / Tier-1 Data Centers / Multi-Tenant MVNO Infrastructure  
**Platform:** NVIDIA BlueField-3 DPU (ARM Cortex-A78 cores + ConnectX-7 eSwitch)  
**Status:** All APIs production-ready. ARM compute budget is the key constraint.

---

## 1. Why This Variant Exists

In central-core data centers, operators deploy DPU-accelerated UPFs where the eSwitch handles 99.9% of forwarding in hardware via hairpin (port-to-port, no CPU). The host CPU runs only PFCP control plane and management.

The challenge: **packets that need DPI never touch the host CPU.** They hairpin inside the DPU. But the DPU has its own ARM cores running Linux — and these cores CAN run custom applications via DOCA SDK.

This variant places the Wasm plugin layer **directly on the DPU's ARM cores**, processing exception-path packets without them ever crossing PCIe to the host.

### Target Deployments

| Deployment | Example | Why Wasm Plugins Add Value |
|:---|:---|:---|
| **Tier-1 central core** | AT&T, Vodafone macro sites | Per-tenant DPI on hardware-offloaded UPF |
| **Multi-tenant MVNO** | Multiple MVNOs sharing UPF infrastructure | Per-MVNO custom billing/inspection plugins |
| **Lawful intercept** | Regulatory compliance (most countries require LI) | Targeted subscriber mirroring via Wasm verdict |
| **CDN / Edge cache** | Content provider traffic classification | Application-level traffic steering to cache |

---

## 2. End-to-End Architecture

### 2.1 Kubernetes Cluster Layout

NVIDIA DPF (DOCA Platform Framework) enables a **dual-cluster architecture**: the host runs one K8s cluster, and the DPU's ARM cores run a separate K8s cluster.

```
┌──────────────────────────────────────────────────────────────┐
│              Host Kubernetes Cluster (x86)                     │
│                                                               │
│   ┌──────────────────────────────────────────────────────┐    │
│   │           WasmPlugin Controller (Go)                  │    │
│   │                                                       │    │
│   │   Watches: WasmPlugin CRDs                            │    │
│   │   Actions: OCI pull → validate → push to DPU sidecar  │    │
│   │   Communicates with DPU cluster via gRPC over PCIe    │    │
│   └───────────────────────┬──────────────────────────────┘    │
│                           │ gRPC (over PCIe / mgmt network)  │
│   ┌───────────────────────┼──────────────────────────────┐    │
│   │  UPF Pod (Host)       │                               │    │
│   │                       │                               │    │
│   │  ┌──────────┐  ┌─────▼──────┐                        │    │
│   │  │ UPF-CP   │  │ DPU Mgmt   │                        │    │
│   │  │ (PFCP)   │  │ Agent      │                        │    │
│   │  │          │  │ (programs  │                        │    │
│   │  │          │  │  DOCA Flow │                        │    │
│   │  │          │  │  via gRPC) │                        │    │
│   │  └──────────┘  └────────────┘                        │    │
│   └──────────────────────────────────────────────────────┘    │
│                                                               │
│ ═══════════════════════ PCIe Bus ════════════════════════════ │
│                                                               │
│   ┌──────────────────────────────────────────────────────┐    │
│   │           BlueField-3 DPU                             │    │
│   │                                                       │    │
│   │   ┌──────────────────────────────────────────────┐    │    │
│   │   │      DPU Kubernetes Cluster (ARM64)           │    │    │
│   │   │      Managed by DPF Operator (DPUCluster CRD)│    │    │
│   │   │                                               │    │    │
│   │   │   ┌─────────────┐  ┌──────────────────────┐  │    │    │
│   │   │   │ OVN-K8s     │  │ Wasm Sidecar         │  │    │    │
│   │   │   │ (DPUService)│  │ (DPUService)         │  │    │    │
│   │   │   │             │  │                      │  │    │    │
│   │   │   │ Standard    │  │ Rust daemon +        │  │    │    │
│   │   │   │ DPU infra   │  │ WasmEdge (ARM64 AOT) │  │    │    │
│   │   │   └─────────────┘  └──────────────────────┘  │    │    │
│   │   └──────────────────────────────────────────────┘    │    │
│   │                                                       │    │
│   │   ┌──────────────────────────────────────────────┐    │    │
│   │   │         eSwitch (ASAP² Hardware)              │    │    │
│   │   │         ConnectX-7 silicon                    │    │    │
│   │   │                                               │    │    │
│   │   │  DOCA Flow Match-Action Tables:               │    │    │
│   │   │  ┌──────────────────────────────────────┐     │    │    │
│   │   │  │ Pipe 1: Normal traffic               │     │    │    │
│   │   │  │   match(teid) → hairpin (no CPU)     │     │    │    │
│   │   │  │                                      │     │    │    │
│   │   │  │ Pipe 2: Exception traffic            │     │    │    │
│   │   │  │   match(teid) → fwd(repr_port_id)   │     │    │    │
│   │   │  │   → ARM core Wasm processing         │     │    │    │
│   │   │  └──────────────────────────────────────┘     │    │    │
│   │   └──────────────────────────────────────────────┘    │    │
│   │                                                       │    │
│   │   Physical Ports:                                     │    │
│   │   ┌────────┐  ┌────────┐                              │    │
│   │   │ Port 0 │  │ Port 1 │                              │    │
│   │   │ N3     │  │ N6     │                              │    │
│   │   │(100GbE)│  │(100GbE)│                              │    │
│   │   └────────┘  └────────┘                              │    │
│   └──────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 Data Plane Architecture

```
    Packet from RAN (N3, Port 0)
         │
         ▼
    ┌─────────────────────────────────────────────┐
    │         eSwitch Hardware Pipeline             │
    │                                               │
    │   DOCA Flow Pipe 1 (Root):                    │
    │   ┌─────────────────────────────────────┐     │
    │   │ Match: GTP-U TEID                   │     │
    │   │                                     │     │
    │   │ TEID ∈ {normal}:                    │     │
    │   │   Action: GTP-U decap               │     │
    │   │           QoS police (MBR)          │     │
    │   │           NAT (if needed)           │     │
    │   │           Counter increment         │     │
    │   │           Hairpin → Port 1 (N6)     │     │
    │   │   Result: ZERO CPU. Wire-speed.     │     │
    │   │                                     │     │
    │   │ TEID ∈ {exception}:                 │     │
    │   │   Action: Forward to repr port      │     │
    │   │           (pf0hpf / pf0vfX)         │     │
    │   │   Result: Packet → ARM core         │     │
    │   └─────────────────────────────────────┘     │
    └───────────────┬──────────────────┬────────────┘
                    │                  │
         BULK HAIRPIN            TARGETED EXCEPTION
              │                        │
              ▼                        ▼
         Port 1 (N6)         ARM Core (Wasm Sidecar)
         (Internet)          ┌─────────────────────┐
                             │ DPDK on repr port   │
                             │ rte_eth_rx_burst()   │
                             │       │              │
                             │ WasmEdge (ARM64 AOT) │
                             │ ┌───────────────┐    │
                             │ │ Tenant Plugin  │    │
                             │ │ .wasm (ARM64)  │    │
                             │ │               │    │
                             │ │ inspect() →   │    │
                             │ │ verdict       │    │
                             │ └───────┬───────┘    │
                             │         │            │
                             │  PASS: rte_eth_tx    │
                             │    via repr port     │
                             │    → eSwitch egress  │
                             │    → Port 1 (N6)     │
                             │                      │
                             │  DROP: free mbuf     │
                             └──────────────────────┘
```

---

## 3. Packet Flow (Step by Step)

### 3.1 Fast Path (Bulk Traffic — pure hardware, zero CPU)

```
1. Packet arrives on Port 0 (N3, 100GbE)
2. eSwitch parses GTP-U header in silicon
3. DOCA Flow Pipe 1 matches TEID in TCAM/flow table
4. Hardware executes action chain:
   a. GTP-U decapsulation (strip outer headers)
   b. QoS policing (check MBR, drop if exceeded)
   c. NAT translation (if FAR specifies)
   d. URR counter increment (hardware counter)
   e. Hairpin forward → Port 1 (N6)
5. Packet exits Port 1. Never touched any CPU.
```

### 3.2 Exception Path (Targeted Traffic — ARM core + Wasm)

```
1. UPF-CP (on host) receives PFCP from SMF:
   "TEID 5001 requires DPI per Enterprise-A policy"

2. DPU Mgmt Agent programs DOCA Flow:
   doca_flow_pipe_add_entry(
     pipe: exception_pipe,
     match: {gtp_teid: 5001},
     fwd: {type: PORT, port_id: repr_port_id}
   )

3. Subsequent TEID 5001 packets → repr port → ARM core

4. Rust daemon on ARM core:
   a. rte_eth_rx_burst(repr_port) → receives mbuf
   b. Copies payload into Wasm linear memory (bounded)
   c. Calls Wasm plugin: inspect(payload_ptr, len)
   d. WasmEdge executes AOT-compiled ARM64 code
   e. Plugin returns verdict

5. Verdict handling:
   PASS  → rte_eth_tx_burst(repr_port) → eSwitch → Port 1
   DROP  → rte_pktmbuf_free(mbuf)
   TAG   → modify mbuf metadata, reinject
   METER → increment per-TEID counter, reinject

6. eSwitch receives reinjected packet on repr port
7. Egress pipeline forwards to Port 1 (N6)
```

---

## 4. Hitless Plugin Upgrade Mechanism

BlueField DOCA Flow provides a native **Port Operation State** mechanism for hitless transitions. This directly replaces the BPF map pointer swap from the original HLD.

```
DOCA Flow Port Operation States:
  • ACTIVE                    — currently handling traffic
  • STANDBY                   — ready to take over
  • ACTIVE_READY_TO_SWAP      — transitioning out

Upgrade Sequence:
═══════════════════════════════════════════════════

T0: Plugin v1 is ACTIVE on ARM cores.
    DOCA Flow exception entries point to v1's repr port config.

T1: kubectl apply -f dpi-v2.yaml
    Controller pulls OCI image, validates, pushes to DPU sidecar.

T2: Sidecar loads v2 into WasmEdge on ARM core.
    AOT-compiles v2 for ARM64. v2 instance ready.
    New DOCA Flow instance created in STANDBY state:
      doca_flow_port_start(port, DOCA_FLOW_PORT_OPERATION_STATE_STANDBY)

T3: Atomic handover:
    a. Set v1 port state → ACTIVE_READY_TO_SWAP
    b. Set v2 port state → ACTIVE
    c. eSwitch atomically redirects new exception packets to v2

T4: Drain window (100ms default):
    v1 processes any packets already in its rx queue.
    No new packets arrive at v1.

T5: v1 drain complete. Unload v1 Wasm instance.
    doca_flow_port_stop(v1_port). Memory freed.

Result: Zero dropped packets. Hitless.
```

---

## 5. Kubernetes Integration — DPU-Specific

### 5.1 DPUService for Wasm Sidecar

The Wasm sidecar runs as a `DPUService` in the DPU's own K8s cluster:

```yaml
apiVersion: svc.dpu.nvidia.com/v1alpha1
kind: DPUService
metadata:
  name: wasm-plugin-sidecar
  namespace: dpf-operator-system
spec:
  helmChart:
    source:
      repoURL: https://registry.example.com/charts
      path: wasm-sidecar
      version: "1.0.0"
  serviceConfiguration:
    serviceDaemonSet:
      nodeSelector:
        dpu-type: bluefield-3
  interfaces:
    - name: repr-exception
      network: exception-traffic-net
```

### 5.2 WasmPlugin CRD (Host Cluster)

```yaml
apiVersion: naas.network/v1alpha1
kind: WasmPlugin
metadata:
  name: enterprise-dpi-v2
  namespace: upf-system
spec:
  targetSelector:
    matchLabels:
      app: upf-dataplane
      tenant: enterprise-A

  # Plugin binary
  ociRef: registry.example.com/plugins/dpi@sha256:abc123

  # Execution target
  executionTarget: dpu-arm    # dpu-arm | host-cpu | hybrid

  # Resource limits
  fuelBudget: 500000
  verdictTimeout: 500us
  memoryLimit: 2Mi

  # Upgrade
  upgrade:
    strategy: DOCAPortStateSwap
    drainWindowMs: 100
```

### 5.3 Control Flow

```
Host Cluster                          DPU Cluster
────────────                          ───────────
kubectl apply                              │
     │                                     │
WasmPlugin Controller                      │
     │ validates CRD                       │
     │ pulls OCI artifact                  │
     │ resolves target DPU                 │
     │                                     │
     │─── gRPC: LoadPlugin(bytes,cfg) ────▶│
     │                                     │
     │                               Wasm Sidecar (DPUService)
     │                                     │ AOT compile for ARM64
     │                                     │ Create STANDBY instance
     │                                     │ DOCA Port State swap
     │                                     │ Drain old instance
     │                                     │
     │◀── gRPC: PluginStatus(running) ─────│
     │                                     │
Controller updates CRD status              │
```

---

## 6. Use Cases

### 6.1 Multi-Tenant MVNO DPI

**Scenario:** Infrastructure operator hosts UPF for three MVNOs on shared BlueField-3 DPU hardware.

- **MVNO-A:** Deploys content-filtering Wasm plugin (blocks gambling sites for family plans)
- **MVNO-B:** Deploys bandwidth-metering Wasm plugin (per-app usage for tiered billing)
- **MVNO-C:** No DPI needed — all traffic hairpins in hardware

Each MVNO's exception TEIDs are isolated. MVNO-A's plugin runs in its own Wasm instance with independent linear memory. MVNO-B cannot access MVNO-A's packet data — enforced by Wasm sandbox.

### 6.2 Lawful Intercept

**Scenario:** Regulatory requirement to mirror specific subscriber traffic to LEA (Law Enforcement Agency) probe.

- SMF marks target subscriber's TEIDs for exception processing
- Wasm plugin copies matching packets to a mirror port (via `TAG` verdict + eSwitch mirror rule)
- Plugin is hot-swappable — target list updates require no UPF restart
- Audit trail: every plugin load/unload logged via K8s events

### 6.3 CDN Traffic Steering at Core

**Scenario:** Content provider deploys Wasm plugin to classify video traffic by resolution.

- 4K traffic → `TAG` with high-priority QoS class → premium CDN edge
- SD traffic → `PASS` with standard QoS → regular path
- Plugin updated when new streaming codec is deployed — zero downtime

---

## 7. Scalability Under High Inspection Ratios (The ARM Constraint)

In scenarios where the exception path is not 1% but **10% or more** (e.g., Enterprise Zero-Trust policies requiring deep inspection on many flows), **Variant A will bottleneck**.

The BlueField-3 ARM Cortex-A78 cores cannot sustain line-rate Deep Packet Inspection for large volumes of traffic. If 10% of a 100Gbps pipe is steered to the ARM cores, they will saturate, causing dropped packets. 
* **Conclusion:** This variant is strictly limited to deployments where DPI traffic is a small percentage of overall throughput, or where the Wasm plugins perform very lightweight operations (e.g., simple header tagging). For high-volume DPI, Variant C (Hybrid) is required.

---

## 8. Limitations (Honest)

| Limitation | Impact | Mitigation |
|:---|:---|:---|
| **ARM cores are "wimpy"** | Cortex-A78 is ~3-4× slower than Xeon P-core for single-threaded Wasm | Only lightweight plugins (pattern match, tag, meter). Heavy compute → Variant C |
| **ARM core contention** | DPU runs OVN-K8s, storage, telemetry on same ARM cores | CPU pinning via DPU cgroup. Dedicate 2-4 cores to Wasm, leave rest for infra |
| **Limited memory** | BlueField-3 has 16-32GB DDR5 on-board (shared with OS + infra) | Wasm plugins are tiny (~100KB). Linear memory capped at 2MB per instance |
| **gRPC over PCIe latency** | Host → DPU control path adds ~100-500μs per API call | Only affects control plane (plugin load/unload), not data path |
| **DPU vendor lock-in** | Architecture tied to NVIDIA DOCA APIs | Plugin ABI (Wasm) is vendor-agnostic. Only the steering layer is DOCA-specific |
