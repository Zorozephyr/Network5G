# 8. Clarifications and Common Doubts

During the architectural review, several critical questions were raised regarding the behavior of Kubernetes, the nature of xApp conflicts, hardware dependencies, and design trade-offs. These clarifications address the nuances of transitioning from a container-based to a Wasm-based Near-RT RIC, citing specific evidence from the research literature where applicable.

## 8.1. K8s Rolling Updates vs. E2 Hitless Migration

**Doubt:** *In a Kubernetes rolling update, the previous version is still running during the transition gap. Since the control loop is theoretically still running on the old pod until the new pod is ready, why is there a "cold start" or downtime issue?*

**Answer:** While Kubernetes handles stateless web applications seamlessly via rolling updates, **E2 subscriptions are highly stateful**. 
If Kubernetes starts a new pod (v2) while the old pod (v1) is still running, you encounter the "overlapping pods" problem:
1. The O-RAN Subscription Manager (SubMgr) and the gNB generally prohibit duplicate identical subscriptions to conserve resources. Thus, the new pod cannot easily spawn a second parallel E2 stream. The SubMgr uses timers to supervise subscription requests — a rolling update that exceeds these timeouts triggers subscription deletion.
2. The incoming E2 reports are actively routed by the RIC Message Router (RMR) to the specific IP address of pod v1. When v1 is terminated, the Routing Manager must update routes — creating a window where messages are routed to an invalid endpoint.
3. For v2 to take over, v1 must gracefully delete its subscription (to avoid crashing the E2 Terminator with dangling state — the exact vector exploited in CVE-2023-41628), and v2 must establish a new subscription. 
4. The gap occurs between v1 dropping the subscription and v2 successfully spinning up, pulling state from the Shared Data Layer (SDL), and securing a new subscription.

**Evidence:** The O-RAN SC SubMgr documentation explicitly describes this lifecycle. CORMO-RAN (arXiv 2506.19760) builds its entire system around solving the resulting migration downtime, offering Stateful Migration (SM, T_D > 0) and SDL-based migration (T_D = 0 but requiring pre-architected xApps). The O-RAN Alliance added "RIC Subscription State Control" to E2AP v7.00 (April 2025), confirming this was a recognized gap in prior specifications.

**Our Wasm Solution:** We bypass K8s network routing for E2 messages entirely. The subscription stays permanently anchored at the E2 Terminator/DLB layer. Upgrading an xApp is achieved by atomically redirecting the hardware DLB queue pointer from the v1 Wasm memory address to the v2 Wasm memory address. The gNB is never aware the xApp was upgraded — no subscription teardown or re-establishment occurs.

## 8.2. Understanding "Parameter Flipping" and the Conflict Management Stack

**Doubt:** *What exactly is "parameter flipping" in multi-vendor xApp conflicts, and what does it mean that current solutions operate at the "intent level" instead of providing "runtime isolation"?*

**Answer:** 

* **Parameter Flipping:** Imagine Vendor A's xApp wants to maximize throughput by allocating 80 PRBs to a slice, while Vendor B's xApp wants to save power by restricting that slice to 20 PRBs. If both send commands simultaneously, the gNB rapidly oscillates between 80 and 20 PRBs, destabilizing the network. PACIFISTA (arXiv 2405.04395) demonstrates 16–30% performance degradation from such conflicts. The O-RAN Alliance defines three conflict types: direct (same parameter), indirect (different parameters → same KPI), and implicit (cascading KPI-triggered actions).

* **The Conflict Management Stack:** The literature addresses this at four layers:
    1. **Prevention** (pre-deployment): PACIFISTA profiles xApps in a sandbox; Samsung requires parameter ownership registration
    2. **Detection** (runtime): GraphSAGE GNN (IEEE WCNC 2025) discovers hidden indirect/implicit conflicts; Adamczyk CMF monitors E2 messages
    3. **Resolution** (runtime): COMIX uses NDT simulation; game theory approaches seek Nash equilibria; LLM agents reason about intent
    4. **Enforcement** (runtime): Ensuring xApps cannot bypass the Conflict Manager — **this is our contribution**

* **The Weakness (Lack of Enforcement):** Layers 1–3 all assume the xApp routes its E2 Control messages through the designated CM via RMR. In a container-based RIC, this is an honor-system assumption. A compromised or buggy xApp can exploit shared Linux network namespaces to bypass the CM entirely and blast the E2 interface with commands. eZTrust/OZTrust partially addresses this with eBPF packet filtering, but the xApp still *possesses* the OS-level network stack — enforcement is reactive (intercept) rather than structural (remove capability).

* **Our Wasm Solution (Structural Enforcement):** Wasm confines the xApp in a strict memory sandbox with zero network access. The *only* way the xApp can output a command is by calling a specific Host Function (e.g., `send_e2_control()`). By embedding the Conflict Manager directly into that Host Function, we make it structurally impossible for an xApp to bypass the CM. We do not solve detection or resolution — we make existing detection/resolution systems tamper-proof.

## 8.3. Can the Original Software HLD Achieve This?

**Doubt:** *Can the original software-based eBPF/Wasm architecture achieve this, or is the Intel GNR-D SoC strictly necessary? Can other hardware alternatives work?*

**Answer:** 

* **Functional Viability:** Yes, the original software-based HLD (running on standard x86 servers) works perfectly for *functional validation*. The Wasm sandbox still provides security isolation, and software pointer swaps still provide hitless upgrades. This software-only variant delivers WA-RAN-class functionality: correct, secure, portable — but with higher latency.

* **Performance Necessity (Why GNR-D):** The original HLD fails to meet the strict 10ms latency budget at line rate when running ML-heavy xApps. Software dispatching burns CPU cycles, and running AI/ML models in standard Wasm takes 10–50ms. GNR-D provides:
    * **DLB** for zero-cost atomic dispatching (replaces software hash + lock contention)
    * **DSA** for zero-copy memory transfers (replaces CPU `memcpy()`)
    * **AMX** to cut ML inference from 10–50ms down to ~5μs per UE
    
    GNR-D elevates the design from a prototype to a production-grade telco platform capable of BubbleRAN-class latency while retaining security isolation.

* **Alternatives and the HAL Approach:**
    * **ARM-based DPUs (NVIDIA BlueField-3):** Offer great network offload but lack the strong CPU cores required to run heavy Wasm ML inference within latency budgets.
    * **Standard x86 servers:** Functional with software fallback for all three accelerators (see Section 6.3 mitigation paths). Degrades gracefully to 10–50ms control loops.
    * **BubbleRAN approach:** Achieves sub-ms on COTS x86 by replacing E2 encoding — but without security isolation or hitless upgrades.
    * A Hardware Abstraction Layer (HAL) allowing software fallback for DLB/DSA/AMX should be part of the design to avoid hard lock-in.

## 8.4. The Role of Kubernetes in the New Architecture

**Doubt:** *Are we still using Kubernetes at all?*

**Answer:** Yes, but its role changes fundamentally. We remove Kubernetes from the high-speed E2 data path but retain it for management-plane orchestration.

* **What K8s still manages:**
    * The Near-RT RIC platform (the E2T, the DLB dispatcher, and the Wasm Host process) runs as a persistent set of K8s Pods.
    * A1 policy delivery, SDL coordination, and management-plane signaling flow through standard K8s networking and RMR.
    * Operators deploy xApps via a Custom Resource Definition (CRD). A custom Wasm K8s Operator watches for CRD changes, pulls the `.wasm` binary, and dynamically loads it into the already-running Wasm Host.

* **What K8s no longer manages:**
    * Individual xApps are **not** deployed as Docker Pods — they are Wasm instances inside a persistent host process.
    * E2 message routing bypasses K8s networking entirely (DLB hardware dispatch).
    * xApp scaling is Wasm instance creation (~50µs), not pod scaling (seconds).

* **O-RAN Compliance:** This maintains 100% compliance with O-RAN Service Management and Orchestration (SMO) standards — the SMO sees K8s CRD-based xApp deployment and standard O1/A1 interfaces. The internal change (Wasm instead of Docker) is transparent to the SMO layer. MANATEE-style CI/CD pipelines and service mesh capabilities remain compatible for the management plane.

## 8.5. Wasm Execution Environment Separation

**Doubt:** *If the Wasm execution environment just keeps running continuously, how are we separating the execution and memory for different xApps from different vendors?*

**Answer:** Through Wasm Instances and Linear Memory — this is the mathematical foundation of our security model.

When the K8s Operator loads an xApp, it compiles it into a discrete **Wasm Instance**. Every instance is assigned its own contiguous block of **Linear Memory** (e.g., 4MB). Because WebAssembly bytecode is mathematically verified before execution, the compiler guarantees that no CPU instruction can address memory outside of that designated block. Vendor A's xApp physically cannot read or write to Vendor B's memory or the host OS memory. They are perfectly isolated inside a single, continuously running host process.

**Comparison with container isolation:**

| Isolation Property | Docker/K8s | Wasm |
|---|---|---|
| **Memory isolation** | OS-level cgroups/namespaces (kernel-enforced) | Mathematical verification (compiler-enforced) |
| **Network access** | Full OS network stack | Zero (no network capability) |
| **Filesystem access** | Container filesystem (shared kernel) | None by default (host-function controlled) |
| **Bypass model** | Kernel exploits, shared namespace abuse | Mathematically impossible (modulo Wasm runtime bugs) |
| **Crash blast radius** | Can crash shared kernel components (E2T, SDL) | Crashes only the Wasm instance |

This is the same isolation model WA-RAN demonstrated for protecting gNB hosts from faulty MVNO scheduler plugins: null pointer dereferences, OOB accesses, and double frees all crashed the plugin without affecting the host.

## 8.6. Are We Reinventing Load Balancing with Intel DLB?

**Doubt:** *If there are multiple instances of xApp A, are we just reinventing load balancing by using Intel DLB?*

**Answer:** We are not reinventing load balancing; we are **hardware-offloading** a very specific, complex type of load balancing: **stateful, flow-aware message dispatching with atomic ordering guarantees**.

* **The Software Problem:** If you rely on software load balancing (like K8s Services or RMR), you hit the "split brain" problem. If UE-42's first report goes to Instance 1, and its second report goes to Instance 2, the ML state is fragmented. Solving this in software requires complex hashing algorithms that burn CPU cycles and cause lock contention. DPDK's `rte_distributor` provides software-based flow-aware dispatch but at significant CPU cost.

* **The DLB Hardware Solution:** Intel DLB provides atomic queues in silicon. It load-balances messages across multiple Wasm workers but guarantees that all messages for a specific "Flow ID" (e.g., Subscription ID + UE ID) are *always* pinned to the exact same worker, in strict order, with zero CPU cost. If a worker crashes, DLB instantly rebalances the flow to another worker — the same atomicity guarantee that the hitless upgrade pointer swap relies on.

* **Why this matters for the E2 data path:** The E2 interface can carry thousands of KPM Indication messages per second per cell. Dispatching these to the correct xApp instance with per-UE ordering is not generic load balancing — it is a real-time message scheduling problem that software solutions struggle to handle within the 10ms budget at scale.

## 8.7. What is the Host-Enforced Conflict Manager?

**Doubt:** *What exactly is the "Host-Enforced Conflict Manager" and how does it prevent bypasses?*

**Answer:** To understand this, you must distinguish between the "Guest" (the xApp) and the "Host" (the platform).

* **The Vulnerability Today:** In a standard RIC, the Conflict Manager (CM) is just another container listening on the RMR bus. Because the xApp has its own network stack, a malicious xApp can ignore the CM and open a direct socket to the E2 Terminator to blast commands. eZTrust/OZTrust mitigates this with eBPF packet filtering, but the xApp still *possesses* the network stack — the eBPF program must react to every unauthorized packet attempt, and kernel-level vulnerabilities could theoretically be exploited to bypass eBPF enforcement.

* **The Host-Enforced Solution:** In WebAssembly, the Guest xApp has absolutely zero network access — no socket API, no filesystem API, no capability to communicate with the outside world except through explicitly provisioned Host Functions. The *only* way it can emit an E2 Control message is by calling `send_e2_control()`, a Host Function provided by the platform. We embed the Conflict Manager logic (which can implement any algorithm — PACIFISTA-style, COMIX-style, Adamczyk CMF, or custom operator logic) directly into that Host Function. Because the CM is baked into the only "exit door" of the sandbox, it is physically impossible for an xApp to bypass it.

* **Why this is stronger than eBPF:** eZTrust adds a **monitoring layer** (intercept unauthorized packets). Our approach removes the **capability** (no packets to intercept). The distinction is reactive enforcement vs. structural enforcement — analogous to the difference between a firewall (monitoring) and an air gap (capability removal).

## 8.8. Does the Shared KV Store Remove Wasm Security?

**Doubt:** *If we introduce a shared Key-Value (KV) store in the Host to let Wasm workers share global cell state, doesn't this remove the security we introduced?*

**Answer:** No, it does not remove the foundational security, but it does introduce a new "choke point" that the Host must actively defend. This is an honest trade-off.

* **Why Core Security is Unbroken:** The xApp still has no network access to crash the E2T, and it still cannot directly read or overwrite another xApp's linear memory. The mathematical sandbox holds firm. The KV store is accessed exclusively through host functions (`read_cell_state()`, `write_vendor_state()`), not through shared memory.

* **The New Risk (Data Poisoning):** By allowing xApps to leave messages for each other, a rogue xApp could write a fake value (e.g., `total_prb_utilization = 100%`) into the KV store, tricking other xApps into making bad decisions. This is a form of indirect conflict — not a security bypass, but a semantic manipulation.

* **The Mitigation:** The Host must enforce strict **Access Control Policies (RBAC)** on the KV API:
    * All xApps have **Read-Only** access to global aggregated metrics (computed by the Host from E2 Indications, not written by any xApp)
    * xApps can only **write** to their own vendor-namespaced keys (e.g., `vendor_A/my_state`)
    * The Host physically rejects unauthorized write attempts
    * Critical global aggregates (PRB utilization, cell load) are computed by the trusted Host process, not by any xApp

* **Comparison with SDL:** The O-RAN standard's SDL (Redis-based) has the same data poisoning risk — and OZTrust's eBPF policies enforce SDL namespace access at the packet level. Our host-function RBAC is the Wasm equivalent of OZTrust's SDL namespace enforcement, but implemented at the API level rather than the packet level.

## 8.9. How Does This Compare to the dApp Approach?

**Doubt:** *The O-RAN nGRG is also standardizing Wasm for dApps. Why not just use dApps instead of redesigning the Near-RT RIC?*

**Answer:** dApps and our xApp platform operate at entirely different layers and timescales. They are complementary, not competing.

| Dimension | dApps (E3 / O-DU) | Our xApps (E2 / Near-RT RIC) |
|---|---|---|
| **Timescale** | Sub-1ms (MAC scheduling, HARQ) | 10ms–1s (traffic steering, slicing) |
| **Data access** | Raw I/Q samples, scheduling grants | E2SM KPM/RC aggregated metrics |
| **Deployment location** | Co-located at O-DU hardware | Near-RT RIC platform (edge or cloud) |
| **Standardization** | E3 interface (nGRG, in development) | E2 interface (O-RAN WG3, established) |
| **Multi-vendor concern** | Less relevant (DU-specific) | Core concern (RIC hosts multi-vendor xApps) |

dApps solve "how do you run custom logic at the MAC layer?" Our architecture solves "how do you run multi-vendor xApps on the Near-RT RIC securely, with hitless upgrades, and with hardware-accelerated performance?" In a complete deployment, dApps handle sub-ms decisions at the DU, while our Wasm xApps handle 10ms–1s RRM decisions at the RIC — and both benefit from Wasm's sandboxing properties.
