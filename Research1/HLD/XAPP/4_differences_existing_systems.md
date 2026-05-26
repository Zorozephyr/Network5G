# 4. Differences from Existing Systems

Our proposed architecture fundamentally alters how xApps are executed and managed compared to the reference implementations provided by the O-RAN Software Community (OSC RIC), BubbleRAN/FlexRIC, and major vendors. This section provides a rigorous feature-level comparison and scopes our contribution relative to the emerging dApp paradigm.

## 4.1. The E2 Subscription Pipeline: Feature-Level Comparison

| Feature | OSC RIC (Standard K8s/Docker) | BubbleRAN/FlexRIC | Our Architecture (Wasm + GNR-D) |
| :--- | :--- | :--- | :--- |
| **Execution Environment** | Docker Container (K8s Pods) | Native C/Python process | Wasm AOT Linear Memory Sandbox |
| **Startup Time** | Seconds (container init + E2 sub) | Fast (native process) | ~50 microseconds (Wasm instantiation) |
| **E2 Encoding** | Standard E2AP/ASN.1 | Custom FlexRIC encoding (non-standard) | Standard E2AP/ASN.1 (compliant) |
| **High-Speed Routing** | Software-based RMR | Custom efficient dispatch | Hardware-accelerated Intel DLB |
| **Control Loop RTT** | 100–500ms typical | 650µs demonstrated | Target: sub-1ms (DLB + DSA + AMX) |
| **Memory Isolation** | Shared Kernel, cgroups (Weak) | No isolation (native process) | Mathematical Sandbox (Strong) |
| **E2 Sub Upgrade** | Drop subscription, reconnect (Disruptive) | Not specifically addressed | DLB pointer swap (Hitless/Lossless) |
| **ML Inference** | Standard CPU execution | Standard CPU execution | Hardware offload via Intel AMX (~5µs) |
| **Conflict Enforcement** | Intent-based (Honor system) | Not specifically addressed | Wasm host-function boundary (Structural) |
| **Security Model** | Container + optional eZTrust eBPF | Minimal (trusted environment) | Capability-based sandbox + Fuel Metering |
| **Standards Compliance** | Full O-RAN SC compliance | Partial (custom E2 encoding) | Full E2AP/SCTP compliance |
| **Hardware Dependency** | None (COTS x86/ARM) | None (COTS x86) | Intel GNR-D SoC required |

### Key Observations

**vs. OSC RIC:** We solve the four problems identified in Document 1 (cold start, E2 disruption, security, enforcement) at the cost of hardware dependency. The OSC RIC runs on any COTS hardware but suffers from all four problems.

**vs. BubbleRAN/FlexRIC:** BubbleRAN achieves comparable latency (650µs) through a different trade-off: they sacrifice E2 standards compliance (custom encoding) and security isolation (native processes) for performance. We retain standards compliance and add security isolation by offloading the performance gap to hardware. BubbleRAN is stronger in production maturity and multi-vendor interop (via Proxy-E2 Agent); we are stronger in security and hitless upgrades.

## 4.2. Comparison with Specific Research Systems

### vs. WA-RAN (HotNets '24)
| Dimension | WA-RAN | Our Architecture |
|---|---|---|
| **Wasm scope** | Communication plugins + xApp plugins | xApp execution only |
| **Hardware acceleration** | None (software-only) | DLB, DSA, AMX |
| **Hitless upgrade** | Plugin swap demonstrated at DU level | E2 subscription transfer at RIC level via DLB |
| **ML inference** | Not addressed | AMX-accelerated INT8/BF16 |
| **Conflict enforcement** | Not addressed | Host-function boundary CM |
| **Target layer** | O-DU + Near-RT RIC | Near-RT RIC only |

WA-RAN establishes the foundational concept; our work is the systems-level realization for the Near-RT RIC with hardware acceleration.

### vs. CORMO-RAN (arXiv 2506.19760)
| Dimension | CORMO-RAN | Our Architecture |
|---|---|---|
| **Migration strategy** | Container checkpoint (SM) or SDL externalization | DLB pointer swap (no migration needed) |
| **Downtime** | T_D > 0 (SM) or T_D = 0 with SDL constraint | Zero (E2 sub never disrupted) |
| **xApp architecture requirement** | SM: any; SDL: must pre-architect for SDL | Any Wasm-compiled xApp |
| **Primary goal** | Energy-efficient node consolidation | Hitless xApp upgrades + security |
| **Scope** | rApp orchestrator (Non-RT RIC) | Near-RT RIC platform redesign |

CORMO-RAN and our architecture solve different problems: they optimize cluster energy via migration; we optimize the xApp execution environment. The approaches are complementary — CORMO-RAN could orchestrate when to consolidate Wasm workers across GNR-D nodes.

### vs. eZTrust/OZTrust (Utah)
| Dimension | eZTrust/OZTrust | Our Architecture |
|---|---|---|
| **Security mechanism** | eBPF packet-level interception | Wasm capability-based sandbox |
| **Deployment model** | Retrofit onto existing containers | Greenfield Wasm runtime |
| **Bypass resistance** | Reactive (drop unauthorized packets) | Structural (no network stack) |
| **Conflict integration** | Separate from CM | CM embedded in host function |
| **Existing xApp compatibility** | Full (transparent injection) | Requires recompilation to Wasm |

eZTrust is the best security retrofit for existing container-based RICs. Our approach is stronger in principle but requires xApps to be recompiled to Wasm — a significant adoption barrier.

## 4.3. Clarifying the Scope: Near-RT RIC xApps vs. O-DU dApps

> [!WARNING]
> **Avoiding the dApp Collision:** The O-RAN Next Generation Research Group (nGRG) is actively standardizing the use of WebAssembly for **dApps (Distributed Applications)**. It is critical to differentiate our work from this emerging concept, as both use Wasm but operate at entirely different layers.

| Dimension | dApps (nGRG / E3) | Our xApps (E2) |
|---|---|---|
| **Interface** | E3 (emerging, not yet standardized) | E2 (O-RAN standardized) |
| **Location** | Co-located at O-DU/O-CU | Near-RT RIC (separate platform) |
| **Timescale** | Sub-1ms (MAC/PHY layer) | 10ms–1s (RRM/control plane) |
| **Data access** | Raw I/Q samples, scheduling grants | E2SM KPM/RC metrics and control |
| **Use cases** | Beam management, HARQ adaptation | Traffic steering, slicing, energy saving |
| **Relationship** | dApps may replace some RT-RIC functions | We optimize Near-RT RIC xApp execution |

*   **dApps (The nGRG Focus):** dApps operate over the upcoming **E3 interface**, co-located directly at the **O-DU (Distributed Unit)**. They operate at the MAC/PHY layer on sub-1ms timescales, manipulating raw I/Q samples. The 2026 arXiv preprint on Wasm-based real-time dApps demonstrates this concept.
*   **xApps (Our Focus):** Our architecture remains strictly anchored to the **Near-RT RIC and the E2 interface** (10ms to 1s timescale). We are solving the container orchestration, state migration, and security issues of standard xApps interacting with the O-CU/O-DU via E2AP/SCTP.

While both utilize Wasm, our contribution is the **systems architecture of the Near-RT RIC platform itself** (utilizing DLB and DSA for hitless state migration and message routing), rather than the sub-millisecond edge logic of O-DU dApps. In a complete O-RAN deployment, both approaches coexist: dApps handle sub-ms MAC decisions at the O-DU, while our Wasm xApps handle 10ms–1s RRM decisions at the Near-RT RIC.
