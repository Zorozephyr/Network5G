# O-RAN xApps: How They Work & Why Wasm Could Be a Real Contribution

## 1. How xApps Work Today (The Actual Architecture)

### The Big Picture

```
                          SMO / Non-RT RIC
                          (Management, A1 Policy)
                                │
                                │ A1 interface (seconds-to-minutes)
                                ▼
┌──────────────────────────────────────────────────────────────┐
│                     Near-RT RIC Platform                      │
│                     (Kubernetes cluster)                      │
│                                                               │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────┐  │
│   │ xApp A   │  │ xApp B   │  │ xApp C   │  │ Conflict  │  │
│   │(Sched.   │  │(Traffic  │  │(Energy   │  │ Manager   │  │
│   │ Optim.)  │  │ Steering)│  │ Savings) │  │           │  │
│   │          │  │          │  │          │  │           │  │
│   │ [Docker] │  │ [Docker] │  │ [Docker] │  │ [Docker]  │  │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────────┘  │
│        │              │              │                        │
│   ┌────▼──────────────▼──────────────▼─────────────────────┐ │
│   │              RMR (RIC Message Router)                    │ │
│   │              Internal message bus                        │ │
│   └─────────────────────┬──────────────────────────────────┘ │
│                         │                                     │
│   ┌─────────────────────▼──────────────────────────────────┐ │
│   │           Subscription Manager (SubMgr)                  │ │
│   │           E2 Terminator (E2T)                            │ │
│   └─────────────────────┬──────────────────────────────────┘ │
└─────────────────────────┼────────────────────────────────────┘
                          │
                          │ E2 interface (SCTP/IP)
                          │ E2SM-KPM (metrics)
                          │ E2SM-RC (control)
                          ▼
              ┌────────────────────────┐
              │     E2 Node (gNB)       │
              │                         │
              │  O-CU-CP  │  O-CU-UP   │
              │           │             │
              │      O-DU (scheduler)   │
              └────────────────────────┘
```

### The xApp Control Loop (Step by Step)

```
1. xApp subscribes to metrics:
   xApp → RMR → SubMgr → E2T → [E2 Subscription Request] → gNB
   "Send me PRB usage, RSRP, throughput every 100ms for all UEs"

2. gNB sends periodic reports:
   gNB → [E2 RIC Indication (REPORT)] → E2T → RMR → xApp
   Contains: E2SM-KPM data (per-UE metrics, per-cell stats)

3. xApp makes a decision:
   "UE-42 is experiencing poor RSRP. Handover to cell 3."
   or: "Slice A needs more PRBs. Reallocate from Slice B."

4. xApp sends control action:
   xApp → RMR → E2T → [E2 Control Request] → gNB
   Contains: E2SM-RC action (handover command, PRB reallocation)

5. gNB executes the action in the MAC scheduler

Total loop time: 10ms to 1 second (near-real-time)
```

### What Makes xApps Different From Any Other Application

This is the critical point. An xApp is NOT an "application" in the normal sense. It controls the **radio scheduler** — the function that decides:
- Which UE transmits in which time slot
- How many Physical Resource Blocks (PRBs) each UE gets
- What Modulation and Coding Scheme (MCS) to use
- When to handover a UE between cells
- How to split resources between network slices

**No application server, no firewall, no N6 appliance, no cloud ML pipeline can influence these decisions.** The only path is through the E2 interface, and only code running on the Near-RT RIC has access to it.

---

## 2. The Real Problems With xApps Today

### Problem 1: Container Overhead

Each xApp is a Docker container running on Kubernetes.

```
xApp container includes:
├── Linux base image (~50-200MB)
├── Language runtime (Python ~100MB, Java ~200MB)
├── RMR library
├── E2SM serialization libraries (protobuf/ASN.1)
├── ML model weights (if AI xApp)
└── Application code

Total image size: 200MB-1GB per xApp
Cold start time: 2-30 seconds
Memory footprint: 100MB-2GB per instance
```

For a 10ms-1s control loop, **seconds of cold start** is unacceptable. When you scale out a new xApp instance to handle load, those first few seconds have NO RAN control — the scheduler runs blind.

### Problem 2: No Hitless Upgrade

When you update an xApp (new scheduling algorithm, new ML model):

```
1. K8s rolling update: terminates old pod
2. Old xApp's E2 subscriptions are deleted
3. SubMgr sends E2 Subscription Delete to gNB
4. ── GAP: No control loop running ──
5. New pod starts (cold start: 2-30s)
6. New xApp re-subscribes to E2 metrics
7. SubMgr sends new E2 Subscription Request to gNB
8. gNB acknowledges and starts sending reports again
9. Control loop resumes

Total disruption: 5-60 seconds of NO RAN CONTROL
```

During this gap, the RAN scheduler falls back to default behavior. For URLLC slices or dense multi-tenant scenarios, this is a real problem. There is **no native hitless migration mechanism** in the current O-RAN SC RIC platform.

### Problem 3: xApp Security — The Wasm-Shaped Hole

This is the **strongest** argument. xApps run as regular containers with:
- Full Linux OS access
- Shared kernel with other xApps
- Network access (can call external APIs, exfiltrate data)
- File system access
- Memory not isolated from other containers beyond cgroups

A malicious or buggy xApp can:
- **Crash the E2 Terminator** — documented CVE-2023-41628: out-of-order E2 messages crash E2T
- **Send unauthorized RAN control commands** — no fine-grained access control per xApp
- **Poison ML models** of other xApps via shared data store
- **DoS the RIC** by flooding RMR with messages
- **Exfiltrate RAN telemetry** (PRB usage, UE locations, subscriber patterns)

The O-RAN security community explicitly calls out:
> "Insufficient verification of digital signatures and lack of sandboxing/isolation allow malicious applications to compromise the near-RT RIC" — Trend Micro O-RAN Security Research

### Problem 4: Multi-Vendor xApp Conflicts

When Vendor A's traffic steering xApp and Vendor B's energy saving xApp both try to control the same PRBs:

```
Vendor A xApp: "Allocate 80 PRBs to Slice A for throughput"
Vendor B xApp: "Reduce Slice A to 20 PRBs to save energy"
    → Parameter flipping, oscillation, degraded service
```

Current solutions: manifest-based coordination, centralized conflict manager. But these operate at the **intent level** (checking before sending). There's no **runtime isolation** that prevents one xApp from overriding another's control actions.

---

## 3. How Your Architecture Maps to This

### Direct Component Mapping

| Your Variant B Component | Maps to Near-RT RIC As |
|:---|:---|
| **DPDK/VPP fast-path** | RMR message bus (message reception from E2T) |
| **rte_flow exception steering** | Subscription-based message routing to specific Wasm xApps |
| **DLB atomic scheduling** | Per-UE message ordering to Wasm xApp workers |
| **Wasm sandbox (WasmEdge AOT)** | xApp execution environment — replaces Docker containers |
| **DSA async copy** | Zero-copy E2 indication payload into Wasm linear memory |
| **AMX** | ML inference inside Wasm xApps (scheduler optimization) |
| **Hitless plugin swap** | Wasm instance swap without dropping E2 subscriptions |
| **K8s CRD lifecycle** | WasmxApp CRD — same pattern as your WasmPlugin CRD |
| **Fuel metering + timeout** | Prevents buggy xApp from stalling the control loop |

### The Architecture (Adapted)

```
┌──────────────────────────────────────────────────────────────┐
│                Near-RT RIC on GNR-D SoC                       │
│                                                               │
│   ┌──────────────────────────────────────────────────────┐   │
│   │           WasmxApp Controller (Go)                    │   │
│   │           Watches: WasmxApp CRDs                      │   │
│   │           Actions: OCI pull → validate → load xApp    │   │
│   └───────────────────────┬──────────────────────────────┘   │
│                           │ gRPC                              │
│   ┌───────────────────────▼──────────────────────────────┐   │
│   │              E2T + Subscription Manager                │   │
│   │              (DPDK-accelerated SCTP termination)       │   │
│   │                                                        │   │
│   │   E2 Indication arrives:                               │   │
│   │   ┌──────────────────────────────────────────┐         │   │
│   │   │  Route by subscription ID:               │         │   │
│   │   │  Sub 1 (KPM, xApp A) → DLB Queue A      │         │   │
│   │   │  Sub 2 (KPM, xApp B) → DLB Queue B      │         │   │
│   │   │  Sub 3 (RC,  xApp A) → DLB Queue A      │         │   │
│   │   └──────────────────────────────────────────┘         │   │
│   └───────────────────────┬──────────────────────────────┘   │
│                           │                                   │
│   ┌───────────────────────▼──────────────────────────────┐   │
│   │              DLB (Hardware Load Balancer)              │   │
│   │              Per-subscription atomic scheduling        │   │
│   │                                                        │   │
│   │   Queue A → Wasm xApp A workers (Core 4-5)            │   │
│   │   Queue B → Wasm xApp B workers (Core 6-7)            │   │
│   │                                                        │   │
│   │   Guarantees: per-UE message ordering preserved        │   │
│   └───────────────────────┬──────────────────────────────┘   │
│                           │                                   │
│   ┌───────────────────────▼──────────────────────────────┐   │
│   │           Wasm xApp Execution Layer                    │   │
│   │                                                        │   │
│   │   ┌─────────────┐   ┌─────────────┐                   │   │
│   │   │ Wasm xApp A  │   │ Wasm xApp B  │                  │   │
│   │   │ (Scheduler   │   │ (Energy      │                  │   │
│   │   │  Optimizer)  │   │  Savings)    │                  │   │
│   │   │              │   │              │                  │   │
│   │   │ Memory: 4MB  │   │ Memory: 2MB  │                  │   │
│   │   │ Fuel: 1M/msg │   │ Fuel: 500K   │                  │   │
│   │   │              │   │              │                  │   │
│   │   │ Receives:    │   │ Receives:    │                  │   │
│   │   │  KPM reports │   │  KPM reports │                  │   │
│   │   │              │   │              │                  │   │
│   │   │ Outputs:     │   │ Outputs:     │                  │   │
│   │   │  RC control  │   │  RC control  │                  │   │
│   │   │  actions     │   │  actions     │                  │   │
│   │   └─────────────┘   └─────────────┘                   │   │
│   │                                                        │   │
│   │   Isolation: Wasm linear memory = CANNOT access         │   │
│   │   other xApp's state, cannot call external APIs,        │   │
│   │   cannot access filesystem, cannot make network calls   │   │
│   └──────────────────────────────────────────────────────┘   │
│                                                               │
│   E2 Control actions go back through E2T → gNB               │
└──────────────────────────────────────────────────────────────┘
```

---

## 4. Why This Survives the "Can't the App Just Do This?" Test

| Challenge | Answer |
|:---|:---|
| "Can the application layer do this?" | **No.** RAN scheduling is controlled ONLY through the E2 interface. No application, firewall, or server has access to PRB allocation, MCS selection, or handover decisions |
| "Can an existing product do this?" | **No.** WA-RAN (HotNets '24) proposed it as a concept. No production or open-source implementation exists |
| "Is the container approach good enough?" | **No.** 2-30s cold start, no hitless upgrade, no memory isolation, documented CVEs (E2T crashes from malicious xApps) |
| "Does it overlap with NEF?" | **No.** NEF exposes capabilities to external apps via API. xApps control the RAN scheduler directly via E2. Completely different plane |
| "Is the Wasm isolation actually needed?" | **Yes.** Multi-vendor xApps run on shared RIC. A buggy Vendor A xApp can crash E2T, flood RMR, or send unauthorized control commands. Wasm sandbox with fuel metering prevents all of these |

---

## 5. Concrete Use Cases (All Genuinely RAN-Only)

### UC1: Hitless Scheduling Algorithm Update

**Problem:** Operator wants to deploy a new ML-based PRB scheduler for their URLLC slice. Today, this requires redeploying the xApp container → 5-60s of no RAN control.

**With Wasm:**
1. New `.wasm` xApp pushed via CRD
2. AOT compiled, new DLB queue created
3. E2 subscription transferred (not deleted/re-created — subscription ID preserved)
4. Old Wasm instance drains in-flight messages
5. Total disruption: **0 messages lost, 0 E2 subscriptions dropped**

This is directly analogous to your UPF hitless swap, but here the impact is much more significant — because losing RAN control for seconds affects ALL users on the cell, not just exception-path packets.

### UC2: Multi-Vendor xApp Isolation

**Problem:** Operator runs Vendor A's traffic steering xApp and Vendor B's energy saving xApp on the same RIC. Vendor A's xApp has a memory leak that eventually crashes E2T (documented CVE pattern).

**With Wasm:**
- Each xApp runs in isolated Wasm linear memory (e.g., 4MB cap)
- Memory leak is contained — Wasm instance killed when memory limit hit
- E2T never affected
- Fuel metering prevents CPU starvation (buggy xApp can't spin forever)
- xApp cannot make network calls, access filesystem, or read other xApp's E2 data

**This is a genuine security improvement that containers cannot provide** — cgroups limit resources but don't prevent memory corruption, network exfiltration, or E2T crashes from malformed messages.

### UC3: Per-UE ML Scheduling with AMX

**Problem:** AI-driven scheduling xApp needs to run inference for each UE's channel quality prediction to optimize PRB allocation. With 100+ UEs and 100ms reporting interval, that's 1000+ inferences/second.

**With Wasm + AMX:**
- Small CNN (channel quality → optimal MCS/PRB) runs inside Wasm
- AMX INT8 inference: ~5μs per UE
- DLB ensures per-UE ordering: all reports for UE-42 go to same worker
- Total: 100 UEs × 5μs = 500μs per reporting cycle — well within 100ms budget
- Model update: hitless Wasm swap, no E2 disruption

Container-based xApp doing the same: Python + PyTorch + K8s overhead = 10-100ms per inference cycle. Barely fits in the 100ms-1s window, no headroom.

---

## 6. Honest Risks & Challenges

| Risk | Severity | Notes |
|:---|:---|:---|
| **E2 interface maturity** | Medium | E2SM-RC Control Styles still evolving. Multi-vendor E2 interop is fragile |
| **RMR replacement** | High | Your architecture replaces the RMR message bus with DLB-based routing. This is a significant departure from the OSC RIC architecture |
| **E2T integration** | High | E2T (E2 Terminator) handles SCTP, ASN.1 encoding. Your Wasm xApp needs to receive decoded E2SM payloads, not raw SCTP. This means E2T still runs as a separate component |
| **Subscription state management** | Medium | Hitless swap requires preserving E2 subscription state across Wasm instances. This is non-trivial — you need a shared subscription store outside the xApp |
| **WA-RAN as prior art** | Medium | WA-RAN (HotNets '24) proposed Wasm for O-RAN. Your work needs to clearly differentiate: they proposed the concept, you provide the complete system with DLB/DSA/AMX/hitless-swap on GNR-D |
| **Testability** | Medium-High | You need srsRAN + FlexRIC or OSC RIC to validate. Setup is complex but documented |

---

## 7. How This Differs From WA-RAN

| Aspect | WA-RAN (HotNets '24) | Your Architecture |
|:---|:---|:---|
| **Paper type** | Workshop paper (4 pages) | Full system design |
| **Hardware** | Generic — no hardware acceleration | GNR-D: DLB, DSA, AMX, QAT |
| **Message scheduling** | Not addressed | DLB atomic per-UE ordering |
| **Memory copy** | Not addressed | DSA async copy into Wasm linear memory |
| **ML inference** | Not addressed | AMX INT8/BF16 inside Wasm |
| **Hitless upgrade** | Mentioned as benefit, not designed | Full mechanism: DLB queue swap + subscription state preservation |
| **Security model** | "Sandboxing" mentioned generically | Fuel metering + timeout + memory cap + no syscall access |
| **K8s integration** | Not addressed | Full CRD → OCI → AOT → deploy pipeline |
| **Implementation** | Proof-of-concept demo | Targeting open-source RICs (FlexRIC, OSC) |
