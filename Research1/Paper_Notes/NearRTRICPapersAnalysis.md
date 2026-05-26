# O-RAN Near-RT RIC & xApp Research: Comprehensive Paper Summaries

> A deep-dive survey of ten frontier research systems spanning WebAssembly-based RAN integration, stateful xApp migration, service mesh orchestration, lifecycle management, high availability, conflict resolution, security, and ultra-lean RIC platforms — plus a cross-cutting analysis of existing challenges and how industry is responding.

---

## Table of Contents

1. [WA-RAN — WebAssembly for O-RAN (HotNets '24)](#1-wa-ran)
2. [CORMO-RAN — Stateful xApp Migration](#2-cormo-ran)
3. [MANATEE — xApp Service Mesh & DevOps Lifecycle](#3-manatee)
4. [PACIFISTA — xApp Conflict Evaluation & Management](#4-pacifista)
5. [OREO — xApp Lifecycle Orchestration & Sharing](#5-oreo)
6. [COMIX — Digital Twin Conflict Simulation](#6-comix)
7. [ORIGAMI/Centralized Conflict Coordination Approaches](#7-origami)
8. [OZTrust / eZTrust — eBPF-based xApp Access Control](#8-eztrust--oztrust)
9. [BubbleRAN — Ultra-Lean RIC with Sub-ms Control Loops](#9-bubbleran)
10. [ORCA-Style LLM-Based Conflict Resolution in O-RAN](#10-llm-based-conflict-resolution)
11. [Cross-Cutting Issues in Near-RT RIC xApp Systems](#11-cross-cutting-issues-in-near-rt-ric--xapp-systems)
12. [Industry Mitigation & Production Use](#12-industry-mitigation--production-use)

---

## 1. WA-RAN

**Full Title:** *Towards Seamless 5G Open-RAN Integration with WebAssembly*  
**Venue:** ACM HotNets '24 (23rd Workshop on Hot Topics in Networks), November 2024, Irvine, CA  
**Authors:** Raphael Cannatà, Haoxin Sun (EPFL, equal contribution), Dan Mihai Dumitriu (Pavonis LLC), Haitham Hassanieh (EPFL)

### Background and Motivation

The Radio Access Network has historically been a closed, single-vendor monolith — Nokia, Ericsson, or Huawei owning every layer of the stack. The O-RAN Alliance (founded 2018) promised to break this by defining open, standardized interfaces such as E2 (between RIC and gNB), F1 (between DU and CU), and the Open Fronthaul (between DU and RU). In theory, operators should be able to mix and match vendors freely. In practice, a disturbing trend is emerging: despite the open interfaces, operators like AT&T (Ericsson-only O-RAN), Vodafone UK (Samsung-only), and Vodafone Italy (Nokia-only) are sourcing their entire O-RAN stacks from single vendors, defeating O-RAN's core purpose.

The authors draw a sharp historical parallel to OpenFlow in data center networking. OpenFlow promised vendor-agnostic switch programmability through standardized protocols, but the need for vendor-specific extensions quickly shattered interoperability and OpenFlow is now rarely used in production data centers. WA-RAN is positioned explicitly as an attempt to prevent O-RAN suffering the same fate.

The root cause of integration failures is nuanced. Standards like E2 deliberately leave certain implementation details unspecified (e.g., the bit length used for encoding radio output power) and vendors interpret these ambiguities differently. When RIC A tries to talk to gNB B, the subtle mismatches accumulate into incompatibilities. Since most commercial RAN software is proprietary and closed, fixing these mismatches requires joint vendor cooperation — which is impractical at scale.

### Core Proposal: WebAssembly Plugins in the RAN

WA-RAN proposes replacing or wrapping the problematic integration points with WebAssembly (Wasm) plugins. Wasm, introduced in 2017 for high-performance web browser execution, has since found use in edge computing, IoT, and serverless functions. Its key properties that make it attractive for RAN are:

**Portability:** Wasm bytecode is platform-agnostic. The same `.wasm` binary runs on ARM-based radio hardware, x86 cloud servers, and everything in between, without recompilation. This eliminates the nightmare of maintaining different builds for different vendor hardware.

**Near-Native Performance:** Wasm supports Ahead-of-Time (AoT) compilation, converting bytecode to native machine code before execution. The overhead versus native code is measurable but small — the authors show plugin execution times well within 5G slot durations (1,000 µs per slot with 15 kHz subcarrier spacing).

**Security via Sandboxing:** Plugins run in isolated sandboxes. Even if an MVNO's scheduling plugin contains a bug or malicious code, it cannot crash the host gNB or access memory outside its sandbox. The paper demonstrates catching null pointer dereferences, out-of-bounds accesses, and double frees — in all cases the gNB host continues operating while an equivalent native crash would have brought down the entire base station.

**Language Agnosticism:** Wasm can be compiled from C, C++, Rust, Go, and many other languages. RAN developers are not constrained to a single language or toolchain.

### System Architecture

WA-RAN introduces two categories of Wasm plugins:

**Communication Plugins** wrap the wired protocol between E2 nodes (CU/DU inside the gNB) and the near-RT RIC. Instead of both sides having to agree on every detail of the E2 ASN.1 encoding, a communication plugin on each side handles serialization, encryption, and protocol details, exporting only a clean functional interface. A system integrator needing to bridge a Vendor A RIC with a Vendor B gNB only needs to write a plugin that translates between their respective wire formats — far simpler than modifying proprietary source code. Operators can freely choose their messaging protocol (ZeroMQ, Kafka), serialization format (ASN.1, Protobuf, JSON), and encryption algorithm (AES, RSA) within the plugin.

**xApp Plugins** encapsulate control logic (traffic steering, slice SLA assurance, handover decisions) within the near-RT RIC as Wasm modules. xApp plugins export functions the RIC host can invoke, and call back into well-defined host functions the RIC exposes (e.g., for inter-xApp messaging). This means the same xApp binary works on any vendor's RIC implementation, eliminating today's situation where xApps must be rewritten for each RIC.

### Use Case 1: MVNO Slice Scheduler

The authors demonstrate a two-level slice scheduler for Mobile Virtual Network Operators. The gNB host runs an inter-slice scheduler that divides physical resource blocks among slices (one per MVNO). Each slice's intra-scheduler is a Wasm plugin developed and signed by the MVNO. The plugin receives a list of UEs with channel quality indicators, buffer status, and long-term throughput, and returns a prioritized resource allocation. This lets MVNOs (Google Fi, Cricket Wireless, etc.) customize their own scheduling without the MNO having to run MVNO code natively on critical infrastructure.

The evaluation on an Intel NUC 12 (i7-1260P, 64GB RAM), srsRAN + Open5GS, operating in FDD band n3 (1.71–1.88 GHz), 15 kHz SCS, 10 MHz BW, shows three MVNOs coexisting with a Maximum Throughput, Round Robin, and Proportional Fair scheduler respectively — each meeting its target bitrate. A live scheduler swap (MT → PF → RR) is demonstrated without stopping the gNB or disconnecting any UEs, proving on-the-fly updates. Memory usage in plugin mode remains flat even with a deliberately leaking plugin (where native execution shows a linear increase leading to a crash).

### Use Case 2: Near-RT RIC Integration

WA-RAN wraps both sides of the E2 interface in plugins. The CU and DU each have communication plugins; xApps in the RIC are also plugins. This lets operators implement the RIC control plane with any distributed systems paradigm they prefer, while still achieving full interoperability with gNBs from different vendors. New features can be introduced as lightweight plugins rather than requiring standardization changes — a process that typically takes years. Additionally, running message decoding inside a sandbox prevents Denial-of-Service attacks where a malicious gNB or xApp sends crafted packets to crash the RIC.

### Open Problems and Limitations

The paper identifies several open research areas: fault tolerance (what happens when a plugin returns invalid output or misses a deadline), resource management in edge-constrained environments, narrowing the remaining performance gap versus native code, developing RAN-specific Wasm toolchains with appropriate compilers and sanitizers, and handling shared memory between components safely. The authors also note that Wasm, unlike eBPF, runs in user space, making it more suitable for RAN control-plane logic (which needs floating point arithmetic and unbounded loops that eBPF cannot support), while eBPF may remain better for user-plane data-path acceleration.

---

## 2. CORMO-RAN

**Full Title:** *CORMO-RAN: Lossless Migration of xApps in O-RAN*  
**Authors:** Antonio Calagna, Carla Fabiana Chiasserini (Politecnico di Torino / CNIT / Chalmers), Stefano Maxenti, Leonardo Bonati, and others  
**Venue:** arXiv 2506.19760, June 2025  
**Platform:** Prototype implemented as an rApp on Red Hat OpenShift; tested with commercial Radio Units and UEs on a private 5G testbed

### Motivation: The Energy Waste Problem in Dense Deployments

Dense urban deployments of O-RAN may require hundreds of concurrent xApps monitoring and controlling cells at peak traffic. However, traffic follows diurnal and spatial patterns: at 3 AM, most cells are sleeping and perhaps only a handful of xApps are needed. Yet the compute cluster — typically a Kubernetes-based near-RT RIC deployment — keeps all nodes running, burning energy to host xApps managing dormant cells.

CORMO-RAN's primary goal is to reclaim this energy by dynamically turning off underutilized compute nodes, while safely migrating the xApps running on those nodes to active ones. The challenge is that xApps are stateful — they maintain internal ML model weights, per-UE state, per-cell counters, and so on. Naively shutting down a node causes those xApps to restart cold on another node, losing state and temporarily degrading RAN control quality (e.g., a traffic steering xApp loses its per-UE flow models and must re-learn from scratch).

### Two Migration Strategies

CORMO-RAN studies two fundamental approaches to stateful xApp migration:

**Stateful Migration (SM)** migrates the entire container including its embedded memory. The container is checkpointed (using CRIU or equivalent), transferred, and restored on the target node. During transfer there is a downtime period (T_D) during which the xApp cannot respond to RAN events. Two variants are considered:
- **SM-MR (Minimize Resource):** Optimizes to use as few computational resources as possible during migration, accepting longer downtime.
- **SM-MD (Minimize Downtime):** Accepts higher resource usage during migration (running a shadow copy while transferring diffs) to minimize downtime.

**SDL-based Migration** uses O-RAN's Shared Data Layer — the distributed key-value store that xApps already use for RAN state — as the migration medium. The xApp's state is externalized to SDL before the node shuts down, and the new instance on the target node reads it back from SDL. When done correctly, this achieves zero downtime (T_D = 0) because the new instance can respond to RAN events from the moment it starts. The key requirement is that the xApp must have been architected to use SDL as its state store rather than keeping critical state purely in process memory.

### CORMO-RAN Orchestration Logic

CORMO-RAN is implemented as an rApp in the non-RT RIC, operating outside the near-RT control loop. It continuously monitors the number of active cells, traffic load, and current xApp workload. Based on a data-driven policy, it decides when to activate or deactivate compute nodes and triggers the appropriate migration strategy for each xApp that must move. Crucially, it selects the migration strategy per-xApp based on three factors: the xApp's state size (larger state = longer SM transfer), the xApp's timing constraints (how long a control gap is tolerable), and the available resources on the cluster at the time of migration.

### Experimental Results

Evaluation on a Red Hat OpenShift cluster with a private 5G testbed using commercial RUs demonstrates up to 64% reduction in RIC cluster energy consumption compared to a static (always-on) baseline — while maintaining xApp availability throughout. SDL-based migration achieves zero downtime under low cluster load, but at high load, defragmentation bottlenecks can arise because SDL itself may be contended. SM strategies provide stronger downtime guarantees but at the cost of compute overhead during migration. The paper provides practical guidelines for operators on which strategy to use given resource availability and xApp characteristics.

### Significance

CORMO-RAN is the first system to simultaneously address both the energy efficiency and the stateful migration problem in O-RAN compute clusters. It demonstrates that green O-RAN operation is achievable without sacrificing control continuity — a key blocker for production deployments where always-on compute costs are prohibitive.

---

## 3. MANATEE

**Full Title:** *MANATEE: A DevOps Platform for xApp Lifecycle Management and Testing in Open RAN*  
**Full Name:** Mesh Architecture for Radio Access Network Automation and TEsting Ecosystems  
**Authors:** Sofia Montebugnoli and collaborators  
**Venue:** arXiv 2601.14009, January 2026

### The Problem: xApp Delivery is Slow and Error-Prone

Today's O-RAN ecosystems lack the mature software delivery machinery that cloud-native application developers take for granted. There is no standard way to continuously test xApp code changes, progressively roll out new xApp versions to production with automatic rollback, or get fine-grained observability into xApp behavior inside the RIC. The result is a slow, manual, error-prone process: an xApp developer makes a change, manually builds and packages a container, manually deploys it to a test RIC, manually validates it against scripted scenarios, and then manually promotes it to production — often with weeks between code commit and production deployment.

More critically, the lack of traffic management means new xApp versions cannot be safely tested in production. If xApp v2 has a bug that causes it to generate conflicting control decisions with other xApps, there is no mechanism to route only 1% of traffic to it while 99% flows through the proven v1. This makes the RIC fragile and innovation slow.

### MANATEE's Approach: Service Mesh for the RIC

MANATEE integrates CI/CD pipelines with service mesh technologies to bring modern DevOps practices to xApp delivery. The core insight is that xApps, despite operating in a 5G RAN context, are fundamentally microservices exchanging messages via the RIC Message Router (RMR). Service mesh proxies (e.g., Istio/Envoy sidecars) can be injected alongside xApp containers to intercept this inter-xApp traffic, enabling:

**Progressive Deployment (Canary Releases):** New xApp versions receive a configurable fraction of RAN messages (e.g., start at 5%, ramp to 50%, then 100%) while the proven version continues handling the rest. If KPIs degrade, traffic is automatically shifted back. This is the industry-standard pattern for safe software rollout, now applied to RAN control applications.

**A/B Testing:** Two versions of an xApp can be deployed simultaneously and exposed to different cell groups or UE populations. Operators can measure which version performs better in production before committing.

**Circuit Breaking:** If an xApp version starts producing error responses or latency spikes (measured by the service mesh), the circuit breaker automatically routes traffic away from it, preventing cascading failures.

**Fine-Grained Observability:** Service mesh telemetry provides per-call latency distributions, error rates, and traffic volumes for every inter-xApp and xApp-to-RIC call, enabling operators to quickly diagnose why a control loop is degrading.

**CI/CD Integration:** MANATEE connects a Git repository to the RIC via automated pipelines. When a developer commits new xApp code, the pipeline automatically builds the container, runs unit and integration tests in a simulated RIC environment, and — if tests pass — deploys a canary to the production RIC.

### Architecture

MANATEE is prototyped on Kubernetes (to match the O-RAN Software Community's near-RT RIC, which runs on Kubernetes) with Istio as the service mesh. It extends the OSC Near-RT RIC with service mesh capabilities while remaining compatible with the standard RMR messaging layer. The existing RMR pub/sub communication paradigm is augmented, not replaced: MANATEE's proxies intercept RMR messages transparently.

### Experimental Results

The service mesh sidecar introduces less than 1 ms additional latency per message — acceptable given that near-RT RIC control loops operate at timescales of 10–1,000 ms. Canary deployments with fine-grained traffic control are demonstrated, and circuit-breaking mechanisms prevent conflict between A/B testing variants. The paper evaluates MANATEE across simulated, emulated (Colosseum), and real testbed environments, establishing that the same platform works across the full development-to-production lifecycle.

### Significance

MANATEE fills a critical gap: the path from xApp code to production RAN control has been entirely unformalized. By bringing CI/CD and service mesh to O-RAN, MANATEE enables the kind of continuous innovation and safe deployment that cloud-native services achieved a decade ago in web infrastructure. It is the first platform to combine these principles specifically for xApp lifecycle management.

---

## 4. PACIFISTA

**Full Title:** *PACIFISTA: Conflict Evaluation and Management in Open RAN*  
**Authors:** Pietro Brach del Prever, Salvatore D'Oro, Leonardo Bonati, Michele Polese, Maria Tsampazi, Heiko Lehmann, Tommaso Melodia  
**Venue:** arXiv 2405.04395, 2024  
**Platforms:** Colosseum wireless network emulator, OpenRAN Gym

### The xApp Conflict Problem

When multiple AI/ML-driven xApps operate simultaneously on a near-RT RIC, their independent optimization objectives can collide in ways that harm the network more than no control at all. The O-RAN Alliance's specifications categorize three types of conflict:

- **Direct conflicts:** Two xApps issue contradictory E2 Control messages targeting the same RAN parameter for the same cell/UE at the same time (e.g., both try to assign a UE to different cells).
- **Indirect conflicts:** Two xApps target different but interdependent parameters whose combined effect is harmful (e.g., one maximizes Power Headroom while another maximizes PRB allocation, together causing excessive interference).
- **Implicit conflicts:** Two xApps pursue different optimization goals whose joint execution produces non-obvious negative outcomes (e.g., a throughput-maximizer and an energy-saver deploy mutually incompatible resource allocation strategies, causing instability).

Even xApps with similar goals — say, two independent traffic-steering xApps — can create subtle conflicts. PACIFISTA demonstrates that users can experience a 16% throughput loss from such "similar-goal" conflicts, and that xApps with diametrically opposing goals (e.g., throughput maximization vs. energy minimization) can cause severe network instability resulting in up to 30% performance degradation.

### PACIFISTA's Architecture

PACIFISTA operates in the SMO environment, sitting above the RICs. Its workflow has three phases:

**Profiling:** Before deploying any xApp to production, PACIFISTA runs it in a sandbox (digital twin) environment across a range of operational scenarios: varying UE densities, signal quality distributions, traffic loads, and combinations with other already-deployed xApps. For each scenario, it captures the statistical distribution of the xApp's control actions and their effect on KPMs (Key Performance Metrics: throughput, latency, RSRP, etc.).

**Conflict Graph Construction:** PACIFISTA models the O-RAN system as a hierarchical conflict graph. Nodes represent xApps, RAN control parameters (RCPs), and KPMs. Edges represent "influences" relationships. By analyzing the profiles, PACIFISTA can identify when two xApps share influence over the same RCP or KPM, flagging potential conflict edges.

**Severity Evaluation:** For each flagged conflict pair, PACIFISTA applies statistical tests — Kolmogorov-Smirnov tests, Integral Area comparisons, and Chi-Square tests — to compare the KPM distributions under single-xApp versus dual-xApp operation. The output is a conflict severity index. If severity exceeds an operator-defined threshold, PACIFISTA either blocks the candidate xApp from deployment or marks existing xApps for removal.

### Key Findings

PACIFISTA can predict conflicts before production deployment by testing in a sandbox, preventing network degradation proactively. It provides granular information to operators: not just "these two xApps conflict" but which specific conditions trigger the conflict and how severe the impact is. However, the principal challenge is constructing complete statistical profiles across all relevant operational scenarios — for every new xApp, and every interaction with existing xApps, this matrix of profiles can grow combinatorially.

---

## 5. OREO

**Full Title:** *OREO: O-RAN intElligence Orchestration of xApp-based network services*  
**Authors:** F. Mungari, C. Puligheddu (Politecnico di Torino / CNIT), A. Garcia-Saavedra (NEC Labs Europe), C.F. Chiasserini  
**Venue:** IEEE INFOCOM 2024

### The Orchestration Problem

O-RAN envisions network operators offering a portfolio of services (beam management, network slicing, traffic steering, mobility robustness optimization, etc.) each realized by deploying specific xApps on the near-RT RIC. With a growing catalog of services and xApps, the operator faces an NP-hard optimization problem: which xApps to deploy, how many instances to run, and on which compute nodes — to maximize the number of active services while minimizing energy consumption and resource usage.

A naive approach deploys one xApp instance per service, but many services share semantically equivalent functions. Two different slicing services may both need a spectrum efficiency predictor as a subcomponent. Why deploy two identical predictors when one could serve both?

### OREO's Key Innovation: xApp Sharing

OREO introduces the concept of xApp sharing: if two services require a semantically equivalent xApp function and the xApp's output quality is sufficient for both services' requirements, a single xApp instance serves both. This has profound implications: OREO deploys up to 35% more services simultaneously with an average of 30% fewer xApp instances than a baseline that does not share — and with a similar reduction in resource consumption.

### Multi-Layer Graph Model

OREO captures the system with a multi-layer graph whose nodes span services, xApps, and compute resources. Service nodes are connected to their required xApp types; xApp nodes carry quality and resource annotations; compute nodes carry capacity constraints. Edges encode deployment and sharing relationships. OREO's algorithmic solution (a heuristic that closely approximates the NP-hard optimal) traverses this graph to find the assignment that maximizes active services. Services that cannot be fully satisfied (due to resource limits or quality constraints) are gracefully degraded rather than abruptly dropped.

### Results

Experimental tests on a proof-of-concept implementation demonstrate that OREO closely matches the optimum and consistently outperforms state-of-the-art baseline orchestration frameworks like OrchestRAN. Importantly, xApp sharing is quality-aware: OREO does not share an xApp if doing so would degrade service quality below an SLA threshold, ensuring that the efficiency gains do not come at the cost of service degradation.

---

## 6. COMIX

**Full Title:** *COMIX: Generalized Conflict Management in O-RAN xApps — Architecture, Workflow, and a Power Control Case*  
**Authors:** Anastasios Giannopoulos and four co-authors  
**Venue:** arXiv 2501.14619, January 2025  
**Funding:** EU HORIZON-JU-SNS-2023 (grant 101139073), REACT-6G project

### Focus: DRL xApps and the Network Digital Twin

While PACIFISTA focuses on statistical profiling across diverse scenarios, COMIX focuses specifically on conflicts between Deep Reinforcement Learning (DRL)-based xApps and introduces the Network Digital Twin (NDT) as a core component of conflict resolution.

The paper examines two DRL xApps for multi-channel power control: one maximizes aggregate UE data rates, the other maximizes system-level energy efficiency. These goals are fundamentally at odds — maximum throughput requires high transmission power, while energy efficiency requires low power. Left unconstrained, these xApps issue contradictory power commands to the same cells, causing rapid oscillations in transmission power and degrading both objectives.

### COMIX Architecture

**Standardized CMF Integration:** COMIX is designed around the O-RAN Alliance's Conflict Mitigation Framework (CMF) specification, which defines how detection and resolution should be structured within the near-RT RIC. COMIX implements this specification concretely, providing a reusable architecture that can be applied to xApp pairs beyond power control.

**Network Digital Twin (NDT):** Before applying a conflict resolution action to the live network, COMIX evaluates it in the NDT — a high-fidelity simulation of the radio environment that mirrors current network state. The NDT predicts the KPI impact of each candidate resolution policy (e.g., "let xApp A's decision take priority" vs. "average the two decisions" vs. "use a learned arbitration policy"), allowing COMIX to select the policy with the best predicted outcome without exposing the live network to experimental risk.

**Resolution Policies:** COMIX evaluates multiple resolution approaches including static priority ordering, weighted averaging of conflicting actions, and learned arbitration. Through NDT simulation, it identifies which policy best balances the competing objectives under current network conditions, and can adapt its choice as conditions change.

### Results

COMIX achieves 30% improvement in energy efficiency compared to the baseline without conflict management, while maintaining acceptable Quality of Service — a compelling demonstration that active conflict resolution can extract net value beyond simply preventing degradation. The validation combines simulation and real-world-inspired scenarios.

---

## 7. ORIGAMI / Centralized Conflict Coordination

**Context:** The term "ORIGAMI" appears in the O-RAN research community to describe a class of centralized coordination approaches at the Service Management and Orchestration (SMO) / non-RT RIC level. This section synthesizes the centralized coordination paradigm and the specific frameworks most aligned with the ORIGAMI concept.

### The Centralized Coordination Philosophy

A recurring theme across multiple research groups — Adamczyk et al. (Poznan), Corici et al. (TU Berlin / DFKI), and others — is that effective xApp conflict management requires a centralized entity with a global view of all deployed xApps, their objectives, their control parameter overlaps, and current network state. This entity, typically placed at the non-RT RIC or SMO layer, acts as an arbiter and coordinator rather than a reactive firefighter.

The key insight of centralized approaches is that conflicts can be prevented before they occur if a coordinator knows what every xApp is trying to do. Rather than each near-RT RIC independently discovering conflicts at runtime and scrambling to resolve them, the SMO-level coordinator maintains a conflict registry: a continuously updated mapping from (xApp pair, parameter domain, network state) to conflict risk level and recommended action.

### The Adamczyk CMF Framework (Conflict Mitigation Framework)

Cezary Adamczyk and Adrian Kliks (Poznan University of Technology) proposed a comprehensive Conflict Mitigation Framework (CMF) built into the near-RT RIC architecture itself. The CMF defines:

- **Message Flow Standards:** Precise specifications for how conflict detection events are communicated between near-RT RIC components (the xApp Manager, Conflict Mitigation Module, and E2 Termination), removing ambiguity about who is responsible at each step.
- **All Three Conflict Types:** Separate detection and resolution algorithms for direct, indirect, and implicit conflicts, each requiring different information flows. Direct conflicts can be detected immediately by watching E2 messages for parameter overlap; indirect conflicts require modeling parameter interdependencies; implicit conflicts require KPM trajectory analysis.
- **Control Loop Integration:** The CMF is woven into the existing near-RT RIC control loop rather than bolted on afterward, enabling low-latency conflict detection without introducing a separate bottleneck.

Simulation results show that enabling the CMF significantly improves network performance by balancing the conflicting control capabilities of competing xApps, with only a small impact on reaction time reliability.

### Corici et al. (TU Berlin): Converged 6G Control Plane Coordination

Research from Fraunhofer FOKUS and TU Berlin addresses the challenge of conflict mitigation in the converged 6G Open RAN control plane, noting that O-RAN specifications leave conflict management largely undefined. Their approach focuses on the boundary between near-RT and non-RT control: long-running optimization objectives expressed as rApp policies create the context within which xApp actions should be constrained, providing a hierarchical conflict prevention framework. Intent-based policies from the non-RT RIC explicitly forbid certain xApp action combinations, reducing the space of possible conflicts before near-RT execution.

### PACIFISTA as SMO-Level Coordinator

As described in Section 4, PACIFISTA itself functions as an SMO-level coordinator: it decides which xApps are even permitted to operate concurrently, based on pre-computed conflict severity indices. This is perhaps the most mature example of centralized conflict coordination in the literature: rather than resolving conflicts at runtime within the RIC, PACIFISTA prevents them proactively by blocking deployment of conflicting xApp combinations.

---

## 8. eZTrust / OZTrust

**Full Titles:**  
- *eZTrust: Context-Aware Zero-Trust for Microservices* (the upstream work)  
- *OZTrust: An O-RAN Zero-Trust Security System* (the O-RAN-specific extension)  
**Platform:** OSC (O-RAN Software Community) Kubernetes-based Near-RT RIC  
**Source:** University of Utah flux.utah.edu group

### The Security Problem in Near-RT RIC

The near-RT RIC is a multi-tenant platform: it simultaneously hosts xApps from potentially different vendors, operators, and third-party developers, all sharing the same Kubernetes pod network, the same SDL database, and the same E2 termination process. This architecture, while flexible, creates a severe trust boundary problem.

As Hung et al. (IEEE Open Journal of Comms, 2024) discovered through a CVE-level analysis of the O-RAN H-Release implementation: the existing near-RT RIC does NOT enforce the access control permissions specified by O-RAN Alliance WG11. A malicious or compromised xApp can:
- Illegally access REST APIs belonging to other xApps
- Inject malicious E2 Control messages that are then forwarded to gNBs as if legitimate
- Read UE data from SDL that it has no legitimate reason to access
- Trigger a complete RAN disruption by corrupting shared state

Two CVEs were filed from this analysis (CVE-2023-42358, CVE-2023-41628). The problem is fundamental to the open, disaggregated nature of O-RAN — and traditional perimeter security ("the RIC cluster is trusted") is wholly insufficient.

### eZTrust: The Foundation

eZTrust is a generic zero-trust framework for microservice environments that operates at the network packet level using eBPF (extended Berkeley Packet Filter). Its design has three components:

**Context Tracing:** When a microservice is launched, eZTrust's eBPF programs discover its context: application name, version, SSL version, geographic location, container image hash, OS version. These contexts are stored in kernel-space maps, keyed by a compact tag.

**Packet Tagging:** Every network packet leaving a microservice is tagged with that service's context tag, embedded in a custom packet header field.

**Packet Verification:** At the receiving microservice, eBPF programs verify that the incoming packet's tag corresponds to a context that is permitted to communicate with this service, according to a policy. Packets from unpermitted senders are silently dropped before reaching the application layer.

This mechanism operates entirely in kernel space via eBPF, adding negligible latency (single-digit microseconds) while providing cryptographically assured context authentication for every inter-service communication.

### OZTrust: O-RAN Extension

OZTrust extends eZTrust for the O-RAN near-RT RIC environment, where eZTrust's generic context tracing does not understand xApp-specific concepts. OZTrust adds:

**xApp-Specific Context Tracing:** New tracing components discover xApp-specific contexts: which E2 service models the xApp is subscribed to, which RAN parameters it is permitted to modify, which SDL namespaces it is authorized to read/write, and which other xApps it is allowed to message.

**Policy Creation:** Operators create policies based on xApp contexts rather than just generic microservice properties. A traffic steering xApp might be permitted to read cell load data from SDL and send E2 RC Control messages, but explicitly prohibited from accessing UE location data or sending E2 Policy messages — enforced at the packet level regardless of application-layer behavior.

**Integration with OSC RIC:** OZTrust injects eBPF programs into the ingress and egress of every xApp container, the SDL, and the E2 termination process. This is done without modifying xApp source code — existing xApps gain zero-trust enforcement transparently.

The system is demonstrated protecting communication among three real-world xApps: Anomaly Detection (AD), Traffic Steering (TS), and QoE Predictor (QP), showing that unauthorized cross-xApp data access and E2 message injection are blocked with negligible overhead.

---

## 9. BubbleRAN

**Context:** BubbleRAN is not a single academic paper but a commercial research platform and product suite from a French startup. It represents arguably the most mature production-grade ultra-lean RIC implementation available, and is extensively referenced in recent academic papers as a testing platform.

### Platform Overview

BubbleRAN's flagship product relevant to near-RT RIC is **RIC-Sphere**, their O-RAN-compliant RIC platform. Key design principles:

**Sub-millisecond Control Loops:** BubbleRAN explicitly targets fast closed loops down to sub-millisecond (sub-ms) timescales — significantly faster than the 10–1,000 ms near-RT RIC specification. This is made possible by their **FlexRIC** implementation, which eliminates the protocol overhead of the standard O-RAN E2 ASN.1 encoding and uses a more efficient custom encoding (the FlexRIC E2 interface) while remaining semantically compatible. Academic publications using BubbleRAN/FlexRIC have demonstrated control loop round trips of 650 µs and below.

**Ultra-Lean Architecture:** BubbleRAN's near-RT RIC is designed for minimal overhead. Unlike the OSC Near-RT RIC (which is a heavy Kubernetes-based deployment with multiple microservices, message routers, and databases), BubbleRAN's implementation is a compact, low-latency process that prioritizes deterministic response time over feature richness. This makes it suitable for deployment on resource-constrained edge hardware rather than cloud datacenters.

**Multi-Vendor Interoperability:** RIC-Sphere ships with a Proxy-E2 Agent for legacy RAN adaptation — essentially the communication plugin concept (similar to WA-RAN), allowing it to connect to RAN vendors whose E2 implementations differ from the standard. Custom O-RAN Service Models (TC: traffic control, SC: Slice Control, LS: Layer stats) are provided beyond the standard KPM, RC, CCC, LLC models.

**Integrated Non-RT RIC and SMO:** BubbleRAN's SMO-Sphere provides non-RT RIC and SMO functionality, enabling an end-to-end platform from intent specification down to radio control in a single integrated system. The company's recent MX-AI work (arXiv 2508.09197) integrates LLM-based agents with the SMO, using BubbleRAN's FlexRIC as the closed-loop enforcement engine for LLM-generated policies.

### Academic Deployments and Validation

BubbleRAN/FlexRIC is used as the testbed platform in multiple high-profile papers including CORMO-RAN (Red Hat OpenShift cluster with BubbleRAN RIC), MX-AI (indoor OAI gNB + UEs + BubbleRAN FlexRIC for LLM-driven RAN control), and numerous MANATEE-style deployments. The 650 µs control loop figure comes from FlexRIC benchmarks where a simple xApp receives an E2 Indication, makes a decision, and sends an E2 Control message — completing the round-trip in well under 1 ms.

### SDK and Developer Ecosystem

BubbleRAN provides Python and C++ xApp SDKs with pre-built abstractions for E2SM handling, SDL access, and A1 policy parsing. Their documentation includes complete worked examples (e.g., an SLA enforcement xApp that monitors per-slice throughput via KPM Indication messages and adjusts PRB allocations via RC Control). The lifecycle from xApp initialization to graceful termination is formalized in their developer guides and directly referenced by MANATEE as one of the open-source foundations for lifecycle management research.

---

## 10. LLM-Based Conflict Resolution in O-RAN

**Relevant Works:**  
- *LLM-xApp: A Large Language Model Empowered Radio Resource Allocation xApp* (NDSS FutureG workshop 2025)  
- *Multi-Agentic AI for Conflict-Aware rApp Policy Orchestration in Open RAN* (arXiv 2603.07375, 2026)  
- *LLM-Based Net Analyzer rApp for Explainable and Safe Automation in O-RAN Non-RT RIC* (arXiv 2603.13775, 2026)  
- *MX-AI: Agentic Observability and Control Platform for Open and AI-RAN* (arXiv 2508.09197, 2025)

### The LLM Opportunity in O-RAN

O-RAN's promise of intelligent, intent-driven network management is a natural fit for Large Language Models: operators want to express goals in natural language ("maintain 10 Mbps for all eMBB UEs while minimizing energy use"), have the system understand the intent, translate it into xApp configurations, detect conflicts with other intents, and explain its decisions. Traditional ML approaches can optimize metrics but cannot reason about novel scenarios, explain their decisions to human operators, or generalize to intent combinations they haven't been explicitly trained on.

### LLM-xApp: Intent-Driven Resource Allocation

The LLM-xApp paper presents the first xApp powered by an LLM for slice resource allocation in O-RAN. The system uses structured meta-prompts to encode the network state, optimization history, and quality targets. The LLM iteratively generates resource allocation decisions, which are evaluated, fed back into the meta-prompt, and refined. This iterative prompting enables the LLM to converge on near-optimal PRB allocations without RL training, generalizing to new traffic patterns without retraining.

### Multi-Agentic Conflict-Aware rApp Orchestration

The most ambitious recent work (arXiv 2603.07375) builds a three-agent LLM system for rApp policy orchestration with conflict awareness:

**Perception Agent:** Monitors the live network state and incoming intent requests, extracting relevant context (which cells are affected, which parameters are involved, which existing rApps have active policies in the same domain).

**Reasoning Agent:** Armed with Retrieval-Augmented Generation (RAG) over a knowledge base of O-RAN specifications and historical conflict patterns, it analyzes whether the incoming intent can be satisfied without conflicting with existing policies. It synthesizes an intent-aligned control pipeline.

**Refinement Agent:** Uses analogical reasoning from memory of past deployment decisions to incrementally refine the proposed deployment, catching edge cases that the Reasoning Agent might miss.

**Results:** Over 70% improvement in deployment accuracy and 95% reduction in reasoning cost compared to zero-shot GPT-4 baselines, with demonstrated zero-shot generalization to intent combinations not seen during development.

### Net Analyzer rApp

The Net Analyzer rApp demonstrates LLMs for explainable network diagnosis in the non-RT RIC. The rApp is event-triggered: when a mobility anomaly is detected (e.g., ping-pong handovers), the LLM interprets the event, inspects relevant logs via tool-gated access (it cannot directly modify configuration without explicit human approval), proposes minimal configuration changes (e.g., adjusting A3 offset thresholds), and waits for operator sign-off before executing. This "human-in-the-loop" design is essential for production deployment — current LLMs can hallucinate, and an unchecked LLM issuing E2 Control messages could cause severe network disruption.

### MX-AI: End-to-End Agentic RAN Control

BubbleRAN's MX-AI system is the most complete integration of LLM agents with a production-grade RIC. A reasoning agent with 12-second inference time (suitable for non-RT timescales) handles planning, assurance, and orchestration, while xApps in the BubbleRAN FlexRIC near-RT RIC handle sub-second enforcement. The agent answers 50 operational queries with measured accuracy, action precision, end-to-end latency, and GPU footprint — the first rigorous benchmark of LLM-driven RAN automation on a real testbed.

---

## 11. Cross-Cutting Issues in Near-RT RIC / xApp Systems

The papers above, collectively, illuminate a set of deep and interrelated challenges that any production O-RAN xApp deployment must confront. Below is a systematic analysis of the principal issues.

### 11.1 Interoperability and the "Single-Vendor Illusion"

Despite the O-RAN Alliance's standardization efforts, the E2 interface has enough specification flexibility that vendor A's RIC cannot reliably control vendor B's gNB without extensive integration testing and often custom shims. The functional splits (7.2x between O-DU and O-RU) face similar issues. Operators who need production-grade reliability are pushed toward single-vendor stacks, recreating the vendor lock-in that O-RAN was supposed to eliminate. WA-RAN identifies this as the existential threat to O-RAN and proposes Wasm as the solution; OZTrust and MANATEE deal with the security and lifecycle consequences of the multi-vendor deployment model.

### 11.2 xApp Conflicts: Detection, Prevention, and Resolution

This is arguably the most studied challenge in the near-RT RIC literature. Conflicts arise because:
- xApps are developed independently by different vendors with no coordination
- They share E2 control parameter namespaces (same cells, same UEs, same parameters)
- Their ML models were trained independently and have no knowledge of each other
- Dynamic network conditions create conflict scenarios not anticipated during development

The O-RAN Alliance WG3 has published technical specifications on conflict mitigation but has not standardized the implementation. The result is a fragmented research landscape: PACIFISTA (profiling + severity analysis), COMIX (NDT simulation + DRL-specific resolution), Adamczyk CMF (standards-aligned detection/resolution built into the RIC), xApp distillation (learning from conflicting xApps to create a unified replacement), and multi-agentic LLM orchestration all propose different approaches at different layers of the stack.

No single solution handles all three conflict types (direct, indirect, implicit) across all xApp implementations. Direct conflicts are the most tractable; implicit conflicts — where the combined effect of two independently reasonable decisions is harmful — may require global optimization and remain largely unsolved.

### 11.3 State Management and Fault Tolerance

xApps that use ML models or track per-UE state are inherently stateful, but O-RAN's standard management framework treats xApps as stateless containers. This creates problems in three scenarios:
- **Node failure:** When the Kubernetes node running an xApp fails, the replacement xApp starts cold, losing all learned state.
- **Planned scaling:** CORMO-RAN is specifically about this — turning off underutilized nodes requires migrating state safely.
- **Software updates:** Deploying a new xApp version while maintaining control continuity requires either dual-running or state transfer.

The SDL (Shared Data Layer) is O-RAN's designated solution for state externalization, but many xApps — especially those with large, complex ML models — store critical state in process memory for performance reasons. CORMO-RAN demonstrates that SDL-based migration can achieve zero downtime, but requires xApps to be architected accordingly from the start.

### 11.4 Timing and Real-Time Guarantees

The near-RT RIC specification nominally allows control loops from 10 ms to 1 s. In practice:
- Standard OSC near-RT RIC implementations with E2 ASN.1 encoding are often in the 100–500 ms range
- Applications like slice SLA enforcement, interference coordination, and handover management would benefit from tighter loops
- BubbleRAN/FlexRIC demonstrates 650 µs round trips, but achieving this requires stripping overhead from the standard stack

The tension between standard compliance (which ensures interoperability) and performance (which often requires non-standard optimizations) is a fundamental challenge. WA-RAN's plugin approach can help by optimizing the communication plugin per-deployment without changing the standard interface semantics.

### 11.5 Security: The Third-Party xApp Threat Model

The near-RT RIC's architecture as a multi-tenant xApp platform introduces a threat model that traditional telecom security frameworks were not designed for. A single compromised or malicious xApp can:
- Issue unauthorized E2 Control messages causing misdirected handovers or misconfigured power settings
- Read sensitive UE data (location, traffic patterns) from shared SDL databases
- DoS the RIC by flooding the E2 termination with crafted messages
- Corrupt shared state used by other xApps

OZTrust/eZTrust addresses this with eBPF-based packet-level enforcement; ZTRAN (Abdalla et al.) proposes xApp-based zero trust subsystems; ZT-RIC (arXiv 2411.07128) proposes privacy-preserving data access controls. None of these are yet standardized or widely deployed.

### 11.6 Lack of Mature Tooling for xApp Lifecycle Management

Building, testing, deploying, monitoring, and updating xApps requires integrating Kubernetes orchestration, CI/CD pipelines, RIC-specific testing frameworks, over-the-air RF testbeds, and conflict analysis tools — none of which are currently integrated out of the box. MANATEE attempts this integration for the first time. The absence of mature tooling means xApp development cycles measured in months rather than days, significantly slowing innovation.

---

## 12. Industry Mitigation & Production Use

### 12.1 Operators: Cautious Pragmatism

**AT&T:** Has committed to a full Ericsson O-RAN deployment, effectively eliminating the multi-vendor xApp opportunity in the near term. This protects against integration complexity but forfeits the flexibility O-RAN promised. xApp deployment at AT&T is correspondingly minimal and under strict Ericsson control.

**Vodafone UK (Samsung), Vodafone Italy (Nokia):** Similar pattern — single-vendor O-RAN deployments for different national subsidiaries, trading interoperability for reliability. Vodafone has participated in O-RAN plugfests and openly published white papers on O-RU/O-DU integration challenges.

**Rakuten Mobile (Japan):** Considered the furthest along in actually deploying a multi-vendor O-RAN with xApps in production. Uses a combination of in-house xApps (traffic steering, energy saving) and vendor-provided xApps on an Altiostar (now Fujitsu)/other RIC. Rakuten's experience confirmed the difficulty of the conflict problem at scale.

**NTT DOCOMO and Vodafone joint work:** Published a white paper on O-RU/O-DU integration challenges in the 7.2x functional split, one of the most detailed industry analyses of the interoperability problem that WA-RAN aims to solve.

### 12.2 RIC Vendors: Building Conflict Management In

**Ericsson:** The Ericsson Intelligent Automation Platform (EIAP) incorporates an xApp/rApp marketplace with conflict detection capabilities built into the SMO. Ericsson's approach is to pre-certify xApps in their ecosystem and enforce parameter namespace separation at the RIC level. Their RSAC (RAN Slice Assurance Coordinator) xApp demonstrates closed-loop SLA enforcement in commercial deployments.

**Nokia:** The Nokia MantaRay Network Intelligence platform includes conflict avoidance through hierarchical policy management: higher-level rApps set bounds within which lower-level xApps must operate, reducing the space of possible conflicts. Nokia has demonstrated this in live network trials.

**Samsung:** Samsung's O-RAN RIC includes an xApp Conflict Manager that uses parameter ownership registration — xApps must declare which parameters they intend to modify before deployment, and the RIC rejects combinations that would create direct conflicts. This is a simpler but more conservative approach that prevents some conflicts at the cost of limiting concurrent xApp functionality.

### 12.3 Open-Source Platforms

**O-RAN Software Community (OSC):** The Near-RT RIC H-Release (and subsequent I/J releases) progressively addresses security (adding XRF authentication), conflict management (integrating an xApp Manager that can enforce parameter scope), and SDL improvements. The CVEs discovered by Hung et al. prompted security patches in subsequent releases.

**OpenAirInterface (OAI) / FlexRIC:** The OAI project's FlexRIC is used in dozens of academic publications (including by BubbleRAN) as the reference high-performance near-RT RIC. It achieves sub-ms loops and is the basis for most BubbleRAN latency claims. OAI has an active community contributing xApp examples and integration guides.

**OpenRAN Gym / Colosseum:** Northeastern University's OpenRAN Gym (used in PACIFISTA, ColO-RAN, and many others) provides a reproducible large-scale O-RAN experimentation platform combining Colosseum SDR emulator with O-RAN-compliant software. It is the closest thing to a standardized O-RAN research testbed.

### 12.4 O-RAN Alliance Standards Progress

The O-RAN Alliance has published:
- **WG3 Conflict Mitigation Technical Specification (2024):** Formally defines direct, indirect, and implicit conflicts and mandates that near-RT RICs implement a Conflict Mitigation Function — but does not specify the implementation.
- **WG11 Zero Trust Architecture White Paper (2024):** Outlines a roadmap for embedding zero trust principles into O-RAN, aligning with NIST ZTMM. Current compliance is at "Initial" level; "Advanced" remains aspirational.
- **WG6 O-Cloud Lifecycle Management:** Addresses xApp deployment, scaling, and upgrade management at the Kubernetes layer, providing hooks that MANATEE-style platforms can use.

### 12.5 Summary: Where the Gaps Remain

Despite significant progress, the following issues remain largely unsolved in production:

- **Implicit conflict detection** at scale without prohibitive profiling overhead
- **Zero-downtime stateful migration** for xApps with large ML models
- **Sub-50ms control loops** in standard-compliant multi-vendor deployments
- **Certified multi-vendor xApp interoperability** — the O-RAN plugfest process is improving but remains far from plug-and-play
- **LLM-driven autonomous control** — exciting research results but not yet production-safe due to hallucination and auditability concerns
- **Standardized conflict resolution** — WG3 defines the framework but not the algorithm, leaving operators to implement their own solutions

---

*Document compiled from: WA-RAN (HotNets '24), CORMO-RAN (arXiv 2506.19760), MANATEE (arXiv 2601.14009), PACIFISTA (arXiv 2405.04395), OREO (IEEE INFOCOM 2024), COMIX (arXiv 2501.14619), OZTrust (Utah flux.utah.edu), BubbleRAN product documentation and academic publications, LLM-xApp (NDSS FutureG 2025), Multi-Agentic rApp Orchestration (arXiv 2603.07375), MX-AI (arXiv 2508.09197), O-RAN Alliance WG3/WG11/WG6 specifications, and industry operator/vendor publications through May 2026.*