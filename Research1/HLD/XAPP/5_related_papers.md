# 5. Related Literature and Differentiation

Our proposed Wasm/GNR-D architecture intersects with several recent advancements in O-RAN research. This section provides a systematic differentiation across five research dimensions, citing specific papers and identifying where our work extends, complements, or diverges from existing solutions.

## 5.1. Wasm in O-RAN: WA-RAN (HotNets '24)

**Paper:** *"Towards Seamless 5G Open-RAN Integration with WebAssembly"* — Cannatà, Sun, Dumitriu, Hassanieh (EPFL), ACM HotNets '24

WA-RAN introduced the foundational concept of using Wasm plugins to run 5G RAN stack components. Their key contributions:
* **Communication Plugins** wrapping the E2 interface for multi-vendor interoperability (any wire format, serialization, encryption within the plugin)
* **xApp Plugins** encapsulating control logic as Wasm modules with host function callbacks
* **Live plugin swap** demonstrated on srsRAN without disconnecting UEs (MVNO slice scheduler use case)
* **Security:** Caught null pointer dereferences, OOB accesses, and double frees — gNB host survived all faults

**How we differ and extend:**

| WA-RAN Limitation | Our Extension |
|---|---|
| Software-only proof of concept — no hardware acceleration | DLB/DSA/AMX provide line-rate performance within 10ms budget |
| Demonstrated plugin swap at DU scheduler level | We design hitless E2 subscription transfer at the RIC level via DLB pointer swap |
| Did not address E2 interface throughput challenges | DLB provides atomic, per-flow message dispatch at zero CPU cost |
| Did not design ML hardware acceleration | AMX enables ~5µs INT8/BF16 inference per UE inside Wasm |
| Did not address conflict management | Host-function CM provides tamper-proof enforcement |
| Open problem: "What happens when a plugin returns invalid output?" | Fuel Metering halts runaway xApps; DLB rebalances on worker crash |

**Positioning:** WA-RAN proposed the *concept*; we provide the *systems architecture* that makes it production-grade for the Near-RT RIC.

## 5.2. Conflict Resolution: PACIFISTA, COMIX, GraphSAGE, and LLM Agents (2024–2026)

Recent literature focuses heavily on solving xApp conflicts at the detection, prevention, and resolution layers. Our contribution operates at a different layer: **enforcement**.

| System | Layer | Method | Our Relationship |
|---|---|---|---|
| **PACIFISTA** (arXiv 2405.04395) | Prevention | Sandbox profiling + statistical severity analysis; blocks conflicting xApp pairs pre-deployment | **Complementary.** PACIFISTA decides *which xApps can coexist*; we enforce that coexisting xApps *cannot bypass the CM*. |
| **COMIX** (arXiv 2501.14619) | Resolution | NDT simulation of DRL xApp conflicts; selects best arbitration policy | **Complementary.** COMIX decides *how to resolve* a conflict; we guarantee the resolution is *respected*. |
| **Adamczyk CMF** (IEEE ComMag 2023) | Detection + Resolution | Standards-aligned CMF built into Near-RT RIC; handles all 3 conflict types | **Complementary.** The CMF logic can be directly embedded in our `send_e2_control()` host function. |
| **GraphSAGE GNN** (IEEE WCNC 2025) | Detection | Data-driven reconstruction of hidden conflict graphs; discovers indirect/implicit conflicts | **Complementary.** GraphSAGE discovers *which conflicts exist*; we provide the enforcement that existing systems assume. |
| **LLM Agents** (arXiv 2603.07375, NDSS FutureG '25) | Detection + Resolution | Multi-agentic RAG for intent-driven conflict-aware orchestration; 70% accuracy improvement | **Complementary.** LLM reasoning operates at Non-RT RIC timescales; our enforcement operates at Near-RT RIC timescales. |

**Our unique position in the stack:**
* All existing conflict systems assume xApps are **well-behaved** and route messages through the designated CM via RMR
* In a container-based RIC, this is an **honor-system assumption** — a compromised xApp can bypass the CM via shared Linux networking
* eZTrust/OZTrust partially addresses this with eBPF packet filtering, but enforcement is **reactive** (drop unauthorized packets after the xApp attempts bypass)
* Our Wasm architecture provides **structural enforcement** — the xApp lacks the OS-level capability to bypass `send_e2_control()`, making the CM un-bypassable by construction

## 5.3. xApp Migration and State Transfer: CORMO-RAN and MANATEE

### CORMO-RAN (arXiv 2506.19760, June 2025)
*Calagna, Chiasserini et al., Politecnico di Torino / Red Hat OpenShift testbed*

CORMO-RAN is the most directly relevant prior work for our hitless upgrade claim. It provides two migration strategies:

| Strategy | Mechanism | Downtime (T_D) | xApp Constraint |
|---|---|---|---|
| **SM-MR** | Container checkpoint via CRIU, transfer, restore | T_D > 0 (minimizes resources) | Any container xApp |
| **SM-MD** | Shadow copy during transfer, minimize gap | T_D > 0 (minimizes downtime) | Any container xApp |
| **SDL-based** | State externalized to SDL before shutdown; new instance reads back | T_D = 0 | Must be pre-architected for SDL |

**How we differ:**
* CORMO-RAN's primary goal is **energy-efficient node consolidation** (up to 64% energy savings). Our primary goal is **hitless xApp version upgrades**.
* CORMO-RAN operates at the **container level** — the E2 subscription lifecycle (teardown/re-establishment) is still coupled to the container lifecycle. Even SDL-based migration with T_D = 0 requires the xApp to be specifically architected for SDL state externalization, which most ML-heavy xApps are not (acknowledged in their paper and in cross-cutting analysis Section 11.3).
* Our architecture **decouples E2 subscription state from the xApp lifecycle entirely**. The subscription is anchored at the DLB/E2T layer. Upgrading an xApp is a DLB pointer swap — the gNB never knows the xApp was upgraded, and no E2 subscription negotiation occurs.

### MANATEE (arXiv 2601.14009, January 2026)
*Montebugnoli et al., service mesh for xApp DevOps*

MANATEE brings CI/CD and service mesh (Istio/Envoy) to xApp lifecycle management, enabling canary releases, A/B testing, and circuit breaking. It introduces <1ms overhead per message.

**How we differ:**
* MANATEE solves the **software delivery** problem (how to safely roll out new xApp versions with progressive traffic shifting)
* We solve the **runtime platform** problem (how to execute xApps with microsecond startup, security isolation, and hitless E2 transfer)
* The approaches are **complementary**: MANATEE-style CI/CD pipelines could manage the deployment of `.wasm` binaries via our K8s Operator, while our platform provides the execution environment that eliminates the cold-start and E2-disruption risks that MANATEE's canary approach merely mitigates

## 5.4. Security Systems: eZTrust/OZTrust and Zero Trust Architectures

### eZTrust/OZTrust (University of Utah, 2024)
*Hung et al., IEEE Open Journal of Communications*

eZTrust is the most rigorous security system for container-based Near-RT RICs. Their contributions:
* CVE-2023-41628 and CVE-2023-42358 filed from systematic analysis of OSC H-Release
* eBPF-based context tracing, packet tagging, and packet verification at kernel level
* xApp-specific policy enforcement: which E2 service models, SDL namespaces, and xApps each xApp may access
* Demonstrated protecting AD, TS, and QP xApps with negligible overhead ("single-digit microseconds")

**How we differ:**

| Dimension | eZTrust/OZTrust | Our Architecture |
|---|---|---|
| **Philosophy** | Retrofit security onto existing containers | Security-by-construction via Wasm |
| **Mechanism** | eBPF packet-level interception at kernel | Capability-based Wasm sandbox; no network stack |
| **Bypass model** | xApp still *has* network stack; eBPF *intercepts* unauthorized packets | xApp *lacks* network stack; bypass is architecturally impossible |
| **Existing xApp compatibility** | Full — transparent eBPF injection, no xApp modification | Requires recompilation to Wasm |
| **CM integration** | Separate from conflict management | CM embedded in the only exit path |

**Positioning:** eZTrust is the best available solution for **existing, deployed** container-based RICs. Our architecture is the next-generation alternative that eliminates the attack surface rather than monitoring it — at the cost of requiring xApp recompilation.

## 5.5. Ultra-Lean RIC Platforms: BubbleRAN/FlexRIC

### BubbleRAN RIC-Sphere / FlexRIC (2024–2026)
*French startup; used as testbed in CORMO-RAN, MX-AI, and numerous academic publications*

BubbleRAN achieves 650µs control loop round-trip times via:
* Custom E2 encoding (non-standard, replacing ASN.1 with efficient binary format)
* Ultra-lean single-process RIC (no K8s microservices overhead)
* Proxy-E2 Agent for legacy vendor adaptation
* Python and C++ xApp SDKs with pre-built E2SM abstractions

**How we differ:**

| BubbleRAN Trade-off | Our Trade-off |
|---|---|
| Sacrifices E2 standards compliance for performance | Retains E2AP/SCTP compliance; offloads performance to hardware |
| No security isolation (native processes) | Strong Wasm sandbox isolation |
| No hitless upgrade mechanism | DLB pointer swap for zero-gap upgrades |
| Runs on COTS x86 (no hardware dependency) | Requires Intel GNR-D (hardware lock-in) |
| Production-mature (commercial product) | Research architecture (not yet production-validated) |

**Positioning:** BubbleRAN demonstrates that sub-ms control loops are achievable today with aggressive software optimization. Our architecture targets the same latency class while additionally providing security isolation and hitless upgrades — capabilities BubbleRAN's native-process model cannot offer.
