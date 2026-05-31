# Deep Paper Analysis: DPU/SmartNIC AI Network Slicing Research Stack

**Prepared:** May 2026  
**Context:** Research landscape analysis for DPU/SmartNIC-based AI network slicing with proactive bandwidth prediction/reservation/forgo based on QoS and policies  
**Scope:** 40 papers across control-plane AI, data-plane ML, proactive slice provisioning, federated learning, and AI-RAN vision

---

## Table of Contents

1. [Cluster 1 — The dApp Lineage (Real-Time Control at the RAN Edge)](#cluster-1)
2. [Cluster 2 — The Data Plane Intelligence Foundations](#cluster-2)
3. [Cluster 3 — Proactive Slice Provisioning (The Prediction Side)](#cluster-3)
4. [Cluster 4 — Distributed/Federated Slice AI](#cluster-4)
5. [Cluster 5 — VNF/Slice Scaling (2025–26 State of the Art)](#cluster-5)
6. [Cluster 6 — The AI-RAN Vision Papers](#cluster-6)
7. [Cluster 7 — Foundational and Survey Papers](#cluster-7)
8. [Cross-Paper Synthesis](#cross-paper-synthesis)
9. [Recommended Reading Order](#recommended-reading-order)

---

## Cluster 1 — The dApp Lineage (Real-Time Control at the RAN Edge) {#cluster-1}

### Paper 1: dApps: Distributed Applications for Real-Time Inference and Control in O-RAN

| Field | Detail |
|---|---|
| **Authors** | S. D'Oro, M. Polese, L. Bonati, H. Cheng, T. Melodia |
| **Venue** | IEEE Communications Magazine, Vol. 60, No. 11, pp. 52–58, Nov 2022 |
| **DOI** | 10.1109/MCOM.002.2200079 |
| **Citations** | ~165 (as of May 2026) |
| **Institution** | Northeastern University (Institute for the Wireless Internet of Things) |
| **ArXiv** | 2203.02370 |

#### What It Actually Does

This paper proposes the notion of **dApps** — distributed applications that complement existing xApps and rApps by allowing operators to implement fine-grained, data-driven management and control in real-time at the Central Units (CUs) and Distributed Units (DUs). dApps receive real-time data from the RAN as well as enrichment information from the near-real-time RIC, and execute inference and control of lower-layer functionalities. This enables use cases with stricter timing requirements than those considered by current O-RAN specifications, which lack a practical approach for executing real-time control loops operating at timescales below 10 ms.

#### The Precise Problem It Solves

The xApp/rApp control stack runs at 10 ms to >1 s timescales. Scheduling, beam management, and power control all need sub-10 ms reactions. The paper introduces dApps as co-located microservices that live *inside* the CU/DU — not remotely in the RIC — to close that timing gap.

#### Deep Insight

This is the conceptual ancestor of the entire real-time RAN control movement. What makes it seminal is that it does not propose a new algorithm — it proposes a new **architectural slot** in the O-RAN stack. Before dApps, there was a clean discontinuity between the data plane (handling packets at µs timescale) and the control plane (making decisions at ms timescale). dApps create a middle layer that has data-plane *proximity* with control-plane *intelligence*.

This is precisely analogous to what a DPU-resident AI would be doing for bandwidth slicing: not a new ML model, but a new place *in the architecture* where ML can live. The paper positions dApps as microservices — which means containerized, lifecycle-managed, orchestrated software processes. The DPU generalisation moves this from software process to hardware-accelerated inference engine.

#### Critical Limitation

The 2022 paper is conceptual — it proposes the architecture but does not demonstrate it running at true line-rate or with quantified sub-ms latency numbers. This limitation is directly addressed by Paper 8 (Lacava et al., 2025) below.

#### Key Relationships

- **Builds on:** O-RAN Alliance specifications; xApp/rApp framework
- **Followed by:** Paper 8 (dApps 2025 — full implementation with latency measurements)
- **Cited by:** CollabORAN, multi-scale agentic AI framework, REAL paper

#### Relevance to DPU/SmartNIC Research

**Maximum.** The dApp concept is the software-side analogue of DPU-resident AI. Your work could be framed as: *"What happens when the dApp is not a software microservice on a general CPU, but a hardware-accelerated AI running on a DPU's ARM/FPGA pipeline?"* This framing is publishable and positions the work clearly in the existing taxonomy.

---

### Paper 8: dApps: Enabling Real-Time AI-Based Open RAN Control

| Field | Detail |
|---|---|
| **Authors** | A. Lacava, L. Bonati, N. Mohamadi, R. Gangula, F. Kaltenberger, P. Johari, S. D'Oro, F. Cuomo, M. Polese, T. Melodia |
| **Venue** | Computer Networks (Elsevier), Volume 269, 2025 |
| **DOI** | 10.1016/j.comnet.2025.111342 |
| **Citations** | ~47 (as of May 2026) |
| **ArXiv** | 2501.16502 |

#### What It Actually Does

This paper extends the O-RAN architecture into the real-time and user-plane domains. It proposes a reference architecture for dApps that defines their lifecycle and interactions with RAN nodes, enabling coexistence and coordination with xApps and rApps via a new **E3 interface** and the existing E2 interface. Control actions or inferences are executed without disrupting RAN operations, with the API design ensuring concurrent functionality across multiple dApps.

#### The Key Measured Result

The paper demonstrates the feasibility of standardized real-time control loops via dApps, **achieving average control latency below 450 microseconds** — addressing the need for control loops at timescales below the 10 ms enabled by xApps. This is the first published proof that O-RAN-adjacent control can cross the sub-millisecond barrier.

#### Deep Insight — The 450 µs Number

Read this carefully: 450 µs is for *control decision + actuation* when the dApp is a software process on a standard Linux server *co-located at the DU*. This is fast, but it still consumes host CPU cycles. A DPU-resident AI could potentially reduce this further by:

1. Freeing the host CPU entirely
2. Running directly on the network card's inline processing engine
3. Eliminating the software interrupt path between NIC and OS

That step — from 450 µs software dApp to <100 µs DPU-resident inference — has not been taken in any published work as of May 2026. This is the latency gap your research can close.

#### Deep Insight — The E3 Interface

The E3 interface is the paper's under-discussed contribution. It is a new standardization proposal that allows dApps to exchange control signals bidirectionally with both the RAN stack and the near-RT RIC. This is the glue that would allow a DPU-side AI to:

- Receive policy intent from the O-RAN non-RT RIC (via rApp → E3)
- Execute autonomously at line rate
- Report telemetry summaries back up the hierarchy

This hierarchical policy-down, telemetry-up architecture is exactly what the DPU+slice system needs.

#### Critical Limitation

The paper notes that transferring I/Q samples or user data out of the RAN through a rate-limited E2 interface would take seconds — incompatible with real-time interactions. The E3 interface addresses this for dApps, but the same concern applies to any DPU-side system that needs to pull rich telemetry up to a management plane.

#### Relevance to DPU/SmartNIC Research

This paper gives you the **concrete latency budget your DPU system must beat**: 450 µs software dApp → your DPU target should be demonstrably <100 µs, ideally <50 µs. Use the E3 interface design as the northbound API specification for your DPU system's policy ingestion layer.

---

## Cluster 2 — The Data Plane Intelligence Foundations {#cluster-2}

### Paper 3: Taurus: A Data Plane Architecture for Per-Packet ML

| Field | Detail |
|---|---|
| **Authors** | T. Swamy, A. Rucker, M. Shahbaz, I. Gaur, K. Olukotun |
| **Venue** | ASPLOS '22 — 27th ACM International Conference on Architectural Support for Programming Languages and Operating Systems, Lausanne, Feb–Mar 2022, pp. 1099–1114 |
| **DOI** | 10.1145/3503222.3507726 |
| **Citations** | ~149 (as of May 2026) |
| **Institution** | Stanford University; Purdue University |
| **ArXiv** | 2002.08987 |
| **Code** | gitlab.com/dataplane-ai/taurus |

#### What It Actually Does

Taurus adds custom hardware based on a flexible **MapReduce abstraction** to programmable network devices such as switches and NICs, using pipelined SIMD parallelism to enable per-packet MapReduce operations at line-rate inference. In a Taurus-enabled data centre, the control plane gathers a global view of the network and trains ML models to optimize security and switch-level metrics, while the data plane uses these models to make **per-packet, data-driven decisions** — unlike traditional SDN-based data centres where both training and inference live in the control plane.

#### The Architecture in Detail

The Taurus architecture has three layers:

1. **Control plane (slow path):** Trains ML models offline using global telemetry; distributes frozen model weights to data-plane devices
2. **Taurus MapReduce block (co-processor):** Custom FPGA/ASIC hardware alongside the PISA P4 pipeline; handles matrix multiplication, nonlinear operations that P4 cannot natively express
3. **Data plane (fast path):** Receives packet features, feeds them to frozen model via MapReduce block, makes per-packet decisions at line rate

The MapReduce abstraction is particularly clever. It allows arbitrary aggregation across packets and flows using a small set of primitives (map, reduce, shuffle) that can be implemented in hardware without floating-point arithmetic — though Taurus adds floating-point capability via the co-processor.

#### Deep Insight — Why This Is the Most Important Hardware Paper for DPU Research

Taurus establishes a clean, proven separation:
- **Slow-path (control plane):** trains models offline using global telemetry
- **Fast-path (data plane):** does inference at line rate per-packet using frozen models

This is the exact split your DPU architecture would use. For a DPU like BlueField-4 (ARM cores + inline processing engine + 800 Gbps connectivity), the division is:
- ARM cores = slow-path training/retraining + model management
- Inline processing engine = fast-path inference at packet rate

Note that Taurus had to build custom co-processor hardware because P4 switches lack multiplication support. BlueField-4's ARM cores *already have* full floating-point and can run quantized neural network inference natively. This makes DPU implementation of the Taurus pattern **more tractable** than the original FPGA path Taurus took.

#### Critical Limitation

Taurus was demonstrated on **anomaly detection** — a classification task. It has not been applied to:
- *Prediction* (forecasting future bandwidth needs at a temporal horizon)
- *Actuation* (modifying slice resource reservation tables in response to predictions)
- *Multi-tenant QoS policy enforcement* (different decision rules per slice)

The jump from "classify this packet as anomalous" to "forecast this flow's bandwidth demand in 200 ms and pre-reserve slice capacity" is the novel step. Taurus proves the hardware substrate can support per-packet ML; it does not prove that substrate can run the specific predict→reserve→forgo loop.

#### Relevance to DPU/SmartNIC Research

**This is the foundational data-plane ML architecture you should build on.** Frame your DPU work as "Taurus applied to the slice bandwidth prediction domain, instantiated on a commercially available DPU platform rather than custom FPGA hardware." The MapReduce abstraction can be directly mapped to the DOCA (NVIDIA) or P4 pipeline of modern DPUs.

---

### Paper 5: Intelligent Data-Planes for Network Traffic Management

| Field | Detail |
|---|---|
| **Author** | K. Zhang |
| **Venue** | PhD Thesis, University of Ottawa, 2025 |
| **URL** | ruor.uottawa.ca |

#### What It Actually Does

This thesis introduces ML directly into the data plane to enable real-time, autonomous decision-making, reducing the latency associated with traditional SDN architectures where traffic analysis runs on a remote controller. The key technical contribution is an **in-network multi-task learning framework** that supports concurrent management tasks by performing simultaneous inference for multiple tasks in the data plane — enabling resource-efficient, accurate decisions without per-task model redundancy.

#### Deep Insight — The Multi-Task Architecture

The multi-task angle is under-appreciated in the literature. A DPU managing multiple slices simultaneously does not need a separate ML model per slice — it needs a **single model with multiple output heads**, one per QoS class or slice type. Consider:

```
Input: [packet header features, flow statistics, queue depths, timestamp]
Shared encoder: [GRU or temporal convolutional layers]
Output heads:
  ├── Head 1: eMBB slice bandwidth demand (next 100 ms)
  ├── Head 2: URLLC slice SLA risk score
  └── Head 3: mIoT slice admission decision
```

This multi-head architecture runs one forward pass per scheduling interval and produces all slice decisions simultaneously — dramatically more efficient than three separate models. The thesis likely demonstrates this on a software switch (Open vSwitch or P4 simulation), not on real DPU hardware, which is the implementation gap your work fills.

#### Relevance to DPU/SmartNIC Research

This gives you the **ML architecture framing**: a single in-network model with per-slice output heads, running on the DPU data plane. Cite this and claim the DPU hardware instantiation as your novel engineering contribution.

---

### Paper 6: Closed-Loop Network Control for Industrial Edge Computing — A Telemetry-Driven and AI-Based Approach for Latency-Critical Applications in Private 5G

| Field | Detail |
|---|---|
| **Authors** | V.S. Simão, L.V. Monteiro, R.D. Gomes et al. |
| **Venue** | IEEE Networking Letters, 2025 |
| **Citations** | 1 (as of May 2026) |

#### What It Actually Does

Presents a closed-loop network control mechanism for industrial private 5G networks, integrating real-time telemetry from a programmable data plane (P4) with an AI-based congestion prediction model. The control loop: P4 data plane collects per-flow metrics → AI model predicts imminent congestion → controller dynamically adjusts traffic priorities to ensure low latency for delay-sensitive applications.

#### Deep Insight — The Closest Existing Closed Loop

This is the closest existing paper to the full DPU+AI+slice closed loop you are describing, but with two important constraints:

1. **Narrower context:** Private 5G industrial edge, not multi-tenant public slicing with SLA contracts
2. **Central controller bottleneck:** The AI runs on an SDN controller that still receives telemetry over the network and sends commands back. The round-trip to the controller is where latency is lost.

Your architecture replaces the SDN controller with **DPU-resident actuation**, eliminating the controller round-trip entirely. The telemetry collection happens inside the DPU's packet processing pipeline; the AI decision happens on the DPU's ARM cores; the actuation happens by rewriting the DPU's QoS policy tables — all without leaving the network card.

#### The Control Loop Comparison

| Step | This Paper | DPU Architecture |
|---|---|---|
| Telemetry collection | P4 switch → SDN controller | DPU inline engine → ARM cores |
| AI inference | SDN controller CPU | DPU ARM core (on-card) |
| Actuation | SDN controller → switch rules | DPU ARM → DPU QoS tables |
| Round-trip latency | ~1–10 ms (network + CPU) | <100 µs (on-card) |

#### Relevance to DPU/SmartNIC Research

This paper is the most direct proof-of-concept that the data-plane telemetry → AI prediction → QoS actuation loop works in a real (if simplified) system. Use it as motivating evidence that the closed loop is viable, then position your DPU work as the next generation that removes the central controller bottleneck.

---

### Paper 11: Deep Data Plane Programming and AI for Zero-Trust Self-Driving Networking in Beyond 5G

| Field | Detail |
|---|---|
| **Authors** | O. Hireche, C. Benzaïd, T. Taleb |
| **Venue** | Computer Networks (Elsevier), 2022 |
| **Citations** | ~66 |

#### What It Actually Does

Explores a framework for fully distributed trustworthy self-driving networks across multiple domains, exploiting programmable data planes (P4), AI, blockchain, and federated learning. The paper leverages programmable data-plane capabilities for real-time in-network telemetry collection and uses P4 + AI to enable the network to automatically translate policy intent into executable actions on network components.

#### Deep Insight — Policy-to-Action Automation

The "policy-to-action automation" framing is critical for your system design. The paper demonstrates:

```
Operator intent (high-level) → Policy compiler → P4 data-plane rules (low-level)
```

In the DPU context this becomes:

```
Slice SLA (JSON/YANG) → QoS policy engine → DPU forwarding rules + AI constraints
```

The blockchain component (for zero-trust verification of policy updates) is less directly relevant but signals that **security in distributed data-plane AI is an open, documented concern** — particularly important for multi-tenant slicing where one tenant's DPU rules must not be visible to another.

#### Relevance to DPU/SmartNIC Research

Provides the policy layer architecture. The northbound interface of your DPU system should accept SLA/intent specifications and compile them to internal QoS constraint tables that the AI uses during its reserve/forgo decisions.

---

## Cluster 3 — Proactive Slice Provisioning (The Prediction Side) {#cluster-3}

### Paper 46 (cited in prior research): DeepCog: Optimizing Resource Provisioning in Network Slicing with AI-Based Capacity Forecasting

| Field | Detail |
|---|---|
| **Authors** | D. Bega, M. Gramaglia, M. Fiore, A. Banchs, X. Costa-Pérez |
| **Venue** | IEEE Journal on Selected Areas in Communications (JSAC), 2020 |
| **DOI** | 10.1109/JSAC.2019.2959187 |
| **Citations** | ~180 (as of May 2026) |
| **Institutions** | IMDEA Networks Institute; Universidad Carlos III de Madrid; CNR Torino; NEC Laboratories Europe |

#### What It Actually Does

DeepCog presents a deep neural network architecture inspired by image processing, trained via a dedicated **cost-aware loss function (α-OMC)**. Unlike traditional traffic volume predictors, DeepCog returns a *capacity forecast* — the minimum bandwidth that must be reserved to satisfy SLA at a given confidence level — which can be directly used for short- and long-term reallocation decisions. Evaluations with real-world metropolitan-scale mobile network data demonstrate **>50% reduction in resource management costs** versus state-of-the-art traffic predictors.

#### The Crucial Design Insight — Capacity vs. Traffic

DeepCog does not predict traffic volume. It predicts *capacity*. This is a fundamentally different framing:

- **Traffic prediction:** "Slice A will consume X Mbps in the next interval"
- **Capacity prediction:** "You must reserve Y Mbps for Slice A to guarantee SLA at 99.9% confidence"

This distinction matters enormously for the "forgo" primitive your architect described. You need to know not just predicted demand but the **confidence interval** around it to safely release bandwidth without risking SLA violations. DeepCog provides both: the point estimate and the risk-adjusted capacity recommendation.

#### The α-OMC Loss Function

The α-OMC (α-Order Magnitude Cost) loss function is the technical heart of the paper. It **asymmetrically penalises under-provisioning more than over-provisioning**, with a tunable α parameter:

- α → 0: conservative (reserve generously, never violate SLA, waste bandwidth)
- α → 1: aggressive (reserve tightly, occasionally violate SLA, maximise utilisation)

For the "forgo" primitive: when confidence is high and α is set aggressively, the system releases bandwidth. When confidence is low, it holds the reservation. This is the risk-calibrated decision logic your DPU needs.

#### Critical Limitation

DeepCog runs in the **management plane** — centralized, periodic, at a minutes timescale. It was designed for operator planning, not for microsecond real-time actuation. Moving DeepCog's forecasting logic to a DPU — running it on the ARM cores, triggered per-flow or per-scheduling-interval — requires:

1. Model quantisation to INT8 or INT4 for fast ARM inference
2. Input feature reduction (fewer metrics, shorter history window)
3. Output post-processing latency budget <1 ms

#### Relevance to DPU/SmartNIC Research

This is **the forecasting model you should adapt**. The cost-aware capacity framing and α-OMC loss function are directly applicable to your reserve/forgo decision logic. Implement a quantised version on the DPU ARM cores and benchmark against the full DeepCog model running on a server CPU — demonstrating that DPU inference achieves comparable accuracy at 10–100× lower latency is a publishable result on its own.

---

### Paper 2: A Modular DTaaS Architecture for Predictive Slice Management in 6G Systems

| Field | Detail |
|---|---|
| **Authors** | T. Bilen, M. Ozdem |
| **Venue** | 2025 4th International Conference on Computing and Machine Intelligence (IEEE), 2025 |
| **Citations** | 1 (as of May 2026) |

#### What It Actually Does

Proposes a **Digital Twin as a Service (DTaaS)** framework that instantiates per-slice Slice Digital Twins (SDTs) with embedded predictive intelligence. SDTs leverage real-time multi-domain telemetry and deep sequential models to forecast traffic evolution and predict SLA risks, enabling proactive decision-making. The framework identifies that most existing slicing solutions operate reactively — allocating or scaling resources only after congestion or degradation has occurred — and proposes edge-embedded closed-loop SLA assurance.

#### Deep Insight — Per-Slice Digital Twins on DPU

The per-slice digital twin concept maps naturally to DPU architecture:

```
DPU ARM cores
├── SDT for eMBB slice (GRU model + telemetry buffer + SLA constraints)
├── SDT for URLLC slice (lighter model + tighter SLA thresholds)
└── SDT for mIoT slice (batch-mode model + looser constraints)
```

Each SDT is a lightweight model that tracks its own telemetry history and generates its own forecasts independently. This is **decentralised prediction**, which maps well to DPU architecture: each slice's model runs as a separate inference process on the DPU's ARM cores, consuming <5% CPU per slice, allowing dozens of slices to be managed simultaneously.

The paper proposes "edge-embedded decision-making" but leaves the hardware unspecified — "the edge" is an abstract compute node. This is exactly where DPU fills the void as the physical realisation of that edge node.

#### Relevance to DPU/SmartNIC Research

Use DTaaS as the **framework justification** and position the DPU as the physical realisation of the edge-embedded SDT node. The per-slice model isolation maps cleanly to DPU multi-tenant isolation requirements.

---

### Paper 25: PreNS: A Hybrid Predictive and Real-Time Resource Allocation Framework for 5G and Beyond RAN Network Slicing

| Field | Detail |
|---|---|
| **Authors** | B. Wu, P.C. Amogh, N.V. Abhishek et al. |
| **Venue** | IEEE Transactions on Network and Service Management, 2026 |

#### What It Actually Does

Combines an **attention-based Bi-LSTM model** for traffic demand forecasting with a **Double Deep Q-Network (DDQN)** for real-time adjustments in resource management. Evaluated on a realistic 5G testbed, demonstrating improvements in resource utilisation, reduced latency, and enhanced QoS satisfaction.

#### Deep Insight — The Two-Stage Hybrid is the Right Architecture

The two-stage hybrid is the right conceptual architecture for DPU deployment:

| Stage | Model | Timescale | DPU Location |
|---|---|---|---|
| Prediction | Attention Bi-LSTM | Every 100–500 ms | ARM cores (periodic inference) |
| Actuation | DDQN Q-table lookup | Every scheduling slot (~1 ms) | Inline engine or ARM cores |

The Bi-LSTM doesn't need to be real-time — it can run on the DPU ARM cores at moderate frequency, updating a shared bandwidth forecast table. The DDQN's Q-table lookup is an O(1) operation given the current state — fast enough to run inline. This two-stage split avoids the latency problem of running a full neural network per scheduling decision.

#### Critical Limitation

The testbed is almost certainly a **software simulation** of a 5G testbed running on commodity servers, not real DPU hardware. The "real-time" in the title refers to real-time *policy decisions* within a simulated environment, not real-time at network card speeds. The latency numbers reported would be very different on actual DPU hardware.

#### Relevance to DPU/SmartNIC Research

**PreNS is your most direct existing competitor.** Study its architecture carefully and design your DPU system to outperform it on: (a) control loop latency, (b) host CPU offload percentage, (c) multi-slice scalability. Claim that PreNS runs in software on a server; your system runs in hardware on a DPU — same logic, 10–100× faster, zero host CPU involvement.

---

### Paper 9: Service-Aware Real-Time Slicing for Virtualized Beyond 5G Networks

| Field | Detail |
|---|---|
| **Authors** | T. Tsourdinis, I. Chatzistefanidis, N. Makris, T. Korakis et al. |
| **Venue** | Computer Networks (Elsevier), 2024 |
| **Citations** | ~28 |

#### What It Actually Does

Designs and implements a service-aware network that classifies cellular network traffic to predict future network load and proactively reallocates slices in the Radio Access Network via programmable APIs. Uses a distributed ML model to classify real-time traffic from different users and infer future connectivity needs (≤10 ms decisions). Operates independently of dedicated 5G slicing components like the NSSF and utilises FlexRAN for slice creation.

#### Deep Insight

The "independent of NSSF" design choice is significant: this system bypasses the 3GPP-standard slice selection machinery and operates directly on the RAN. This is analogous to what a DPU would do — bypassing the management plane entirely and operating from within the forwarding hardware. The use of FlexRAN as the slice control API is the practical implementation hook; on a DPU the equivalent is the NVIDIA DOCA SDK or Marvell OCTEON SDK.

#### Relevance to DPU/SmartNIC Research

Demonstrates that bypassing the standard 5G NSSF and operating directly at the RAN level is viable. Your DPU architecture does the same — bypassing the O-RAN xApp/rApp stack for time-critical decisions.

---

### Paper 43: Dynamic Allocation of 5G Transport Network Slice Bandwidth Based on LSTM Traffic Prediction

| Field | Detail |
|---|---|
| **Authors** | S. Xiao, W. Chen |
| **Venue** | IEEE 9th International Conference on Software Engineering and Service Science, 2018 |
| **Citations** | ~50 |

#### What It Actually Does

Introduces LSTM-TPDTNS (LSTM-based Traffic Prediction Dynamic Transport Network Slicing framework). Uses LSTM for traffic prediction and models the bandwidth configuration phase as a **fractional knapsack problem** solved with greedy algorithms. Focuses on dynamic allocation of bandwidth resources to services of different priorities.

#### Deep Insight — The 2018 Baseline

This is the 2018 baseline that all 2024–26 papers build on. Every newer system (DeepCog, PreNS, DATURA) is benchmarked against or motivated by this approach. The fractional knapsack + greedy formulation is computationally tractable but ignores future uncertainty — it optimises for the *current* predicted demand without accounting for prediction error. DeepCog and the α-OMC loss function are the direct answer to this limitation.

For DPU implementation, the knapsack solver is actually fast enough to run inline (it is a simple O(n log n) greedy sort), but the LSTM inference is not — LSTMs have sequential hidden state dependencies that prevent parallelism. Replacing LSTM with a GRU or temporal convolutional network (TCN) is the standard DPU adaptation.

#### Relevance to DPU/SmartNIC Research

This is your **baseline comparison point** — the simplest credible prior work. Your DPU system should significantly outperform LSTM-TPDTNS on latency, while matching or exceeding it on accuracy.

---

## Cluster 4 — Distributed/Federated Slice AI {#cluster-4}

### Paper 7: Zero-Touch AI-Driven Distributed Management for Energy-Efficient 6G Massive Network Slicing

| Field | Detail |
|---|---|
| **Authors** | H. Chergui, L. Blanco, L.A. Garrido, K. Ramantas et al. |
| **Venue** | IEEE Network Magazine, 2022 |
| **Citations** | ~104 |
| **Institutions** | i2CAT (Barcelona); CTTC; University of Patras |

#### What It Actually Does

Introduces a **Statistical Federated Learning (StFL)-based** analytic engine for zero-touch 6G massive network slicing. Performs slice-level resource prediction by learning in an offline fashion while respecting long-term SLA constraints defined via empirical cumulative distribution functions and percentile statistics. Uses a proxy-Lagrangian two-player strategy to solve the local non-convex federated learning task. Results: **20× lower SLA violation rate** vs. FedAvg scheme; **>10× energy efficiency gain** vs. SLA-constrained centralised deep learning.

#### Deep Insight — Statistical Federated Learning (StFL)

Rather than sharing raw gradients (standard FL), StFL nodes share **compressed statistics** — CDFs, percentile estimates — that encode SLA constraints. This has two critical advantages for DPU deployment:

1. **Communication efficiency:** Sharing percentile statistics requires far less bandwidth than sharing full gradient tensors — critical when DPU-to-management-plane uplinks are rate-limited
2. **Privacy preservation:** No raw user traffic data leaves the DPU — only statistical summaries, which preserves tenant data isolation

The proxy-Lagrangian formulation is the mechanism that converts SLA constraints (e.g. "99.9% of packets must see <1 ms latency") into training constraints on the federated model. On a DPU, these constraints would be loaded as part of the slice's SLA policy table.

#### The Distributed Architecture

```
DPU 1 (Server A)          DPU 2 (Server B)          DPU 3 (Server C)
  └── Local slice model      └── Local slice model      └── Local slice model
        │                           │                           │
        └─── Statistical summary ───┴─── Statistical summary ──┘
                               │
                    Central aggregator
                    (non-RT RIC or MANO)
                               │
                    Updated global model
                               │
              ┌────────────────┼────────────────┐
           DPU 1             DPU 2            DPU 3
```

#### Relevance to DPU/SmartNIC Research

StFL gives you the **inter-DPU learning protocol**. In a cluster of servers each with a DPU, the DPUs can run StFL among themselves to collaboratively learn slice demand patterns without sharing raw user traffic data. This is a privacy-preserving distributed intelligence layer at the hardware level — a completely novel application of federated learning.

---

### Paper 22: AI-Enabled Network Slicing and Resource Management for Open and Programmable Next-Generation (6G) Networks

| Field | Detail |
|---|---|
| **Author** | S.S. Mhatre |
| **Venue** | PhD Thesis, UPC Barcelona, 2025 |
| **Citations** | 1 |

#### What It Actually Does

Proposes intelligent, scalable, and explainable solutions using Deep Reinforcement Learning (DRL) and related AI techniques for resource management in 6G networks. Develops a DRL-based, QoS-aware slice resource allocation framework with user association parameterisation for beyond-5G O-RAN environments. Presents a multi-time scale resource management framework under an AI-as-a-Service (AIaaS) paradigm.

#### Deep Insight — AIaaS Framing

The "AI as a Service" framing is an interesting inversion: rather than each network function having its own AI, a dedicated AI service provides inference on demand to multiple network functions. On a DPU, this could be implemented as:

```
DPU DOCA Application
├── AI inference service (ARM cores, always-on)
│     ├── Slice 1 inference endpoint
│     ├── Slice 2 inference endpoint  
│     └── Slice N inference endpoint
└── QoS policy enforcement (inline engine)
      ├── Reads AI service recommendations
      └── Enforces bandwidth reservations
```

This "AI service on DPU" model separates the intelligence from the enforcement — making the system modular and easier to update.

#### Relevance to DPU/SmartNIC Research

The multi-time scale framework (fast RL for slot-level decisions, slow DRL for session-level decisions) maps cleanly to the DPU's two processing tiers: inline engine (fast) vs. ARM cores (slower but more capable).

---

## Cluster 5 — VNF/Slice Scaling: 2025–26 State of the Art {#cluster-5}

### Paper 21: DATURA: A Deep Learning-Based Adaptive Traffic-Aware Unified Resource Autoscaling Framework for VNFs in 5G/B5G Networks

| Field | Detail |
|---|---|
| **Authors** | K.K. Tiwari, K.M. Sivalingam |
| **Venue** | IEEE Open Journal of the Communications Society (OJCOMS), 2026 |
| **DOI** | 10.1109/OJCOMS.2026.3668155 |
| **Institution** | IIT Madras |

#### What It Actually Does

Introduces DATURA — a Deep Learning-based Adaptive Traffic-aware Unified Resource Autoscaling framework for VNFs in 5G and beyond networks. A predictive **GRU-MLP based scaling architecture** is proposed and integrated with a DRL-based traffic generator. The GRU-MLP scaling controller forecasts per-VNF and per-slice latencies from streaming telemetry. Temporal dependencies are captured through the GRU layer, while two multitask MLP heads estimate local (per-VNF) and global (slice-level) latency trends. Once deployed, DATURA operates fully online executing inference-driven scaling decisions in real time. The underlying models are periodically retrained with updated telemetry without service disruption. Detailed experiments show: **73% latency reduction, 55% throughput gain, 47% higher provisioning efficiency** vs. LSTM and GRU baselines.

#### Deep Insight — The GRU-MLP Architecture Fits a DPU

The GRU-MLP multitask architecture is precisely what fits on a DPU ARM core:

| Component | Why it fits DPU |
|---|---|
| GRU (vs. LSTM) | Smaller, fewer parameters, comparable temporal modelling — lower inference latency |
| MLP heads | Fixed-size fully-connected layers — run in microseconds on ARM |
| Multitask (2 heads) | Local VNF + global slice — mirrors DPU's per-flow and per-slice visibility |
| Online inference | No batch accumulation required — suitable for streaming packet telemetry |

DATURA is currently running on a **SimPy-based 5G Core simulation** — not on DPU hardware. This is the exact implementation gap your research fills. The SimPy environment models realistic traffic but has none of the hardware constraints (memory bandwidth, inference latency, model size limits) of a real DPU.

#### The Periodic Retrain Challenge — An Open Engineering Problem

> *"The underlying models are periodically retrained with updated telemetry, enabling the framework to adapt to long-term traffic evolution without service disruption."*

On a DPU, this statement hides a non-trivial engineering challenge. The active model is simultaneously serving inference at 400 Gbps line rate while the ARM cores are retraining. The solution requires:

1. **Shadow model:** New model trains in parallel on ARM cores
2. **Validation gate:** New model must pass accuracy validation before promotion
3. **Atomic swap:** Old model replaced with new model in a single memory-safe operation
4. **Rollback mechanism:** If new model causes SLA violations, revert to previous version

This model lifecycle problem is completely unaddressed in the literature for DPU hardware. It is a publishable contribution in itself.

#### Relevance to DPU/SmartNIC Research

DATURA is the **most mature ML architecture** in this set for your DPU implementation. The GRU-MLP is your candidate on-DPU model. The 73% latency reduction is your **software baseline** — your DPU implementation should exceed this. The periodic retrain challenge is an open problem you can solve as part of your contribution.

---

### Paper 10: E2E Network Slice Assurance for B5G/6G — Data Collection, MLOps, and Closed-Loop Control

| Field | Detail |
|---|---|
| **Authors** | S. Marinova, Y. Tian et al. |
| **Venue** | IEEE Open Journal of the Communications Society, 2025 |
| **Citations** | ~7 |

#### What It Actually Does

Proposes an end-to-end slice assurance framework including data collection, MLOps pipeline, and closed-loop control for per-slice SLA assurance. Implements and evaluates a traffic forecasting use case with predictions of per-slice throughput under real operator requirements. Details design and implementation of closed-loop control driven by data analysis within each domain.

#### Deep Insight — MLOps for Network Slicing

Most ML-for-slicing papers ignore **model lifecycle**: how do you retrain when traffic distributions shift? How do you detect model drift? How do you roll back a bad model? This paper is one of the few that attempts to address these questions systematically.

The MLOps pipeline it proposes:
```
Data collection → Feature engineering → Model training → Validation → Deployment → Monitoring → Drift detection → Retrain trigger
```

For a DPU deployment, this pipeline needs to be implemented in two locations:
- **On-DPU:** Inference serving + telemetry collection + drift detection signal
- **Off-DPU (management plane):** Full model retraining + validation + model distribution

The drift detection signal is particularly important. A DPU can detect when its current model's predictions are consistently wrong (by comparing predictions against actual measurements) and trigger an out-of-band retrain without human intervention. This is the autonomous self-healing aspect of the system.

#### Relevance to DPU/SmartNIC Research

This paper gives you the **operational framework** your DPU system needs alongside the ML. Do not just design the inference engine — design the model update pipeline. An ML system without MLOps is a research demo; an ML system with MLOps is a production-ready contribution.

---

### Paper 26: Proactive Resource Optimization for Heterogeneous 5G Network Slicing

| Field | Detail |
|---|---|
| **Authors** | J. Ajayi, T. Braun |
| **Venue** | 2026 21st Wireless On-Demand Network Systems and Services (IEEE), 2026 |
| **Citations** | 1 |

#### What It Actually Does

Presents a framework integrating an **attention-based traffic forecasting model** with a **Deep Reinforcement Learning (PPO) resource allocator** to anticipate future slice demands and proactively adjust network configurations. Evaluates on real LTE traffic traces for eMBB and URLLC-like services, demonstrating improved SLA compliance.

#### Deep Insight — PPO on DPU

PPO (Proximal Policy Optimisation) is a policy gradient RL algorithm that is moderately compute-intensive but well-suited to DPU ARM cores for *policy execution* (not training). Training PPO requires GPU-scale compute; executing a trained PPO policy is a simple neural network forward pass that runs in microseconds on ARM. The DPU runs the trained policy; a server-side system handles periodic retraining with updated replay buffers.

The attention-based forecasting model is the heavier component — attention requires O(n²) computation over the sequence length. On a DPU, this must be bounded: limit sequence length to 16–32 timesteps and use local attention rather than global attention.

#### Relevance to DPU/SmartNIC Research

Very recent (2026), low citation count, but directly relevant architecture. Positions the attention+PPO combination as the 2026 state of the art for proactive slicing — your DPU system should benchmark against this.

---

## Cluster 6 — The AI-RAN Vision Papers {#cluster-6}

### Paper 4: AI-RAN: The Pathway to Future Wireless Networks

| Field | Detail |
|---|---|
| **Authors** | C. Feng, H.H. Yang, K. Guo, W. Xia, C. Liu, T.Q.S. Quek |
| **Venue** | Journal of Information and Intelligence (Elsevier), Vol. 4, No. 1, pp. 5–22, Jan 2026 |
| **Citations** | 2 (as of May 2026) |
| **Institutions** | University of Exeter; Zhejiang University/UIUC; ECNU; NUPT; BUPT; SUTD Singapore |

#### What It Actually Does

Provides a comprehensive overview of the AI-RAN paradigm, which integrates high-performance computing resources into RAN infrastructures to enable execution of both AI and RAN workloads on the same hardware. Categorises AI-RAN into three aspects:

1. **AI and RAN:** Hardware architecture, software stack, orchestration of compute and communication
2. **AI for RAN:** Optimisation of RAN using AI (beam management, scheduling, resource allocation)
3. **AI on RAN:** AI computation physically embedded in the radio infrastructure — inference at the radio edge

#### Deep Insight — Where DPU Research Belongs

**"AI on RAN"** is the category your DPU work belongs in. The paper's unified sensing-learning-decision-actuation pipeline:

```
Sensing (standardised telemetry from radio units)
    ↓
Learning (model training on collected data)
    ↓
Decision (inference → resource management choice)
    ↓
Actuation (enforcement of bandwidth reservations)
```

Is exactly the DPU loop. But the hardware instantiation in this paper assumes a **GPU-equipped base station server** — AI on RAN means a GPU in the rack, not a DPU in the data path. The jump from GPU-in-server to DPU-in-data-path is architecturally distinct and unexplored in the AI-RAN literature.

#### The Intent-Driven Slicing Use Case

The paper examines intent-driven slicing: the operator expresses intent ("guarantee 10 ms RTT for slice A") and the system autonomously translates this to resource actions. On a DPU, intent becomes a **QoS policy table** that the AI uses as a constraint during bandwidth reservation decisions:

```
Intent: {slice: "URLLC", max_latency: 1ms, reliability: 99.999%}
    ↓ policy compiler
DPU QoS table: {slice_id: 2, min_bw: 100Mbps, max_queue_depth: 4, priority: STRICT}
    ↓ AI forecast
DPU action: {reserve: 120Mbps, confidence: 0.97, forgo_trigger: demand < 80Mbps for 50ms}
```

#### Relevance to DPU/SmartNIC Research

AI-RAN is the **2026 vision document** for this entire field. Position your work explicitly within the "AI on RAN" sub-category and as the **DPU hardware realisation of the AI-RAN sensing-learning-decision-actuation pipeline**. A 2026 paper that can claim "we implement the AI-RAN pipeline on a DPU for the first time" has strong positioning.

---

### Paper 13: Edge General Intelligence through World Models, Large Language Models, and Agentic AI

| Field | Detail |
|---|---|
| **Authors** | C. Zhao, G. Liu, R. Zhang, Y. Liu, J. Wang et al. |
| **Venue** | IEEE Transactions on Communications, 2026 |
| **Citations** | 8 |

#### What It Actually Does

Explores architectural foundations of world models for edge intelligence, including latent representation learning, dynamics modelling, and imagination-based planning. Illustrates proactive applications of agentic systems across vehicular networks, UAV networks, and IoT systems. Addresses optimisation under edge constraints including latency, energy, and privacy.

#### Deep Insight — World Models on DPU

A world model learns a compact latent representation of the network state and uses it to *simulate* future states without observing them — "imagining" future traffic patterns. On a DPU, this enables:

- **Imagination-based planning:** The DPU simulates 5–10 future scheduling intervals in milliseconds, evaluating different reserve/forgo decisions against the simulated future, and selecting the best one
- **Low-data adaptation:** World models generalise better to new slice types with less telemetry data than pure model-free RL approaches

This is speculative for current DPU hardware — world model inference is compute-intensive. But as DPU ARM cores scale (BlueField-5 generation), world model planning becomes feasible at sub-10 ms timescales.

#### Relevance to DPU/SmartNIC Research

This is a **future direction** paper for your research, not an immediate implementation target. Position world model-based planning as a long-term evolution of your system.

---

### Paper 14: Self-Running Networks: A Comprehensive Survey of Foundations, Applications, and Challenges

| Field | Detail |
|---|---|
| **Authors** | S. Shajarian, S. Khorsandroo, M. Abdelsalam |
| **Venue** | TechRxiv, 2025 |
| **Citations** | 2 |

#### What It Actually Does

Explores a paradigm for fully autonomous network infrastructure capable of self-configuration, self-optimisation, self-healing, and self-protection without human intervention. Details a seven-layer reference model for end-to-end autonomy. Analyses key self-* functionalities, core mechanisms, operational challenges, and representative application domains.

#### Deep Insight — The Self-* Framework for DPU Systems

The seven-layer autonomy model provides a useful taxonomy for characterising how autonomous your DPU system is:

| Layer | Function | DPU Implementation |
|---|---|---|
| Self-awareness | Monitor own state | Per-slice telemetry aggregation |
| Self-configuration | Initial setup | Slice policy table initialisation |
| Self-optimisation | Performance tuning | AI-driven bandwidth adjustment |
| Self-healing | Fault recovery | SLA violation detection + remediation |
| Self-protection | Security | Anomaly detection + rate limiting |
| Self-evolution | Model updates | Periodic retraining + hot-swap |
| Self-explanation | Transparency | XAI for operator audit trails |

Your DPU system addresses layers 1–4 and 6. Layers 5 and 7 are extensions.

#### Relevance to DPU/SmartNIC Research

Use this framework to characterise the autonomy level of your DPU system in the paper's related work section. A DPU-resident AI that implements self-optimisation + self-healing for bandwidth slicing is a concrete instantiation of the self-running network concept.

---

## Cluster 7 — Foundational and Survey Papers {#cluster-7}

### Paper 15: Machine Learning in Network Slicing — A Survey

| Field | Detail |
|---|---|
| **Authors** | H.P. Phyu, D. Naboulsi, R. Stanica |
| **Venue** | IEEE Access, 2023 |
| **Citations** | ~75 |

#### Deep Insight

The most comprehensive ML-for-slicing survey up to 2023. Categorises the literature by: ML algorithm type (supervised, RL, federated), addressed challenge (admission control, resource allocation, SLA assurance, traffic classification), and resource type (RAN, transport, core). Identifies the persistent gap that most work uses simulated traffic rather than real operator data. The taxonomy in this survey is the standard way to position new work in the field.

---

### Paper 27: Energy-Efficient Deep Reinforcement Learning Assisted Resource Allocation for 5G-RAN Slicing

| Field | Detail |
|---|---|
| **Authors** | Y. Azimi, S. Yousefi, H. Kalbkhani et al. |
| **Venue** | IEEE Transactions on Vehicular Technology, 2021 |
| **Citations** | ~114 |

#### Deep Insight

Introduces the two-timescale RL framework that has become the standard approach: slow DL model for large-timescale decisions (session-level provisioning) + fast DRL model for small-timescale decisions (slot-level scheduling). This two-timescale separation maps cleanly to the DPU architecture:

- **Large timescale (100 ms – seconds):** ARM cores running GRU/LSTM forecast model
- **Small timescale (<1 ms):** Inline engine applying pre-computed QoS policy tables

The energy efficiency dimension (this paper's focus) is also directly relevant to DPU design: a DPU consumes ~25–75W; if AI-driven slice management reduces over-provisioning, the energy savings at the infrastructure level far exceed the DPU's own power draw.

---

### Paper 28: Deep Reinforcement Learning for Online Resource Allocation in Network Slicing

| Field | Detail |
|---|---|
| **Authors** | Y. Cai, P. Cheng, Z. Chen, M. Ding et al. |
| **Venue** | IEEE Transactions on Mobile Computing, 2023 |
| **Citations** | ~67 |

#### Deep Insight

Introduces **PW-DRL (Prediction-aided Weighted DRL)** — the first paper to formally integrate prediction uncertainty into the DRL reward function. Rather than treating the traffic forecast as a ground truth input, PW-DRL weights the DRL reward by the forecasting model's confidence, so uncertain predictions lead to conservative actions. This is the rigorous formalisation of what your α-OMC/DeepCog-inspired forgo logic needs: an uncertainty-weighted actuation signal.

---

### Paper 29: Applications of Machine Learning in Resource Management for RAN-Slicing in 5G and Beyond Networks — A Survey

| Field | Detail |
|---|---|
| **Authors** | Y. Azimi, S. Yousefi, H. Kalbkhani, T. Kunz |
| **Venue** | IEEE Access, 2022 |
| **Citations** | ~70 |

#### Deep Insight

The definitive survey on RAN-slicing resource management with ML. Classifies papers by ML algorithm, challenge addressed, and resource type. Key finding: virtually all work assumes the ML runs on a server-side controller; no paper in the survey runs ML in the data plane or on a DPU. This survey's bibliography is your starting literature map, and its gap identification section directly motivates data-plane ML for slicing.

---

### Paper 30: Traffic Scheduling, Network Slicing and Virtualization Based on Deep Reinforcement Learning

| Field | Detail |
|---|---|
| **Authors** | P.M. Kumar, S. Basheer, B.S. Rawal, F. Afghah et al. |
| **Venue** | Computers and Electrical Engineering (Elsevier), 2022 |
| **Citations** | ~12 |

#### Deep Insight

Develops three main network slicing blocks: (1) traffic analysis and network slice forecasting, (2) network slice admission management decisions, and (3) adaptive load prediction corrections. The three-block architecture is notable: the third block (adaptive correction) is an online error correction mechanism that adjusts the first block's forecasts based on observed prediction errors. This is similar to the error-correction loop in the ECP framework and is directly implementable on DPU ARM cores as a lightweight Kalman filter or exponential moving average correction.

---

### Paper 31: QoS Guaranteed Network Slicing Orchestration for Internet of Vehicles

| Field | Detail |
|---|---|
| **Authors** | Y. Cui, X. Huang, P. He, D. Wu et al. |
| **Venue** | IEEE Internet of Things Journal, 2022 |
| **Citations** | ~63 |

#### Deep Insight

Proposes **LSTM-DDPG** (LSTM for long-term environment modelling + Deep Deterministic Policy Gradient for online resource tuning) for vehicular network slicing. The DDPG component is notable because it handles *continuous* action spaces — bandwidth allocation is a continuous variable, and DDPG can output exact bandwidth amounts rather than discretised choices. This is architecturally superior to DQN-based approaches for bandwidth management and is implementable on DPU ARM cores for the actuation component.

---

## Cross-Paper Synthesis {#cross-paper-synthesis}

Reading across all papers in this collection, five patterns emerge that directly shape your DPU/SmartNIC research agenda:

### Pattern 1 — The Latency Chain Breaks at the DPU Boundary

Every paper in this collection operates at millisecond timescales or slower:

| System | Control Latency | Hardware |
|---|---|---|
| Non-RT RIC (rApp) | >1 second | Server CPU |
| Near-RT RIC (xApp) | 10–1000 ms | Server CPU |
| dApp (software, 2025) | ~450 µs | Server CPU at DU |
| **DPU-resident AI** | **<100 µs (target)** | **DPU ARM + inline engine** |

None of the papers cross into the DPU data plane at microsecond timescales. This is a **clean, documented, unclaimed gap** with a specific latency target (<100 µs) that can be rigorously validated.

### Pattern 2 — The ML Architecture is Converging

Across the 2024–2026 papers, a consensus architecture is emerging:

```
Temporal feature extractor (GRU or attention)
    ↓
Multi-head decision layer (one per QoS class or slice type)
    ↓
Uncertainty quantification layer (prediction confidence)
    ↓
Risk-calibrated actuation (reserve/forgo threshold based on confidence)
```

The DPU implementation challenge is not *which* ML architecture to use — it is how to **quantise and compress** these models to run on ARM/FPGA at microsecond decision intervals while preserving sufficient accuracy.

### Pattern 3 — Federated/Distributed Learning is the Consensus for Multi-Tenant Systems

Chergui (StFL), the DTaaS paper, and the AI-RAN paper all converge on distributed learning as the only scalable approach when many slices with privacy constraints are involved. Your DPU architecture should natively support:

1. Local model training on per-DPU data (no raw data leaves the card)
2. Statistical summary aggregation across DPUs
3. Federated model updates distributed back to each DPU

### Pattern 4 — The "Forgo" Primitive is Entirely Absent

Every paper in this collection handles over-provisioning *after the fact* — reactive teardown, penalty terms in the loss function, post-hoc scaling down. **No paper proposes a forward-looking mechanism to voluntarily release reserved bandwidth before a slice goes idle.** This is your genuine novel contribution: the "forgo" primitive — an AI decision that proactively releases bandwidth when prediction confidence is high that future demand will be lower than the current reservation.

The forgo decision rule would be:

```
if predicted_demand(t+T) < current_reservation × forgo_threshold
   AND prediction_confidence > confidence_threshold:
    release = current_reservation - predicted_demand(t+T)
    update_slice_table(slice_id, new_bandwidth = predicted_demand(t+T))
```

Where `forgo_threshold` and `confidence_threshold` are policy parameters tied to the slice's SLA.

### Pattern 5 — MLOps for On-Device Models is an Open Problem

How do you retrain, validate, and deploy updated models on a DPU that is simultaneously forwarding traffic at 400 Gbps? The Marinova/E2E paper begins to address this in software; no paper addresses it for DPU hardware. The hot-swap model update problem is a critical operational challenge that must be solved for any production DPU-AI system.

---

## Recommended Reading Order {#recommended-reading-order}

If you are onboarding a research team, the optimal reading sequence is:

| # | Paper | Why at This Point |
|---|---|---|
| 1 | **Taurus (Swamy et al., ASPLOS 2022)** | Establishes the hardware foundation: data-plane ML at line rate |
| 2 | **dApps 2022 (D'Oro et al., IEEE Comms Magazine)** | Establishes the architectural motivation: the latency gap in O-RAN |
| 3 | **dApps 2025 (Lacava et al., Computer Networks)** | Gives concrete latency numbers: 450 µs is the software baseline to beat |
| 4 | **DeepCog (Bega et al., IEEE JSAC 2020)** | The forecasting model design: cost-aware capacity prediction + α-OMC loss |
| 5 | **DATURA (Tiwari & Sivalingam, IEEE OJCOMS 2026)** | 2026 ML architecture: GRU-MLP + PPO, your candidate on-DPU model |
| 6 | **PreNS (Wu et al., IEEE TNSM 2026)** | Your most direct competitor: Bi-LSTM + DDQN hybrid |
| 7 | **Zero-Touch FL (Chergui et al., IEEE Network 2022)** | Multi-DPU learning protocol: statistical federated learning |
| 8 | **AI-RAN (Feng et al., Elsevier 2026)** | Vision positioning: where your work fits in the field's future |
| 9 | **DTaaS (Bilen & Ozdem, IEEE 2025)** | Per-slice digital twin framing for your DPU-side architecture |

This nine-paper sequence gives a team everything needed to frame a credible research proposal. All remaining papers in this collection serve as related work references and baseline comparisons.

---

## Summary Gap Table

| Gap | Status in Literature | Your DPU Contribution |
|---|---|---|
| Sub-ms AI control for slicing | Open (dApp shows 450 µs in software) | DPU-resident AI at <100 µs |
| Predict + forgo bandwidth | Entirely absent | Novel primitive |
| DPU ↔ slice co-design | No published work | Core novel contribution |
| Per-flow AI policy at line rate | In-NIC ML limited to classification | Extend to prediction + actuation |
| On-DPU model hot-swap | Unaddressed | Engineering contribution |
| Multi-DPU federated learning | Protocol exists (StFL), no DPU impl. | Hardware instantiation |
| Cost-aware capacity forecasting on DPU | DeepCog (server-side only) | Quantised DPU port of DeepCog |

---

*Analysis prepared May 2026. All citation counts approximate and sourced from web search as of the research date.*