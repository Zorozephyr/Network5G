# Research Glossary — Part 1: Quick Reference & O-RAN Terms

> Your paper: **Hardware-Accelerated Wasm-Based Near-RT RIC**
> Use this when you forget what a term is or why it matters to your work.

---

## Master Acronym Table

| Acronym | Full Form | Category |
|---------|-----------|----------|
| AMF | Access and Mobility Management Function | 5G Core NF |
| AMX | Advanced Matrix Extensions (Intel) | Hardware Accel |
| AOT | Ahead-of-Time Compilation | Wasm |
| ARP | Address Resolution Protocol | Networking L2 |
| ASN.1 | Abstract Syntax Notation One | Encoding |
| AUSF | Authentication Server Function | 5G Core NF |
| BGP | Border Gateway Protocol | Routing |
| CBRS | Citizens Broadband Radio Service | Spectrum |
| CIO | Cell Individual Offset | RAN Parameter |
| CMF | Conflict Mitigation Function | O-RAN |
| COTS | Commercial Off-The-Shelf | Hardware |
| CQI | Channel Quality Indicator | RAN |
| CRD | Custom Resource Definition | Kubernetes |
| CRIU | Checkpoint/Restore In Userspace | Linux |
| CU | Centralized Unit | RAN |
| CVE | Common Vulnerabilities and Exposures | Security |
| dApp | Distributed Application (O-RAN E3) | O-RAN |
| DLB | Dynamic Load Balancer (Intel) | Hardware Accel |
| DMA | Direct Memory Access | Hardware |
| DPDK | Data Plane Development Kit | Data Plane |
| DPI | Deep Packet Inspection | Packet Processing |
| DPU | Data Processing Unit | Hardware |
| DSA | Data Streaming Accelerator (Intel) | Hardware Accel |
| DSCP | Differentiated Services Code Point | QoS |
| DU | Distributed Unit | RAN |
| E2 | Interface: Near-RT RIC ↔ gNB | O-RAN Interface |
| E2AP | E2 Application Protocol | O-RAN Protocol |
| E2SM | E2 Service Model | O-RAN Protocol |
| E2T | E2 Terminator | O-RAN Component |
| E3 | Interface: dApps ↔ O-DU (emerging) | O-RAN Interface |
| eBPF | Extended Berkeley Packet Filter | Data Plane |
| eCPRI | Enhanced Common Public Radio Interface | Fronthaul |
| eMBB | Enhanced Mobile Broadband | 5G Use Case |
| F1 | Interface: DU ↔ CU | 3GPP Interface |
| gNB | Next Generation Node B (5G base station) | RAN |
| GNR-D | Granite Rapids-D (Intel SoC) | Hardware |
| GRE | Generic Routing Encapsulation | Tunneling |
| GTP-U | GPRS Tunneling Protocol - User Plane | Tunneling |
| GUTI | Globally Unique Temporary Identifier | 5G Identity |
| HAL | Hardware Abstraction Layer | Software Design |
| HARQ | Hybrid Automatic Repeat Request | RAN L2 |
| IMEI | International Mobile Equipment Identity | Device ID |
| IMSI | International Mobile Subscriber Identity | Subscriber ID |
| IPsec | Internet Protocol Security | Security |
| KPM | Key Performance Measurement | O-RAN E2SM |
| LAN | Local Area Network | Networking |
| LTE | Long-Term Evolution (4G) | Cellular |
| MAC | Medium Access Control (Layer 2) | Networking |
| MEC | Multi-access Edge Computing | Edge |
| MIMO | Multiple-Input Multiple-Output | RAN Antenna |
| mMTC | Massive Machine-Type Communication | 5G Use Case |
| MPLS | Multiprotocol Label Switching | Transport |
| MVNO | Mobile Virtual Network Operator | Business |
| NAT | Network Address Translation | Networking |
| NDT | Network Digital Twin | O-RAN |
| NEF | Network Exposure Function | 5G Core NF |
| NFV | Network Functions Virtualization | Architecture |
| NR | New Radio (5G) | RAN |
| NRF | Network Repository Function | 5G Core NF |
| NSSF | Network Slice Selection Function | 5G Core NF |
| NTN | Non-Terrestrial Network | Satellite |
| O-CU | O-RAN Centralized Unit | O-RAN RAN |
| O-DU | O-RAN Distributed Unit | O-RAN RAN |
| O-RU | O-RAN Radio Unit | O-RAN RAN |
| O1 | Interface: SMO ↔ O-RAN components | O-RAN Interface |
| OFDMA | Orthogonal Frequency Division Multiple Access | RAN |
| OSC | O-RAN Software Community | O-RAN |
| OSPF | Open Shortest Path First | Routing |
| OVS | Open vSwitch | Networking |
| PCF | Policy Control Function | 5G Core NF |
| PDCP | Packet Data Convergence Protocol | RAN L2 |
| PRB | Physical Resource Block | RAN |
| QoS | Quality of Service | Networking |
| rApp | RIC Application (Non-RT RIC) | O-RAN |
| RBAC | Role-Based Access Control | Security |
| RC | RAN Control (E2SM) | O-RAN E2SM |
| RIC | RAN Intelligent Controller | O-RAN |
| RLC | Radio Link Control | RAN L2 |
| RMR | RIC Message Router | O-RAN Component |
| RSRP | Reference Signal Received Power | RAN Metric |
| RU | Radio Unit | RAN |
| SASE | Secure Access Service Edge | Enterprise Security |
| SCS | Subcarrier Spacing | RAN |
| SCTP | Stream Control Transmission Protocol | Transport |
| SDL | Shared Data Layer | O-RAN Component |
| SDN | Software-Defined Networking | Architecture |
| SD-WAN | Software-Defined Wide Area Network | Enterprise |
| SLA | Service Level Agreement | Business |
| SMF | Session Management Function | 5G Core NF |
| SMO | Service Management and Orchestration | O-RAN |
| S-NSSAI | Single Network Slice Selection Assistance Info | 5G Slicing |
| SR-IOV | Single Root I/O Virtualization | Hardware |
| SRv6 | Segment Routing over IPv6 | Transport |
| SUCI | Subscription Concealed Identifier | 5G Identity |
| SUPI | Subscription Permanent Identifier | 5G Identity |
| TC | Traffic Control | Linux/O-RAN |
| TEID | Tunnel Endpoint Identifier | GTP-U |
| UDM | Unified Data Management | 5G Core NF |
| UE | User Equipment | Mobile Device |
| UPF | User Plane Function | 5G Core NF |
| URLLC | Ultra-Reliable Low-Latency Communication | 5G Use Case |
| VLAN | Virtual Local Area Network | Networking L2 |
| vRAN | Virtualized RAN | RAN Deployment |
| VXLAN | Virtual Extensible LAN | Tunneling |
| Wasm | WebAssembly | Execution Runtime |
| WLAN | Wireless Local Area Network | Networking |
| xApp | RIC Application (Near-RT RIC) | O-RAN |
| XDP | eXpress Data Path | Data Plane |
| ZTNA | Zero Trust Network Access | Security |

---

## Section 1: O-RAN Architecture Terms

### Near-RT RIC (Near-Real-Time RAN Intelligent Controller)

**What it is:** A software platform that hosts xApps — microservices that make RAN optimization decisions (traffic steering, scheduling, energy saving) within 10ms to 1s control loops. It sits between the SMO (management) and the gNB (radio). It receives telemetry from gNBs via E2 Indications, processes it through xApps, and sends back E2 Control messages.

**Why it matters to YOUR paper:** This is the thing you're redesigning. Your paper replaces the container-based execution environment of the Near-RT RIC with a Wasm+GNR-D architecture. Every problem you solve (cold start, E2 disruption, security, conflict enforcement) is a problem of the current Near-RT RIC.

**Your files:** [1_current_issues.md](file:///home/user/Coding/Network5GPractice/Research1/HLD/XAPP/1_current_issues.md), [3_overall_architecture.md](file:///home/user/Coding/Network5GPractice/Research1/HLD/XAPP/3_overall_architecture.md)

---

### xApp (Near-RT RIC Application)

**What it is:** A microservice that runs inside the Near-RT RIC. Each xApp subscribes to specific E2 Service Models (e.g., KPM for metrics, RC for control), receives E2 Indication reports from gNBs, runs optimization logic (often ML-based), and sends E2 Control messages back. Examples: Traffic Steering xApp, Energy Saving xApp, Anomaly Detection xApp.

**Current deployment:** Docker containers on Kubernetes. The xApp image includes a Linux base, language runtime (Python/Java), RMR libraries, and ML model weights → 200MB–1GB images, seconds-long cold starts.

**Your paper's change:** xApps become Wasm AOT modules (~few MB), instantiating in ~50µs, sandboxed with linear memory isolation, communicating only through host functions.

**Your files:** [2_our_solutions.md](file:///home/user/Coding/Network5GPractice/Research1/HLD/XAPP/2_our_solutions.md), [4_differences_existing_systems.md](file:///home/user/Coding/Network5GPractice/Research1/HLD/XAPP/4_differences_existing_systems.md)

---

### rApp (Non-RT RIC Application)

**What it is:** An application running in the Non-RT RIC (inside the SMO). Operates on timescales >1 second. rApps handle: ML model training for xApps, long-term policy creation (A1 policies), analytics, lifecycle orchestration. Example: PACIFISTA runs as an rApp — it profiles xApps before they're deployed to detect conflicts.

**Why it matters to YOUR paper:** rApps feed policies to your Wasm xApps via A1 (unchanged in your architecture). CORMO-RAN's migration orchestrator is also an rApp. Your architecture doesn't change the rApp layer — you change what happens *below* it.

---

### dApp (Distributed Application — E3 Interface)

**What it is:** A NEW concept from O-RAN nGRG. dApps run *co-located at the O-DU* over the emerging E3 interface. They operate at sub-1ms timescales on MAC/PHY layer data (raw I/Q samples, scheduling grants). Think: real-time beam management, HARQ adaptation. Also use Wasm, but for a completely different purpose than your work.

**Critical distinction for YOUR paper:** Both dApps and your xApps use Wasm, but at different layers. dApps = E3 interface, O-DU, sub-1ms, MAC/PHY. Your xApps = E2 interface, Near-RT RIC, 10ms–1s, RRM control plane. You must clarify this distinction to avoid reviewer confusion.

**Your file:** [4_differences_existing_systems.md § 4.3](file:///home/user/Coding/Network5GPractice/Research1/HLD/XAPP/4_differences_existing_systems.md)

---

### E2 Interface

**What it is:** The standardized interface between the Near-RT RIC and E2 Nodes (gNBs, O-CUs, O-DUs). It carries three message types:
- **E2 Setup:** Initial handshake — the gNB tells the RIC what Service Models it supports
- **E2 Indication:** gNB → RIC. Telemetry reports (KPM metrics like throughput, RSRP, PRB usage)
- **E2 Control:** RIC → gNB. Action commands (change handover threshold, adjust power, re-allocate PRBs)

Uses **SCTP** transport and **ASN.1** encoding. The encoding overhead is a key trade-off in your architecture (Section 6.1 of your unresolved problems).

**Your paper's change:** You keep E2 wire protocol unchanged (standards compliance), but redesign everything after SCTP termination — DLB replaces RMR for dispatch, DSA copies payloads into Wasm sandboxes.

---

### E2T (E2 Terminator)

**What it is:** The Near-RT RIC component that terminates the SCTP connection from the gNB. It decodes E2AP messages, extracts Subscription IDs, and routes payloads to the correct xApp via RMR (current) or DLB (your architecture). If E2T crashes, ALL RAN control is lost.

**Why it matters:** CVE-2023-41628 shows a malicious xApp can crash E2T by sending out-of-order subscription responses. In your architecture, xApps can't reach E2T directly — they must go through the `send_e2_control()` host function, making E2T crash structurally impossible from xApp code.

---

### RMR (RIC Message Router)

**What it is:** The pub/sub message bus inside the Near-RT RIC. All inter-component communication (E2T → xApp, xApp → xApp, xApp → Conflict Manager) flows through RMR. It uses message type IDs and Subscription IDs for routing.

**Your paper's change:** You replace RMR for the high-frequency E2 data path with Intel DLB hardware dispatch. RMR remains for management-plane signaling (A1 policies, inter-xApp coordination). This is a critical scoping distinction in your architecture doc.

---

### SDL (Shared Data Layer)

**What it is:** A distributed key-value store (backed by Redis) that xApps use to share state. Namespaced — each xApp gets its own namespace, with optional shared namespaces for cross-xApp data. O-RAN's designated mechanism for state externalization.

**Why it matters:** CORMO-RAN uses SDL for zero-downtime migration (T_D=0), but this requires xApps to be pre-architected for SDL. Many xApps store ML state in process memory instead. Your architecture replaces SDL for fast-path state with an in-process host KV store accessible via `read_cell_state()` / `write_vendor_state()` host functions (microsecond access vs. SDL's network round-trip to Redis).

---

### SMO (Service Management and Orchestration)

**What it is:** The top-level management layer of O-RAN. Contains the Non-RT RIC, hosts rApps, manages the entire O-RAN deployment via O1 (config/fault/PM), O2 (cloud infrastructure), and A1 (policy to Near-RT RIC) interfaces. Think of it as "the brain above the brain" — the Near-RT RIC makes 10ms–1s decisions, the SMO makes >1s strategic decisions.

**Your paper:** SMO workflow unchanged. Operators still deploy xApps via SMO — your K8s Operator translates CRD deployments into Wasm binary loading instead of Docker container instantiation.

---

### SubMgr (Subscription Manager)

**What it is:** Near-RT RIC component that manages E2 subscriptions. When an xApp wants to receive KPM reports from a gNB, it sends a subscription request to SubMgr, which forwards it to E2T → gNB. SubMgr tracks which xApp owns which subscription, handles merging (multiple xApps subscribing to the same metric), and uses timers to expire stale subscriptions.

**Why it matters to YOUR paper:** During a K8s rolling update, the old pod dies → SubMgr's timers expire the subscription → the new pod must re-subscribe. This is the "zero RAN control gap" you solve with DLB pointer swap. The E2 subscription is never torn down in your architecture — only the Wasm instance behind the DLB queue changes.

---

### CMF (Conflict Mitigation Function)

**What it is:** O-RAN WG3's specification for how conflicts between xApps should be detected and resolved. Defines the framework but NOT the algorithm. Three conflict types: direct (same parameter), indirect (different params → same KPI), implicit (cascading KPI triggers). Systems like PACIFISTA, COMIX, and Adamczyk CMF implement different algorithms within this framework.

**Your paper's contribution:** You don't build detection/resolution — you build the **enforcement layer**. Your `send_e2_control()` host function is the only way an xApp can emit E2 Control messages. The CM logic runs inside this boundary. The xApp physically cannot bypass it because Wasm has no OS network stack. This makes ANY detection/resolution algorithm tamper-proof.

---

## Section 2: Hardware Acceleration Terms

### Intel GNR-D (Granite Rapids-D)

**What it is:** Intel's SoC (System-on-Chip) designed for edge/networking workloads. Unlike standard server CPUs (Granite Rapids-SP for data centers), GNR-D integrates accelerators directly onto the chip die: DLB, DSA, QAT, AMX. 128 E-cores, designed for high-throughput parallel workloads rather than single-thread performance.

**Why it matters:** Your entire hardware-accelerated architecture runs on GNR-D. DLB for dispatch, DSA for memory copy, AMX for ML inference — all on one chip, no external accelerator cards needed. This is also your biggest limitation (Section 6.3) — hardware lock-in to Intel.

**Your files:** [Module1_CPU_GNR-D.md](file:///home/user/Coding/Network5GPractice/Research2/Module1/Module1_CPU_GNR-D.md)

---

### Intel DLB (Dynamic Load Balancer)

**What it is:** A hardware accelerator on GNR-D that dispatches work items to worker cores with guaranteed per-flow ordering. Think of it as a hardware-level message router. It maintains queues, ensures packets for the same UE always go to the same worker (atomic scheduling), and can rebalance load across workers without software overhead.

**Your paper's role:** DLB replaces RMR for E2 data-path routing. E2T maps Subscription IDs to DLB queues. During xApp upgrade (v1→v2), you atomically swap the DLB queue pointer from v1's Wasm memory to v2's — the gNB never knows an upgrade happened. This is your **hitless E2 subscription transfer** mechanism.

---

### Intel DSA (Data Streaming Accelerator)

**What it is:** Hardware accelerator for memory copy/move/fill operations. Instead of burning CPU cycles on `memcpy()`, DSA performs asynchronous DMA transfers in the background. On GNR-D, DSA can move data between memory regions at wire speed.

**Your paper's role:** After DLB dispatches an E2 payload to a specific Wasm worker, DSA performs the zero-copy transfer of that payload into the Wasm instance's linear memory sandbox. This preserves the 10ms latency budget that would otherwise be consumed by software memory copies.

---

### Intel AMX (Advanced Matrix Extensions)

**What it is:** CPU instructions for matrix multiplication (INT8, BF16 data types). Designed for AI/ML inference on CPU without needing a GPU. Can process tile-sized matrix operations in a single instruction. On GNR-D, AMX achieves ~5µs per UE for inference workloads.

**Your paper's role:** Many xApps use ML models (predict channel quality, optimize scheduling). On standard CPUs, inference takes 10–50ms — breaking the Near-RT budget. By exposing AMX intrinsics into the Wasm runtime, your xApps do per-UE ML inference in ~5µs. This is what WA-RAN couldn't solve — they proved Wasm works for RAN but couldn't meet latency for ML-heavy xApps.

---

### DPU (Data Processing Unit)

**What it is:** A SmartNIC on steroids. Contains a full programmable CPU (usually ARM cores), hardware accelerators, and high-speed networking. Examples: NVIDIA BlueField-3, Intel IPU. DPUs offload infrastructure tasks (encryption, firewalling, storage) from the host CPU.

**Why it matters to YOUR paper:** DPUs are the alternative approach to your GNR-D architecture. Your [PointsToConvince.md](file:///home/user/Coding/Network5GPractice/Research2/PointsToConvince.md) argues why GNR-D > DPU for your use case: DPUs create NVIDIA vendor lock-in, cost $2000+/card, can't run arbitrary Wasm plugins on fixed silicon, and can't do hitless plugin upgrades.

---

### DPDK (Data Plane Development Kit)

**What it is:** A set of user-space libraries that bypass the Linux kernel network stack entirely. Instead of packets going through the kernel (interrupts → driver → socket), DPDK takes ownership of the NIC and polls for packets directly in user-space. This eliminates context switches and achieves millions of packets/sec.

**Your paper's role:** Your E2T uses DPDK for fast-path SCTP termination. In your earlier UPF research, DPDK was the industry standard for high-throughput packet processing (replacing your eBPF/XDP approach at scale). Most production UPFs use DPDK.

**Your files:** [Module2_SRIOV_DPDK.md](file:///home/user/Coding/Network5GPractice/Research2/Module2/Module2_SRIOV_DPDK.md)

---

### SR-IOV (Single Root I/O Virtualization)

**What it is:** A hardware feature that lets a single physical NIC present itself as multiple virtual NICs (VFs — Virtual Functions) to the OS. Each VM or container gets its own dedicated VF, bypassing the hypervisor/kernel for near-native I/O performance. Used heavily in NFV deployments where VMs need line-rate networking.

**Your files:** [Module2_SRIOV_DPDK.md](file:///home/user/Coding/Network5GPractice/Research2/Module2/Module2_SRIOV_DPDK.md)

---

## Section 3: Your Research Papers — What Each One Is For

### WA-RAN (HotNets '24) — The Foundation You Build On

**What they did:** Proposed using Wasm plugins in the RAN for both communication (E2 protocol wrapping) and xApp execution. Demonstrated live scheduler swap on srsRAN without disconnecting UEs. Showed Wasm's sandboxing catches crashes that would kill a native gNB.

**What they didn't do:** No hardware acceleration, no hitless E2 subscription transfer, no conflict enforcement, no ML inference solution.

**Your paper's relationship:** "WA-RAN establishes the foundational concept; our work is the systems-level realization for the Near-RT RIC with hardware acceleration."

---

### CORMO-RAN (arXiv 2506.19760) — The Migration Problem

**What they did:** Addressed energy waste from always-on RIC clusters. Built an rApp that migrates stateful xApps between nodes using either container checkpointing (downtime T_D>0) or SDL-based migration (T_D=0 if xApp is pre-architected for SDL). Achieved 64% energy reduction.

**Why you cite them:** They prove the E2 disruption problem is real. Their solution (container migration) is the alternative to yours (DLB pointer swap). Your approach is simpler (no migration needed — just swap the pointer) but requires GNR-D hardware.

---

### PACIFISTA (arXiv 2405.04395) — Conflict Detection

**What they did:** Statistical profiling of xApps in a sandbox (digital twin) before production deployment. Builds conflict graphs, applies KS tests and chi-square tests to measure severity. Can predict 16–30% throughput loss from conflicts.

**Why you cite them:** They prove conflict is a real, measurable problem. They do detection — you do enforcement. Your architecture makes their detection decisions tamper-proof.

---

### COMIX (arXiv 2501.14619) — Conflict Resolution via Digital Twin

**What they did:** Used a Network Digital Twin to test conflict resolution policies before applying them live. Focused on DRL-based power control xApps. Achieved 30% energy efficiency improvement.

**Why you cite them:** Another conflict system that does resolution but assumes xApps can't bypass the resolver. Your enforcement layer completes their stack.

---

### eZTrust / OZTrust (Utah) — Security Retrofit

**What they did:** eBPF-based zero-trust enforcement for the Near-RT RIC. Tags every packet with sender context, verifies at receiver. Blocks unauthorized cross-xApp communication with negligible overhead. Filed CVEs proving the RIC has no access control.

**Why you cite them:** They prove the security problem (CVE-2023-41628, CVE-2023-42358). Their solution is reactive (drop bad packets) — yours is structural (xApps can't even attempt to send bad packets because they have no network stack).

---

### BubbleRAN / FlexRIC — The Performance Benchmark

**What they did:** Built an ultra-lean Near-RT RIC achieving 650µs control loop RTT. Replaced standard E2 ASN.1 with custom encoding. Used by CORMO-RAN, MX-AI, and many academic papers as the reference high-performance RIC.

**Why you cite them:** They're your performance comparison target. They achieve sub-ms loops via software optimization but sacrifice E2 standards compliance and security isolation. You retain compliance and add security by offloading to hardware.

---

### GraphSAGE GNN (Zolghadr, WCNC 2025) — Hidden Conflict Discovery

**What they did:** Used Graph Neural Networks to discover hidden indirect and implicit conflicts that aren't visible from subscription info alone. 100% detection accuracy with enough training samples.

**Why you cite them:** They validate your Problem 1.4 (parameter flipping). They do discovery — you do enforcement. Even if GraphSAGE perfectly detects a conflict, without your enforcement layer, a malicious xApp can still bypass the Conflict Manager.

**Your file:** [paper_analysis.md](file:///home/user/Coding/Network5GPractice/Research1/Paper_Notes/paper_analysis.md)

---

*Continued in Part 2: Networking Fundamentals, 5G Core, Data Plane, Security terms...*
