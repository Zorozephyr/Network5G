# NaaS 5G UPF — High-Level Design Document v2.0

**Title**: Intent-Driven Orchestration of Hybrid eBPF/WebAssembly Data Planes for 5G User Plane Functions

**Version**: 2.0 — April 2026
**Status**: Pre-Implementation Design (Phase 1 Prototype Scope)

---

## 1. Abstract

This document specifies the architecture of a **Network-as-a-Service (NaaS)** framework that enables telecom operators and enterprise tenants to dynamically inject custom packet-processing logic into a live 5G User Plane Function (UPF) — without pod restarts, without vendor coordination, and without dropping active GTP-U sessions.

The system combines three technologies at a novel intersection:

1. **eBPF/XDP** for ultra-fast in-kernel GTP-U packet forwarding (the fast path, handling ~99% of traffic)
2. **WebAssembly (Wasm)** for sandboxed, tenant-supplied Deep Packet Inspection (DPI) and billing plugins (the exception path)
3. **A Kubernetes Operator** for intent-driven plugin lifecycle management via Custom Resource Definitions (CRDs)

The central research contribution is a **hitless plugin upgrade mechanism** that achieves sub-millisecond switchover via RCU-protected atomic BPF map pointer swap, combined with a drain cycle for in-flight packets. This enables live replacement of Wasm plugins on active 5G sessions with zero packet loss on the fast path.

---

## 2. Problem Statement and Motivation

### 2.1 The Industry Problem

Standard 5G UPFs have rigid, vendor-controlled data paths. Adding custom logic — a new DPI rule, a billing metric for a novel IoT protocol, or a security patch — requires:

- Waiting months for the vendor to release a new binary
- Restarting UPF pods to deploy it, dropping thousands of active sessions
- Paying the vendor for each customisation

This vendor lock-in is the single largest operational pain point for telecom operators running cloud-native 5G cores.

### 2.2 What This Architecture Solves

With this architecture, a telecom operator can:

1. Write custom DPI or billing logic in Rust/Go/C++
2. Compile it to a `.wasm` binary (~100KB)
3. Declare an intent via Kubernetes: `kubectl apply -f enterprise-dpi-v2.yaml`
4. The system automatically fetches the binary, injects it into the running UPF pod, updates the eBPF kernel maps, and begins routing targeted traffic to the new plugin — **in milliseconds, without a single dropped packet**

### 2.3 Research Questions

| ID | Question | Validation Method |
|----|----------|-------------------|
| **RQ1** | What is the per-packet latency overhead of the eBPF → AF_XDP → Wasm → veth handoff for exception-path packets? | Measure at P50/P99/P99.9 for 64B, 512B, 1500B payloads |
| **RQ2** | Can BPF map pointer swap achieve sub-millisecond hitless plugin switchover with zero fast-path packet loss? | Inject plugin upgrade during steady-state traffic; measure switchover time and verify zero drops |
| **RQ3** | Does fuel-metered Wasm provide sufficient protection against misbehaving plugins without unacceptable overhead on well-behaved ones? | Compare throughput of fuel-metered vs. unlimited execution for reference DPI plugins |

---

## 3. Prior Art and Positioning

### 3.1 Established Prior Art (Not Claimed as Novel)

| Technology | Key Examples | Our Usage |
|-----------|-------------|-----------|
| eBPF/XDP for 5G UPF fast-path | eUPF (edgecomllc/eupf), Cable (free5GC), NgKore | We adopt eBPF/XDP as an established acceleration layer |
| Wasm on Kubernetes | SpinKube, KWasm, runwasi, Envoy Wasm filters | We leverage Wasm as a known sandboxing technology |
| Wasm managing eBPF | wasm-bpf (Zheng et al., 2024), Eunomia-bpf | We invert the relationship: eBPF feeds Wasm |
| eBPF shared-memory processing | SPRIGHT (Qi et al., SIGCOMM 2022/2024) | We draw on shared-memory techniques for AF_XDP |
| Wasm in O-RAN | WA-RAN (HotNets '24) | Different domain (RAN RIC, not UPF payload processing) |
| Atomic BPF replacement | `bpf_link_update()`, BPF Map Tracing (Burton, Google) | We extend the mechanism to Wasm plugin lifecycle |

### 3.2 Novel Contributions

| Contribution | Nearest Prior Art | What We Add |
|-------------|-------------------|-------------|
| **Hitless BPF map pointer swap for Wasm plugin lifecycle** | BPF Map Tracing (Burton) — stateful program migration; Envoy — listener drain (~seconds) | First to combine RCU-protected per-TEID map swap with Wasm dual-instance drain cycle for 5G UPF. Sub-millisecond switchover |
| **Telecom NaaS plugin ABI** (PASS / PASS_MOD / DROP / TAG / METER) | Proxy-Wasm SDK — HTTP streams; WA-RAN — RAN-specific interfaces | First tenant-facing Wasm ABI for in-band GTP-U L7 payload processing with header-transparent execution |
| **K8s CRD-driven operator for UPF plugin management** | Istio WasmPlugin CRD — Envoy filters; NetEdit (Meta) — eBPF fleet orchestration | First to combine CRD intent → OCI pull → Wasm injection → BPF map update on live GTP-U traffic |
| **Fail-open fault model for telecom Wasm** | No prior work | Fuel metering + watchdog timeout + AF_XDP backpressure as combined fail-open strategy for carrier-grade sessions |

### 3.3 Differentiation from Proxy-Wasm (Envoy / Istio)

Istio's `WasmPlugin` CRD and Envoy's Proxy-Wasm ABI represent the closest architectural analogue in the service mesh domain. The differentiation is fundamental, not incremental:

| Dimension | Proxy-Wasm (Envoy/Istio) | This Work |
|-----------|--------------------------|-----------|
| Interception point | L7 HTTP proxy (user-space) | L2/L3 XDP (kernel, pre-stack) |
| Plugin target | HTTP request/response streams | Raw GTP-U encapsulated L7 payloads |
| Upgrade mechanism | Envoy listener drain (~seconds) | RCU-protected BPF map swap (~nanoseconds) |
| CRD scope | Envoy sidecar filter chain | eBPF kernel maps + Wasm runtime + OCI lifecycle |
| Protocol awareness | HTTP/gRPC | GTP-U/TEID |

Proxy-Wasm cannot inspect GTP-U tunneled traffic. This work does not target HTTP. The two systems are complementary, not competing.

### 3.4 Why eBPF/XDP Over DPDK

| Criterion | eBPF/XDP | DPDK |
|-----------|----------|------|
| CPU allocation | Runs within kernel; no dedicated cores | Requires exclusive CPU core binding (poll-mode driver) |
| K8s integration | Compatible with standard K8s pod scheduling | Conflicts with K8s resource management; needs device plugins |
| Map accessibility | BPF maps accessible from both kernel and user-space, enabling atomic pointer swap | Requires custom IPC for state sharing |
| UPF modification | Augments existing UPFs (e.g., Open5GS) without modification | Requires fully user-space UPF implementation |
| NIC binding | No exclusive driver binding | Requires NIC takeover (no shared access) |

eBPF/XDP enables the atomic BPF map pointer swap mechanism that is central to this work's hitless upgrade contribution. DPDK would preclude this.

---

## 4. System Architecture

The architecture is organised into four layers:

```
┌───────────────────────────────────────────────────────────────────────┐
│                       Layer 4: Kubernetes Control Plane               │
│   Central Go Controller ──watches──▶ WasmPlugin CRDs                 │
│   Per-pod Sidecar Agent ──watches──▶ Pod annotations (downward API)  │
│   OCI Registry ──pulls──▶ .wasm binaries                             │
├───────────────────────────────────────────────────────────────────────┤
│                       Layer 3: Wasm Sandbox                          │
│   WasmEdge Runtime (AOT mode)                                        │
│   Per-slice isolated instances with independent linear memory        │
│   Plugin ABI: (payload_ptr, payload_len, teid, timestamp) → Verdict  │
├───────────────────────────────────────────────────────────────────────┤
│                       Layer 2: Rust Daemon + AF_XDP                  │
│   AF_XDP zero-copy socket (shared UMEM with kernel)                  │
│   Bounded payload copy into Wasm linear memory (safety isolation)    │
│   Software UDP checksum recalculation before veth reinjection        │
├───────────────────────────────────────────────────────────────────────┤
│                       Layer 1: eBPF/XDP (Kernel)                     │
│   GTP-U header parsing + TEID extraction                             │
│   BPF_MAP_TYPE_HASH lookup: TEID → {slice_id, plugin_id}            │
│   Fast path: XDP_PASS (99% traffic)                                  │
│   Exception path: XDP_REDIRECT to per-slice AF_XDP socket            │
└───────────────────────────────────────────────────────────────────────┘
```

### 4.1 Layer 1 — eBPF/XDP (Kernel Fast Path)

**Location**: Attached to the XDP hook on the UPF pod's N3-facing vNIC, at the driver level, before `sk_buff` allocation.

**Behaviour**:

1. Intercept every inbound packet at driver level (pre-stack)
2. Parse GTP-U header, extract TEID (O(1) from fixed offset)
3. Lookup TEID in `BPF_MAP_TYPE_HASH` exception map
4. **No match** → `XDP_PASS`. Packet enters normal UPF processing pipeline (PFCP rule enforcement, N6 routing). This is the fast path (~99% of traffic)
5. **Match found** → Read `{slice_id, plugin_id}` from map value. Compute `payload_offset` and `payload_length`. Prepend 8-byte metadata header to frame. `XDP_REDIRECT` to the AF_XDP socket corresponding to `slice_id`

**Key data structures**:

```c
// Exception map — populated by Sidecar Agent or CLI tool
struct bpf_map_def exception_map = {
    .type        = BPF_MAP_TYPE_HASH,
    .key_size    = sizeof(__u32),         // TEID
    .value_size  = sizeof(struct teid_route), // {slice_id, plugin_id, flags}
    .max_entries = 65536,
};

// Blocklist map — populated by Rust daemon on DROP verdict
struct bpf_map_def blocklist_map = {
    .type        = BPF_MAP_TYPE_HASH,
    .key_size    = sizeof(__u32),         // TEID
    .value_size  = sizeof(__u8),          // 1 = blocked
    .max_entries = 65536,
};

// 8-byte metadata header prepended to redirected frames
struct exception_meta {
    __u32 teid;
    __u16 payload_offset;
    __u16 payload_length;
};
```

### 4.2 Layer 2 — Rust Daemon + AF_XDP

**Location**: User-space process within the UPF pod, sharing the pod's network namespace.

**Reception (kernel → user-space zero-copy)**:
- AF_XDP socket binds to the XDP-redirected queue
- Shared UMEM region: kernel and Rust daemon share the same physical memory pages
- No `sk_buff` allocation, no socket buffer copy — true zero-copy from NIC to user-space

**Processing**:
1. Read 8-byte metadata header from frame → extract `teid`, `payload_offset`, `payload_length`
2. Compute payload slice pointer within the AF_XDP UMEM frame
3. **Copy payload bytes into Wasm linear memory** (bounded copy: `payload_length` bytes)
4. Call Wasm entry point: `process_packet(payload_ptr, payload_len, teid, timestamp)`
5. Read verdict from Wasm return value

**Resubmission (on PASS / PASS_MOD verdict)**:
1. On PASS_MOD: copy modified payload bytes back from Wasm linear memory to the original frame at `payload_offset`
2. **Strip 8-byte metadata header** — the frame written to the TX ring contains only the original Ethernet frame with modifications applied
3. **Recalculate UDP checksum** in software over the modified payload (required because the veth reinjection path uses a virtual device that does not support hardware checksum offload via `CHECKSUM_PARTIAL`)
4. GTP-U, UDP, and IP headers are intact and unmodified (except UDP checksum)
5. Write frame to AF_XDP TX ring → kernel submits frame into UPF namespace via veth pair
6. UPF processes packet normally on N6 path

**On DROP verdict**:
1. Call `bpf_map_update_elem()` to add TEID to eBPF blocklist map
2. Discard current packet
3. All future packets from this TEID hit `XDP_DROP` before reaching the AF_XDP socket

**Zero-copy framing**: The data path achieves kernel-to-userspace zero-copy via AF_XDP shared UMEM. A bounded payload copy (`2 × payload_len` bytes per exception-path packet) is performed at the Wasm boundary to preserve Wasm linear memory isolation. This is a deliberate safety/performance trade-off: the copy overhead is the measurable cost of guaranteeing that a tenant's plugin cannot corrupt the shared packet buffer or adjacent packets in the UMEM ring.

**Threading model**: The threading model of the Rust daemon is implementation-defined; multi-threaded dispatch with per-thread Wasm instances is the expected production configuration, following the pattern established by Envoy's per-worker-thread Wasm VM isolation. The Phase 1 prototype uses a single-threaded worker bound to a single AF_XDP RX queue to isolate variables for per-packet latency measurement and hitless switchover validation. Multi-queue scalability via RSS-mapped per-queue AF_XDP socket pools with shared UMEM is documented as future work.

### 4.3 Layer 3 — Wasm Sandbox

**Runtime**: WasmEdge in **Ahead-of-Time (AOT) compiled mode**. The 500μs execution timeout is calibrated against AOT-compiled Wasm, not interpreted execution. AOT compilation occurs during the warm-up phase (Step 2 of the hitless upgrade flow) and does not affect per-packet processing latency.

**Plugin ABI**:

The plugin receives:

| Input | Type | Access |
|-------|------|--------|
| Pointer to L7 payload bytes | `*mut u8` | Read/Write |
| Payload length | `u32` | Read-only |
| TEID of current session | `u32` | Read-only |
| Monotonic timestamp | `u64` (nanoseconds) | Read-only |

Nothing else. No syscalls. No network access. No filesystem. No other memory regions.

**Plugin verdicts**:

| Verdict | Code | Effect |
|---------|------|--------|
| PASS | 0 | Resubmit the frame unchanged |
| PASS_MOD | 1 | Resubmit the frame with the L7 payload as modified in-place by the plugin |
| DROP | 2 | Instruct the Rust daemon to add this TEID to the eBPF blocklist. Current packet is discarded |
| TAG | 3 | Write a 1-byte tag value into the GTP-U extension header before resubmission. Used for QoS marking and billing probes |
| METER | 4 | Increment a per-TEID byte counter in a BPF array map. No packet modification |

**Safety constraints**:

1. **Fuel metering**: Each invocation is budgeted a fixed instruction count. Exceeding the budget returns PASS (fail-open) and increments a telemetry counter
2. **Memory isolation**: Wasm linear memory is allocated separately from the AF_XDP UMEM. Payload bytes are copied in on entry and copied back on PASS_MOD exit. A buggy plugin cannot corrupt the shared packet buffer
3. **Execution timeout**: A watchdog timer in the Rust daemon releases the current packet as PASS and logs an anomaly if a Wasm invocation exceeds 500 microseconds

### 4.4 Layer 4 — Kubernetes Control Plane

The control plane uses a Master/Sidecar split to separate cluster-wide intent from node-level execution.

**WasmPlugin CRD**:

```yaml
apiVersion: naas.network/v1alpha1
kind: WasmPlugin
metadata:
  name: enterprise-dpi-v2
spec:
  targetSelector:
    upfPodLabel: "tenant=enterprise-A"
  ociRef: registry.example.com/plugins/dpi@sha256:abc123
  fuelBudget: 500000
  verdictTimeout: 500us
  drainWindowMs: 100
```

**Central Go Controller**:
- Watches all `WasmPlugin` CRDs cluster-wide
- On CRD create/update: identifies target UPF pods via label selector
- Patches each target pod's metadata annotation with desired state: `naas.plugin/desired: enterprise-dpi-v2@sha256:abc123`
- Does **not** perform any data-plane operations — it only writes to the K8s annotation plane

**Per-pod Sidecar Agent**:
- Runs as a lightweight Go container in every UPF pod, sharing the pod's network namespace
- Watches its own pod's annotations via the downward API (no API server polling)
- On annotation change: pulls the `.wasm` binary from the OCI registry, validates the SHA256 digest, then triggers the upgrade sequence via gRPC to the Rust daemon over a Unix Domain Socket
- Reports completion by patching the pod annotation: `naas.plugin/active: enterprise-dpi-v2@sha256:abc123`

---

## 5. Multi-Tenant Isolation Model

Multi-tenant isolation is achieved through **per-slice Wasm instantiation**.

```
┌───────────────────────────────────────────────────────────────┐
│                          UPF Pod                              │
│                                                               │
│  eBPF XDP Program                                             │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │  TEID Map:  Key = TEID → Value = {slice_id, plugin_id} │  │
│  └──────────────────────┬──────────────────────────────────┘  │
│                         │ XDP_REDIRECT per slice_id           │
│             ┌───────────┴───────────┐                         │
│             ▼                       ▼                         │
│       ┌───────────┐          ┌───────────┐                    │
│       │  AF_XDP   │          │  AF_XDP   │  (per-slice)       │
│       │  Slice A  │          │  Slice B  │                    │
│       └─────┬─────┘          └─────┬─────┘                    │
│             ▼                       ▼                         │
│       ┌───────────┐          ┌───────────┐                    │
│       │   Wasm    │          │   Wasm    │  (isolated linear  │
│       │  Runtime  │          │  Runtime  │   memory per slice)│
│       │ (DPI v2)  │          │(Billing)  │                    │
│       └───────────┘          └───────────┘                    │
│                                                               │
│       Rust Daemon (manages all instances)                     │
└───────────────────────────────────────────────────────────────┘
```

**Why this works**:

1. **Automatic isolation**: Each slice gets its own Wasm runtime instance with independent linear memory. Cross-tenant data leakage is impossible at the Wasm level — a tenant's plugin literally cannot address another tenant's memory
2. **Maps to 5G natively**: Network slices are the natural 3GPP multi-tenancy boundary. TEIDs already belong to specific slices. We are not inventing a new isolation primitive — we are mapping Wasm instances 1:1 to the isolation boundary that 3GPP already defines
3. **Free eBPF routing**: The BPF map value extends from `{plugin_id}` to `{slice_id, plugin_id}`. eBPF uses `slice_id` to redirect to the correct AF_XDP socket — a single extra field, zero additional kernel complexity
4. **Per-slice hitless upgrades**: Upgrading Slice A's plugin has zero impact on Slice B's processing. The drain cycle is scoped to a single slice's TEID set

Phase 1 validates with a single slice. Multi-slice concurrent isolation is an architectural guarantee validated in Phase 2.

---

## 6. Hitless Plugin Upgrade Flow

This is the central mechanism of the paper. The goal is to transition from plugin v1 to plugin v2 without any active GTP-U session experiencing a dropped packet or a processing gap.

| Step | Detail |
|------|--------|
| **1. OCI Pull** | Sidecar pulls `v2.wasm` from the OCI registry into the shared EmptyDir volume. SHA256 digest is verified. v1 continues serving all traffic normally |
| **2. Warm-up + AOT** | Sidecar sends `load_plugin(v2_path)` to the Rust daemon. The daemon instantiates v2 in the Wasm runtime, performs AOT compilation, and assigns it a new, non-overlapping memory region. v2 is idle: no traffic flows through it |
| **3. Readiness check** | Rust daemon calls a `wasm_health_check()` export on v2 with a synthetic payload. If v2 returns a non-PASS verdict on the test, the upgrade is aborted. v1 continues. The Sidecar records a K8s event |
| **4. Atomic BPF pointer swap** | Rust daemon calls `bpf_map_update_elem()` updating the per-TEID value from `v1_instance_id` to `v2_instance_id`. This is serialised by kernel RCU locks and completes in a single CPU cycle. The **very next packet** is dispatched to v2 |
| **5. Drain cycle** | v1 remains alive for `drainWindowMs` (default 100ms). In-flight invocations are tracked via an atomic counter in the Rust daemon. Once the counter reaches zero and the window has elapsed, v1 is unloaded |
| **6. Status update** | Rust daemon notifies the Sidecar via gRPC. Sidecar patches the pod annotation. Central Controller reconciles and marks `WasmPlugin` CRD as `Ready` |

**Configuration constraint**:
```
drainWindowMs > P99.9(fuelBudget execution time for max-size GTP-U payload)
```

The default 100ms is calibrated assuming AOT-compiled DPI execution at P99.9 < 50μs, providing a 2000× safety margin. This ensures in-flight v1 invocations complete within the drain window.

**Known limitation — drain timeout race**: If a v1 Wasm invocation takes longer than `drainWindowMs` (e.g., a large payload that gracefully exhausts its fuel budget), v1 is force-killed after the window expires. At most one packet per affected session may be processed by neither v1 nor v2. The window is bounded to `drainWindowMs` and affects only the exception-path ~1% of traffic. The 99% fast-path (`XDP_PASS`) is unaffected throughout the upgrade.

---

## 7. End-to-End Packet Flow

### 7.1 Fast Path (99% of traffic)

```
1. Packet arrives at NIC from RAN (N3 interface, GTP-U encapsulated)
2. eBPF XDP program intercepts at driver level, before sk_buff allocation
3. TEID extracted from GTP-U header → O(1) lookup in BPF_MAP_TYPE_HASH
4. No match found → XDP_PASS returned immediately
5. Packet enters normal UPF processing pipeline (PFCP rule enforcement, N6 routing)
```

**Latency**: Near-zero additional overhead. XDP operates at the driver level.

### 7.2 Exception Path — PASS_MOD Verdict (Inline Modification)

```
 1. TEID match found in exception map
 2. eBPF computes payload_offset and payload_length
 3. eBPF prepends 8-byte metadata header to frame
 4. XDP_REDIRECT → AF_XDP socket (zero-copy into Rust daemon's UMEM)
 5. Rust daemon reads metadata header, computes payload slice pointer
 6. Payload bytes copied into Wasm linear memory (bounded copy)
 7. Rust daemon calls Wasm entry: process_packet(ptr, len, teid, ts)
 8. Wasm plugin executes tenant logic, modifies payload bytes, returns PASS_MOD
 9. Rust daemon copies modified payload back into original frame at payload_offset
10. 8-byte metadata header STRIPPED from frame
11. UDP checksum recalculated in software
12. Frame written to AF_XDP TX ring
13. Kernel submits frame into UPF namespace via veth pair
14. UPF processes packet normally on N6 path
```

**Known overhead**: The veth reinjection (step 13) traverses the kernel network stack on the receiving end (qdisc, netfilter, IP routing). This adds ~2–5μs per packet. This is a bounded, measured cost that will be reported in the evaluation.

```
Full packet journey:
NIC → XDP (pre-stack) → AF_XDP → Wasm → XSK TX → veth TX → kernel stack → UPF
                                                              ^^^^^^^^^^^^^^^^
                                                              ~2-5μs overhead
```

### 7.3 Exception Path — DROP Verdict

Steps 1–7 as above. Wasm module returns DROP.

```
8. Rust daemon calls bpf_map_update_elem() to add TEID to eBPF blocklist map
9. Current packet is discarded
10. All future packets from this TEID hit XDP_DROP before reaching the AF_XDP socket
```

---

## 8. Fault Management and Graceful Degradation

**Design principle**: Every failure mode either **fails open** (preserving the 5G session at the cost of telemetry) or **fails safe** (preserving data-plane state via BPF map persistence). No failure mode should cause an active session to drop.

| Failure Mode | System Behaviour | Session Impact |
|-------------|-----------------|----------------|
| **Wasm panic** | Runtime catches the trap. Rust daemon releases frame as PASS. If AF_XDP buffer exceeds 80% fill, all matched TEIDs flipped to XDP_PASS via BPF map flag. Telemetry lost. Sessions survive | Fail open. Session preserved |
| **Fuel exhaustion** | Runtime returns fuel-exceeded error. Rust daemon treats as PASS. Telemetry counter incremented | Fail open. Session preserved |
| **Execution timeout** (>500μs) | Watchdog fires. Current packet released as PASS. Anomaly logged | Fail open. Session preserved |
| **Rust daemon crash** | AF_XDP socket closes. eBPF checks health flag in `BPF_MAP_TYPE_ARRAY` on each exception-path packet. If flag is unset, packets fall through to XDP_PASS. Sidecar detects death via gRPC heartbeat and restarts daemon. BPF maps are pinned to bpffs and survive the restart | Fail open. Session preserved |
| **Sidecar crash** | No active-path impact. Rust daemon continues serving last known plugin. K8s restarts Sidecar. On restart, Sidecar reads current annotations and reconciles | No data-plane impact |
| **Central Controller crash** | No active-path impact. BPF maps persist. K8s reschedules Controller. On restart, Controller re-lists all WasmPlugin CRDs and reconciles against pod annotation state | No data-plane impact |
| **veth injection failure** | Rust daemon drops the frame and increments telemetry counter. Monitoring alerts on elevated drop counts. 99% fast-path unaffected | Packet loss on exception path only |
| **Upgrade abort** (v2 fails health check) | v2 unloaded. v1 continues as active plugin. Failure recorded as K8s event | No data-plane impact |
| **Drain timeout exceeded** | v1 force-killed. At most one in-flight packet per affected TEID may be abandoned. See Section 6 | Bounded: at most 1 packet per TEID |

---

## 9. Design Trade-offs and Considerations

### 9.1 Safety vs. Zero-Copy (Bounded Wasm Copy)

The data path achieves kernel-to-userspace zero-copy via AF_XDP shared UMEM. However, a bounded payload copy (`2 × payload_len` bytes per exception-path packet) is performed at the Wasm boundary to preserve Wasm linear memory isolation.

**Why not map UMEM directly into Wasm?** Wasm's linear memory is a strictly isolated contiguous byte array. Exposing the AF_XDP UMEM as Wasm shared memory would break this isolation entirely — a buggy plugin could corrupt the live packet buffer and adjacent packets.

This is a deliberate design decision: **the copy overhead is the measurable cost of tenant-safe plugin execution**. The paper will measure this cost at 64B, 512B, and 1500B payload sizes and present it as a design-space contribution.

### 9.2 veth Reinjection

**Phase 1 decision**: Use veth pair for reinjection (works with unmodified Open5GS).

| Alternative | Kernel Stack Traversal | Complexity | Status |
|-------------|----------------------|------------|--------|
| veth pair (current) | Full (qdisc, netfilter, IP routing) | Low | **Phase 1** |
| AF_XDP on both sides | Zero | Very High — requires UPF modification | Future |
| tun/tap device | Partial (bypasses L2/qdisc) | Medium | Possible Phase 2 |

### 9.3 Software Checksum Recalculation

On PASS_MOD verdict, the Rust daemon recalculates the UDP checksum in software before writing to the XSK TX ring. Hardware checksum offload (`CHECKSUM_PARTIAL`) cannot be used because the veth reinjection path uses a virtual device that has no physical NIC hardware. Cost: ~100–200ns for a 1500B payload, negligible relative to the ~2–5μs veth traversal overhead.

### 9.4 Encrypted Traffic Limitation

If GTP-U payloads are encrypted (e.g., IPsec in transport mode), L7 DPI is blind. The system can still perform TEID-level operations (DROP, METER, TAG) but cannot inspect or modify encrypted payload content. This is an inherent limitation of any user-plane DPI system and is acknowledged as out of scope.

---

## 10. Phase 1 Prototype Scope

Phase 1 validates the three research questions in isolation from full 3GPP integration complexity. It proves the core mechanisms, not production readiness.

| Component | Prototype Implementation |
|-----------|------------------------|
| **Traffic generator** | Open5GS in loopback mode with a synthetic GTP-U packet generator. Fixed TEID set. No live RAN |
| **TEID map population** | Static: TEIDs written to the BPF map at startup via a CLI tool. No PFCP integration |
| **Reference plugins** | Three Rust-compiled Wasm binaries: (1) null plugin (PASS only — baseline overhead), (2) L7 string match plugin (Boyer-Moore scan, return DROP/PASS), (3) byte counter plugin (METER verdict, validates per-TEID counter maps) |
| **K8s environment** | Single-node k3s cluster. UPF pod runs Open5GS UPF + Rust daemon + Sidecar. Controller runs as a separate deployment |
| **Threading** | Single-threaded Rust daemon, single AF_XDP RX queue |
| **Multi-tenancy** | Single slice (single TEID set) |

**Evaluation metrics**:

| Metric | Description |
|--------|-------------|
| Fast-path throughput (Gbps) | eBPF-only baseline vs. eBPF+Wasm overhead |
| Exception-path per-packet latency (ns) | P50 / P99 / P99.9 for 64B, 512B, 1500B payloads |
| Hitless upgrade switchover time (ns) | Time from `bpf_map_update_elem()` to first v2-processed packet |
| Drain cycle correctness | Zero dropped fast-path packets during upgrade |
| Fault recovery time (ms) | Time from Rust daemon crash to resumed exception-path processing |
| veth reinjection latency (μs) | Isolated measurement of the egress kernel-stack traversal cost |

**Baselines for comparison**:
1. Open5GS UPF (vanilla, no eBPF)
2. eUPF (eBPF-only, no Wasm)
3. This system (eBPF + Wasm + Operator)

---

## 11. Not Validated in Phase 1 (Future Work)

| Item | Phase | Notes |
|------|-------|-------|
| PFCP session event subscription for dynamic TEID discovery from the SMF | Phase 2 | Required for production; static TEIDs sufficient for research validation |
| Multi-slice concurrent isolation | Phase 2 | Architectural guarantee documented in Section 5; single-slice validated in Phase 1 |
| Multi-queue RSS-mapped AF_XDP scaling with per-thread Wasm instances | Phase 2 | Production throughput requires this; single-queue sufficient for RQ1/RQ2 |
| AF_XDP-to-AF_XDP direct reinjection bypass (eliminating veth overhead) | Phase 2+ | Requires Open5GS modification or custom UPF |
| Inner 5-tuple secondary filter for sub-TEID exception routing granularity | Phase 2 | Enables selective per-flow DPI within a single session |
| Multi-TEID concurrent upgrades | Phase 2 | Phase 1 validates single-TEID upgrade correctness |
| Production NIC throughput at line rate | Phase 2+ | Requires bare-metal testbed with 25/100G NICs |
| Formal threat model for malicious tenant plugins | Paper | Must be addressed in the paper even if not coded in Phase 1 |

---

## 12. Open Questions for Phase 1 Evaluation

These are the unresolved questions the prototype **must answer** before strong claims can be made in the paper.

1. What is the steady-state per-packet latency overhead of the eBPF-to-Wasm handoff for payloads of 64B, 512B, and 1500B?
2. Does fuel-metered Wasm provide sufficient protection against adversarial plugins without unacceptable overhead on well-behaved ones?
3. Is 100ms an appropriate drain window? What is the P99 Wasm execution time for a realistic DPI plugin on maximum-size GTP-U payloads?
4. What is the AF_XDP buffer fill rate under burst traffic? At what pps rate does the exception path begin dropping packets before Wasm sees them?
5. Does the veth reinjection path introduce measurable per-packet latency compared to direct UPF processing? (This is the primary overhead cost of inline versus observe-only)
6. What is the minimum fuel budget that allows realistic DPI (e.g., a 256-byte Boyer-Moore string search) to complete within the 500μs timeout?
7. What is the ratio of `memcpy` time to total exception-path latency for each payload size? (Validates the bounded-copy design decision)

---

**End of High-Level Design Document — v2.0**
