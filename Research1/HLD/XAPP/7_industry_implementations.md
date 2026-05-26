# 7. Current Industry Implementations (2025–2026 Context)

To position our architecture effectively, it is vital to understand how major telecommunications vendors, open-source platforms, and operators are currently implementing the Near-RT RIC and dealing with the constraints of the O-RAN architecture. This section draws on vendor publications, operator deployment reports, and the O-RAN Alliance's standardization progress through May 2026.

## 7.1. The Shift to Non-RT RIC and rApps

The most notable trend in 2025/2026 is a commercial pivot away from the Near-RT RIC (xApps) in favor of the Non-RT RIC (rApps). 

*   Because the 10ms–1s E2 control loop is so technically demanding and prone to the container/latency issues outlined in Document 1, operators have struggled to deploy complex Near-RT xApps reliably. The cross-cutting analysis (Section 11.4 of the paper survey) confirms that "standard OSC near-RT RIC implementations with E2 ASN.1 encoding are often in the 100–500 ms range."
*   Instead, vendors are pushing automation into **rApps**, which operate on >1-second loops over the A1 interface, managing slower tasks like long-term energy saving and policy updates. The LLM-based systems (MX-AI, Multi-Agentic rApp Orchestration) are explicitly designed for Non-RT timescales with 12-second inference times.
*   **Our Position:** Our architecture directly addresses the technical debt that caused this pivot, providing a viable path to resurrecting high-speed, sub-second xApp commercial viability. If the fundamental cold-start, security, and E2 disruption problems are resolved, operators may re-engage with Near-RT xApps for latency-critical tasks (URLLC slice assurance, real-time interference coordination) currently deemed too risky.

## 7.2. Vendor Landscape

### Ericsson (EIAP)
*   **Architecture:** Ericsson's Intelligent Automation Platform (EIAP) focuses heavily on the Service Management and Orchestration (SMO) layer and the Non-RT RIC. 
*   **Implementation:** Ericsson approaches the Near-RT RIC with caution, preferring to handle critical, low-latency radio resource management inside their proprietary O-DU/O-CU software rather than exposing it to third-party xApps, due to security and performance concerns. Their RSAC (RAN Slice Assurance Coordinator) xApp demonstrates closed-loop SLA enforcement in commercial deployments — but within a tightly controlled, pre-certified ecosystem.
*   **Conflict Management:** EIAP incorporates an xApp/rApp marketplace with conflict detection capabilities built into the SMO. Ericsson pre-certifies xApps and enforces parameter namespace separation at the RIC level — a conservative approach that prevents some conflicts at the cost of limiting the multi-vendor openness that O-RAN promises.
*   **Operator Adoption:** AT&T has committed to a full Ericsson O-RAN deployment, effectively eliminating the multi-vendor xApp opportunity in the near term. This protects against integration complexity but forfeits the flexibility O-RAN promised.

### Nokia (MantaRay & Juniper Acquisition)
*   **Architecture:** In late 2025, Nokia absorbed Juniper Networks' highly regarded Near-RT RIC assets to bolster its MantaRay SMO platform. NTT DOCOMO and Vodafone have published joint white papers on O-RU/O-DU integration challenges in the 7.2x functional split — one of the most detailed industry analyses of the interoperability problem.
*   **Implementation:** The Juniper RIC was built on a cloud-native Kubernetes microservices architecture. It heavily utilizes standard containerization and relies on robust conflict management modules to handle multi-vendor xApps. However, it still suffers from the fundamental container cold-start and state-transfer limitations inherent to K8s.
*   **Conflict Management:** Nokia's MantaRay includes conflict avoidance through **hierarchical policy management**: higher-level rApps set bounds within which lower-level xApps must operate, reducing the space of possible conflicts. This is philosophically aligned with the ORIGAMI/centralized coordination approach.

### Samsung
*   **Architecture:** Samsung's O-RAN RIC includes an xApp Conflict Manager that uses **parameter ownership registration** — xApps must declare which parameters they intend to modify before deployment, and the RIC rejects combinations that would create direct conflicts.
*   **Limitation:** This is a simpler but more conservative approach that prevents direct conflicts at the cost of limiting concurrent xApp functionality. It does not address indirect or implicit conflicts (the harder problems that PACIFISTA and GraphSAGE target).
*   **Operator Adoption:** Vodafone UK deploys Samsung-only O-RAN, and Vodafone Italy deploys Nokia-only — the same single-vendor pattern seen with Ericsson.

### VMware (Broadcom)
*   **Status:** VMware's Distributed RIC was once a leading vendor-neutral implementation. Following the Broadcom acquisition and corporate restructuring, their presence in the standalone RIC market has significantly diminished.
*   **Legacy:** Their architecture relied heavily on a distributed, shared Redis database (functionally similar to SDL) to manage E2 state and K8s for xApp orchestration.

## 7.3. Open-Source Platforms

### O-RAN Software Community (OSC)
The Near-RT RIC H-Release (and subsequent I/J releases) progressively addresses the issues identified in our analysis:
*   **Security:** Added XRF (xApp Registration Framework) authentication following CVE-2023-41628/42358 disclosures by Hung et al. Subsequent CVEs (CVE-2024-34046/34047/34048) prompted further patches.
*   **Conflict Management:** Integrated an xApp Manager that can enforce parameter scope — a step toward Samsung-style parameter ownership.
*   **SDL Improvements:** Enhanced Shared Data Layer for cross-xApp state coordination.
*   **Remaining Gaps:** No hitless E2 subscription transfer, no Wasm or lightweight runtime support, no hardware acceleration integration.

### OpenAirInterface (OAI) / FlexRIC
OAI's FlexRIC is used in dozens of academic publications as the reference high-performance Near-RT RIC. It is the basis for BubbleRAN's latency claims (650µs control loops). OAI has an active community contributing xApp examples and integration guides. It achieves performance through aggressive software optimization and custom E2 encoding — a different trade-off than our hardware-acceleration approach.

### OpenRAN Gym / Colosseum
Northeastern University's OpenRAN Gym provides a reproducible large-scale O-RAN experimentation platform combining Colosseum SDR emulator with O-RAN-compliant software. It is used as the testbed in PACIFISTA, ColO-RAN, and many other papers. It is the closest thing to a standardized O-RAN research testbed and would be the natural evaluation platform for our architecture.

## 7.4. Multi-Vendor Operator Experience

### Rakuten Mobile (Japan)
Considered the furthest along in actually deploying a multi-vendor O-RAN with xApps in production. Uses a combination of in-house xApps (traffic steering, energy saving) and vendor-provided xApps on an Altiostar (now Fujitsu)/other RIC. **Rakuten's experience confirmed the difficulty of the conflict problem at scale** — making them the most relevant real-world validation that the issues documented in Document 1 are not merely theoretical.

### Single-Vendor Pattern
AT&T (Ericsson-only), Vodafone UK (Samsung-only), Vodafone Italy (Nokia-only) — the dominant pattern is operators trading O-RAN's promised multi-vendor flexibility for single-vendor reliability. WA-RAN's observation that "operators are sourcing their entire O-RAN stacks from single vendors, defeating O-RAN's core purpose" is directly supported by these deployments.

## 7.5. O-RAN Alliance Standards Progress

| Standard | Year | Relevance to Our Work |
|---|---|---|
| **WG3 Conflict Mitigation Technical Specification** | 2024 | Formally defines direct/indirect/implicit conflicts; mandates CMF but does not specify implementation. Our host-function CM is a concrete CMF implementation. |
| **WG11 Zero Trust Architecture White Paper** | 2024 | Outlines ZTA roadmap; acknowledges current compliance at "Initial" level. Our Wasm sandbox provides "Advanced"-level isolation by construction. |
| **WG6 O-Cloud Lifecycle Management** | 2024 | Addresses xApp deployment, scaling, and upgrade via K8s hooks. Our K8s Operator CRD approach extends this for Wasm binary deployment. |
| **E2AP v7.00 / E2GAP v7.00** | April 2025 | Added RIC Subscription State Control — standardization-level acknowledgment of the hitless subscription gap. Our DLB-based approach resolves this at the platform level. |

## 7.6. Summary: Where the Gaps Remain in Industry

Despite significant progress, the following issues remain largely unsolved in production:

| Gap | Status | Our Contribution |
|---|---|---|
| **Implicit conflict detection at scale** | Active research (GraphSAGE, PACIFISTA) | Out of scope — we provide enforcement |
| **Zero-downtime stateful migration** | CORMO-RAN (with constraints) | Eliminated via DLB pointer swap |
| **Sub-50ms control loops in standard-compliant deployments** | BubbleRAN (non-standard encoding) | Target via hardware acceleration (standard-compliant) |
| **Certified multi-vendor xApp interoperability** | O-RAN plugfest (improving slowly) | WA-RAN-style plugin approach could help |
| **LLM-driven autonomous RAN control** | MX-AI (promising but not production-safe) | Orthogonal — our platform could host LLM-based xApps |
| **Standardized conflict resolution algorithm** | WG3 defines framework, not algorithm | Algorithm-agnostic enforcement substrate |
