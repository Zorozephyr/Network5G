# K8s, DPUs, and What UPF Software Actually Does

## Question 1: Can DPUs and GNR-D Be Controlled by Kubernetes?

### Yes. Completely. This is how production 5G works today.

---

### How UPFs Run on Kubernetes (Real Production)

Every major vendor deploys their UPF as a **containerized pod** on Kubernetes:

```
┌──────────────────────────────────────────────────────────┐
│                   Kubernetes Cluster                      │
│                (Red Hat OpenShift / Ericsson CCD)          │
│                                                           │
│  ┌────────────────────────────────────────────────────┐   │
│  │                    UPF Pod                          │   │
│  │                                                     │   │
│  │  ┌───────────┐  ┌──────────┐  ┌─────────────────┐ │   │
│  │  │ Container │  │Container │  │   Container      │ │   │
│  │  │ UPF-CP    │  │ UPF-DP   │  │   OAM/Telemetry │ │   │
│  │  │ (PFCP)    │  │ (DPDK)   │  │   (Prometheus)  │ │   │
│  │  └───────────┘  └────┬─────┘  └─────────────────┘ │   │
│  │                      │                              │   │
│  │           Multus CNI (multiple interfaces)          │   │
│  │           ┌──────┐  ┌──────┐  ┌──────┐             │   │
│  │           │ eth0 │  │ net1 │  │ net2 │             │   │
│  │           │Mgmt  │  │ N3   │  │ N6   │             │   │
│  │           │(OVN) │  │SR-IOV│  │SR-IOV│             │   │
│  │           └──────┘  └──┬───┘  └──┬───┘             │   │
│  └─────────────────────────┼────────┼──────────────────┘   │
│                            │        │                      │
│                     ┌──────┴────────┴──────┐               │
│                     │  Physical NIC (100G)  │               │
│                     │  SR-IOV VFs passed    │               │
│                     │  directly to pod      │               │
│                     └──────────────────────┘               │
└──────────────────────────────────────────────────────────┘
```

**Key networking stack in real deployments:**

| Component | What It Does | Who Uses It |
|:---|:---|:---|
| **Multus CNI** | Gives each pod multiple network interfaces (management + data) | Everyone (Ericsson, Nokia, Samsung) |
| **SR-IOV** | Passes NIC virtual functions directly into pods, bypassing kernel | Every UPF pod needing high throughput |
| **DPDK inside pod** | User-space packet processing inside the container | Ericsson, Nokia, Samsung, Mavenir |
| **Red Hat OpenShift** | Hardened K8s distribution with telco operators + SR-IOV operator | AT&T, Vodafone, Deutsche Telekom |
| **Ericsson CCD** | Ericsson's own K8s distribution (Cloud Container Distribution) | Ericsson deployments |
| **Custom K8s Operators** | Automate ISSU, scaling, self-healing for UPF pods | All vendors |

---

### How DPUs Are Managed by Kubernetes

NVIDIA built an **entire K8s-native framework** for managing DPUs:

```
┌──────────────────────────────────────────────────────────┐
│              Host Kubernetes Cluster (x86)                 │
│                                                           │
│   DPF Operator (DOCA Platform Framework)                   │
│   ├── DPUSet CRD         → Provision/flash DPU firmware   │
│   ├── DPUCluster CRD     → Manage DPU K8s control plane  │
│   ├── DPUService CRD     → Deploy apps to DPU ARM cores  │
│   └── DPUDeployment CRD  → Lifecycle management          │
│                                                           │
│   ┌─────────────┐    ┌─────────────┐                      │
│   │  Host Pod   │    │  Host Pod   │                      │
│   │  (App)      │    │  (UPF-CP)   │                      │
│   └──────┬──────┘    └──────┬──────┘                      │
│          │                  │                              │
│   ═══════╪══════════════════╪═══════════   PCIe bus        │
│          │                  │                              │
│   ┌──────┴──────────────────┴──────────────────────┐      │
│   │           BlueField-3 DPU                       │      │
│   │                                                 │      │
│   │   ┌─────────────────────────────────────────┐   │      │
│   │   │     DPU Kubernetes Cluster (ARM64)       │   │      │
│   │   │     (separate control plane)             │   │      │
│   │   │                                          │   │      │
│   │   │   ┌──────────┐  ┌────────────────────┐  │   │      │
│   │   │   │ OVN-K8s  │  │  Your Wasm Plugin  │  │   │      │
│   │   │   │ (DPU     │  │  DPUService        │  │   │      │
│   │   │   │  Service)│  │  (could run here!)  │  │   │      │
│   │   │   └──────────┘  └────────────────────┘  │   │      │
│   │   └─────────────────────────────────────────┘   │      │
│   │                                                 │      │
│   │   eSwitch Hardware (ASAP² / Match-Action)       │      │
│   └─────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────┘
```

**This is production-ready today.** The DPU literally runs **its own Kubernetes cluster** on its ARM cores. You deploy services to the DPU using standard `kubectl apply` with `DPUService` CRDs.

> Your WasmPlugin CRD → could become a DPUService that deploys to the DPU's K8s cluster. The architecture maps directly.

---

## Question 2: What Does the UPF Software Actually Do?

### The Complete Picture

Even with full hardware offload, the UPF software application is **essential**. It's the brain. The hardware is the muscle.

```
                        UPF SOFTWARE (runs on CPU)
    ═══════════════════════════════════════════════════
    
    ┌─────────────────────────────────────────────────┐
    │              1. PFCP SESSION MANAGEMENT           │
    │                                                   │
    │  • Terminates N4 interface from SMF               │
    │  • Receives Session Establishment/Modification    │
    │  • Parses PDR/FAR/QER/URR rules                  │
    │  • Manages session state (millions of sessions)   │
    │  • Handles session release/timeout                │
    │                                                   │
    │  This is ALWAYS software. Hardware can't do       │
    │  PFCP protocol negotiation.                       │
    └─────────────────────────────────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────────┐
    │           2. RULE TRANSLATION & PROGRAMMING       │
    │                                                   │
    │  SMF says:  "PDR: match TEID=1001, QFI=5"        │
    │             "FAR: forward to N6, GTP decap"       │
    │             "QER: MBR=100Mbps"                    │
    │             "URR: volume-based reporting"          │
    │                                                   │
    │  UPF software TRANSLATES this into:               │
    │                                                   │
    │  For DPU:   doca_flow_pipe_add_entry(             │
    │               match: {teid=1001},                 │
    │               action: {decap, fwd_hairpin,        │
    │                        police_100mbps})            │
    │                                                   │
    │  For GNR-D: rte_flow_create(                      │
    │               pattern: {GTPU, teid=1001},         │
    │               action: {queue=3, mark=5})           │
    │                                                   │
    │  Hardware doesn't understand 3GPP. Software       │
    │  translates 3GPP rules → hardware instructions.   │
    └─────────────────────────────────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────────┐
    │           3. FIRST-PACKET / EXCEPTION HANDLING     │
    │                                                   │
    │  • New flow arrives → no hardware rule yet         │
    │  • Hardware sends to CPU ("exception")             │
    │  • CPU does PFCP lookup, determines action         │
    │  • CPU programs hardware with new rule             │
    │  • All future packets → hardware fast path         │
    │                                                   │
    │  Also handles:                                    │
    │  • Malformed packets                              │
    │  • Unsupported encapsulations                     │
    │  • Flow table overflow (TCAM full)                │
    │  • Error Indication messages (3GPP)               │
    └─────────────────────────────────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────────┐
    │           4. USAGE REPORTING & BILLING             │
    │                                                   │
    │  • Hardware maintains per-flow byte/packet        │
    │    counters (URR)                                  │
    │  • Software PERIODICALLY reads these counters     │
    │  • Aggregates them into Usage Reports             │
    │  • Sends reports to SMF (who forwards to CHF)     │
    │  • Handles threshold triggers ("notify when        │
    │    user exceeds 10GB")                            │
    │                                                   │
    │  Hardware counts. Software REPORTS.                │
    └─────────────────────────────────────────────────┘
                         │
                         ▼
    ┌─────────────────────────────────────────────────┐
    │           5. MANAGEMENT & TELEMETRY               │
    │                                                   │
    │  • Prometheus metrics endpoint                    │
    │  • Health checks (K8s liveness/readiness probes)  │
    │  • Logging (session events, errors)               │
    │  • gNMI/SNMP for network management               │
    │  • N9 inter-UPF forwarding decisions              │
    │  • Buffering (when UE is idle/paging)             │
    │  • Lawful Intercept (LI) control                  │
    └─────────────────────────────────────────────────┘
```

### The Real Split in Numbers

| Function | Where It Runs | % of CPU Time |
|:---|:---|:---|
| PFCP session management | **Always software** | ~15-25% |
| Rule translation → hardware | **Always software** | ~5-10% |
| First-packet exception handling | **Software** (then offloaded) | ~5-15% |
| Usage report aggregation | **Software** (reads HW counters) | ~10-15% |
| Management/telemetry/logging | **Software** | ~10-20% |
| **Steady-state forwarding** | **Hardware** (zero CPU) | 0% CPU |

> The CPU is NOT idle. It's doing session management, rule programming, billing, and telemetry constantly. It just doesn't touch the forwarding packets.

---

## Question 3: Where Does Your Wasm Plugin Fit?

### The map is clean. Here's exactly how it fits:

```
EXISTING UPF SOFTWARE STACK          YOUR WASM ADDITION
═══════════════════════════          ═══════════════════

┌─────────────────────┐
│ PFCP Session Mgmt   │ ─── unchanged ───────────────────
│ (N4 termination)    │
└────────┬────────────┘
         │
         │ When SMF says "this TEID needs DPI":
         │
┌────────▼────────────┐     ┌──────────────────────────┐
│ Rule Translation    │────▶│ ADDITION: Program this   │
│                     │     │ TEID as "exception" in   │
│                     │     │ hardware flow table      │
│                     │     │ (DOCA Flow or rte_flow)  │
└────────┬────────────┘     └──────────────────────────┘
         │
         │ Exception packets arrive at CPU/ARM:
         │
┌────────▼────────────┐     ┌──────────────────────────┐
│ Exception Handling  │────▶│ ADDITION: Route to Wasm  │
│ (first-pkt, errors) │     │ plugin instead of        │
│                     │     │ default exception handler │
│                     │     │                          │
│                     │     │ Wasm returns verdict:    │
│                     │     │ PASS → reinject to HW    │
│                     │     │ DROP → add to blocklist  │
│                     │     │ TAG  → set QoS marker    │
│                     │     │ METER→ count bytes       │
└────────┬────────────┘     └──────────────────────────┘
         │
┌────────▼────────────┐
│ Usage Reporting     │ ─── enhanced by METER verdict ──
│ (billing)           │     (per-tenant custom metering)
└────────┬────────────┘
         │
┌────────▼────────────┐
│ K8s Management      │ ─── enhanced by WasmPlugin CRD ─
│ (probes, telemetry) │     (tenant-driven plugin lifecycle)
└─────────────────────┘
```

### The CRD fits naturally into the existing K8s UPF deployment:

```yaml
# Existing UPF deployment (how operators do it today)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: upf-data-plane
spec:
  template:
    metadata:
      annotations:
        k8s.v1.cni.cncf.io/networks: sriov-net-n3, sriov-net-n6  # Multus
    spec:
      containers:
      - name: upf
        resources:
          requests:
            openshift.io/sriov_n3: '1'    # SR-IOV VF for N3
            openshift.io/sriov_n6: '1'    # SR-IOV VF for N6
            hugepages-1Gi: "4Gi"          # DPDK hugepages
---
# YOUR ADDITION — same K8s cluster, same workflow
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
```

Operators already use `kubectl apply` to deploy UPF pods. Your WasmPlugin CRD adds tenant-driven DPI/billing plugins using **the exact same workflow they already know**.

---

## TL;DR

| Question | Answer |
|:---|:---|
| **Can DPUs be managed by K8s?** | **Yes.** NVIDIA's DPF framework runs a full K8s cluster on the DPU ARM cores. CRDs: `DPUSet`, `DPUService`, `DPUCluster`. Production-ready. |
| **Are real 5G UPFs deployed on K8s?** | **Yes.** Ericsson, Nokia, Samsung all deploy on K8s (OpenShift/CCD). UPF pods use Multus + SR-IOV + DPDK. Custom Operators handle ISSU. |
| **What does UPF software do?** | PFCP session management, 3GPP rule → hardware translation, first-packet exception handling, usage reporting/billing, telemetry/logging. The CPU is the **brain**, hardware is the **muscle**. |
| **Does your CRD fit?** | **Perfectly.** It slots into the existing K8s UPF deployment model. Same `kubectl apply` workflow operators already use. |
