# O-RAN Internals — Extreme Deep Dive (Foundations to 2026 State-of-the-Art)

> This document is the definitive master reference for the Near-RT RIC. It combines the foundational architecture of how the OSC (O-RAN Software Community) RIC actually works with the extreme cutting-edge 2026 AI-native enhancements. This is the exact domain your research paper targets — to replace it, you must first master how it currently works.

---

## Part 1: The Standard Near-RT RIC Architecture (The Baseline)

The O-RAN Alliance specifies the architecture, but the **O-RAN Software Community (OSC)** provides the reference implementation used by most researchers (and many vendors as a starting point). The OSC Near-RT RIC is deployed as a set of Kubernetes microservices.

### 1.1 Core Microservices

#### 1. E2 Terminator (E2T)
- **Role:** The gateway between the RIC and the gNBs.
- **Protocol:** Terminates the SCTP (Stream Control Transmission Protocol) connection from each E2 Node. SCTP is used instead of TCP because it supports multi-homing (resilience against link failure) and multi-streaming (prevents head-of-line blocking).
- **Processing:** 
  1. Receives ASN.1-encoded E2AP (E2 Application Protocol) messages from the radio network.
  2. Partially decodes the E2AP wrapper to extract the message type and Subscription ID.
  3. Wraps the payload in an RMR message and publishes it to the internal RMR bus.
- **Vulnerability:** As noted in your paper (CVE-2023-41628), the E2T in standard releases lacks rigorous input validation. An xApp sending malformed E2 Control messages (or out-of-order subscription responses) can cause the E2T to panic and crash, dropping all SCTP connections and blinding the entire RIC.

#### 2. Subscription Manager (SubMgr)
- **Role:** Handles the lifecycle of E2 subscriptions.
- **Why it's needed:** A gNB doesn't just broadcast telemetry endlessly; it must be told what to send, how often, and under what conditions.
- **Process:** 
  1. xApp sends a Subscription Request via RMR to SubMgr.
  2. SubMgr assigns a unique Subscription ID.
  3. SubMgr forwards the request to E2T, which sends it to the gNB.
  4. gNB accepts, E2T routes the acceptance back to SubMgr.
  5. SubMgr updates the routing table in the RMR Route Manager: "Any E2 Indication with Subscription ID X goes to xApp Y."
- **The "Merge" Feature:** If xApp A and xApp B both request the exact same KPM (Key Performance Measurement) data from the same cell, SubMgr merges them. It maintains one subscription with the gNB, but RMR duplicates the incoming messages to both xApps. This saves radio bandwidth on the E2 link.
- **Your paper's target:** During K8s xApp upgrades, the SubMgr times out the subscription if the xApp pod dies. You bypass SubMgr entirely for fast-path state transfer by using hardware DLB pointer swaps.

#### 3. RIC Message Router (RMR)
- **Role:** The nervous system. A C-based, low-latency, point-to-point messaging library (not a centralized broker like Kafka).
- **How it works:** RMR is compiled into every xApp and RIC component. It uses a routing table (distributed by the Route Manager component). When an xApp calls `rmr_send_msg()`, the library looks up the message type and Subscription ID in local memory, finds the destination IP/port of the target K8s pod, and sends it directly via TCP or NNG (Nanomsg Next Gen).
- **Latency:** RMR is reasonably fast (single-digit microseconds per hop in software), but it relies on Linux kernel networking (TCP/IP loopback or overlay networking between pods).
- **Your paper's target:** You replace RMR in the *data path* (E2 Indication → xApp) with Intel DLB (hardware dispatch), entirely bypassing the Linux kernel network stack.

#### 4. Shared Data Layer (SDL)
- **Role:** The memory of the RIC. A distributed key-value store, typically backed by Redis (often a highly available Redis Sentinel cluster).
- **Why it's needed:** xApps are designed as stateless K8s pods so they can scale horizontally or restart without losing the network state. If they need to remember UE state, ML weights, or cell historical data, they write it to SDL.
- **Namespaces:** Data is isolated into namespaces. xApp A writes to namespace "A". O-RAN WG11 (Security) mandates RBAC (Role-Based Access Control) on these namespaces so xApp B cannot read xApp A's data without permission.
- **Your paper's target:** You replace SDL for fast, localized state with an in-process Wasm host KV store, trading cluster-wide availability for microsecond access times (detailed in Part 4).

#### 5. xApp Manager (xAppMgr)
- **Role:** The lifecycle controller.
- **Process:** Receives Helm charts containing xApp images and descriptors. Deploys the K8s pods. Registers the xApp with the system, triggering the Route Manager to set up RMR routes for the new xApp.
- **Monitoring:** Monitors xApp health via K8s liveness/readiness probes.

---

## Part 2: The E2 Message Flow — Step by Step

Let's trace exactly how an xApp controls the RAN. Assume a **Traffic Steering xApp**.

### Step 1: Subscription (The Setup)
1. The xApp boots up and reads its configuration.
2. It sends an **E2 Setup Request** (via SubMgr) asking for E2SM-KPM for Cell 1.
3. The request specifies a **Report Style** (e.g., periodic, every 10ms) and specific metrics (e.g., PRB utilization, UE throughput).
4. The gNB accepts and begins generating reports.

### Step 2: Indication (The Telemetry)
1. **gNB:** Every 10ms, gathers the metrics, ASN.1 encodes them into an **E2AP Indication Message**, and sends via SCTP.
2. **E2T:** Receives the SCTP packet. Extracts the Subscription ID. Wraps the ASN.1 payload in an RMR wrapper (Message Type: `RIC_INDICATION`). Sends to RMR.
3. **RMR:** Looks at the routing table. "Sub ID 42 goes to Traffic Steering xApp Pod at 10.244.1.15". Sends over TCP.
4. **xApp:** Receives the RMR message. Runs an ASN.1 decoder to extract the raw integers/floats.

### Step 3: Processing (The Intelligence)
1. The xApp feeds the decoded metrics into its algorithm (e.g., an ML model predicting future congestion).
2. The model detects Cell 1 is about to be congested and UE-A has a strong signal to Cell 2.
3. The xApp decides: "Handover UE-A to Cell 2."

### Step 4: Control (The Action)
1. The xApp crafts an **E2AP Control Message** using the E2SM-RC (RAN Control) service model. It specifies the target (UE-A) and the action (force handover to Cell 2).
2. It ASN.1 encodes the payload.
3. It calls `rmr_send_msg()` (Message Type: `RIC_CONTROL`).
4. **Conflict Manager:** (If deployed properly, the RMR route forces the message to the CM first). The CM evaluates the action. If approved, it forwards to E2T.
5. **E2T:** Strips the RMR wrapper, sends the ASN.1 payload over SCTP to the gNB.
6. **gNB:** Receives the Control message, translates it into an RRC (Radio Resource Control) message, and sends it over the air to UE-A to execute the handover.

---

## Part 3: The AI-Native O-RAN Hierarchy (2026 Research State)

The original promise of O-RAN was "intelligence." In 2026, we have moved past simple heuristics (if/then rules) into a fully **AI-Native Network Architecture**. The three tiers of applications (rApp, xApp, dApp) represent a hierarchy of timescales, compute power, and AI model complexity.

### 3.1 The rApp (Non-RT RIC): The Strategic Brain (>1 second)
**Execution Environment:** Centralized Telco Cloud (Standard Kubernetes, heavy GPU access).
**Data Source:** R1 interface (historical data from SMO, network digital twins).

**The 2026 AI Landscape in rApps:**
*   **Telecom Foundation Models (Telco-LLMs):** The biggest shift in 2026 is **Intent-Based Networking (IBN)** via LLMs. Instead of an engineer writing A1 policies, an operator types: *"Maximize energy efficiency in the downtown sector between 2 AM and 5 AM without dropping URLLC reliability below 99.999%."* The Telco-LLM (fine-tuned on 3GPP/O-RAN specs) translates this natural language intent into deterministic A1 JSON policies and pushes them to the Near-RT RIC.
*   **Federated Learning (FL) Orchestrators:** rApps act as global aggregators. They collect locally updated ML model weights from hundreds of edge xApps, aggregate them, and push the newly generalized global model back down via A1 Enrichment Information.
*   **Network Digital Twins (NDT):** Before sending a policy to an xApp, rApps like **COMIX** simulate the policy against a real-time digital twin of the RAN to proactively prove it won't cause conflicts.

### 3.2 The xApp (Near-RT RIC): The Tactical Optimizer (10ms – 1s)
**Execution Environment:** Regional Edge (Currently K8s/Containers; **Your research shifts this to Wasm + Intel GNR-D**).
**Data Source:** E2 interface (KPM telemetry from gNB).

**The 2026 AI Landscape in xApps:**
*   **Deep Reinforcement Learning (DRL):** Used heavily for Traffic Steering and Energy Saving. DRL models continuously observe the environment (state = PRB usage, UE RSRP) and take actions (E2 Control = handover commands) to maximize a reward signal (throughput).
*   **Graph Neural Networks (GNN):** Used by advanced Conflict Mitigation Functions (like **GraphSAGE**, Zolghadr WCNC 2025) to map the hidden relationships between different radio parameters.
*   **The Latency/Compute Bottleneck:** 2026 research explicitly identifies that running deep neural networks in Python/Containers within a 10ms loop is failing. The overhead of container orchestration ruins the deterministic latency required for ML inference. **This is why your Wasm + AMX (Advanced Matrix Extensions) architecture is perfectly timed.** You run ML inference directly on the CPU hardware accelerators inside a microsecond sandbox.

### 3.3 The dApp (O-DU / E3 Interface): The Real-Time Reflex (< 1ms)
**Execution Environment:** Cell Site / Far-Edge (Bare metal or Wasm embedded directly in the O-DU L1/L2 pipeline).
**Data Source:** Raw I/Q samples, MAC scheduler grants.

**The 2026 AI Landscape in dApps:**
*   **Neural Receivers:** Traditional PHY layer blocks (channel estimation, MIMO equalization) rely on complex mathematical models. **AI-Native PHY** (a major 6G stepping stone being tested in 2026) replaces these blocks entirely with deep neural networks trained to decode noisy signals directly into bits.
*   **CSI (Channel State Information) Compression:** UEs use autoencoders to compress Massive MIMO channel feedback, and the dApp on the O-DU uses the decoder half to reconstruct it, saving massive amounts of uplink bandwidth.
*   **Why Wasm?** You cannot run Docker in a 500µs radio slot. The O-RAN nGRG is pushing Wasm for dApps because it compiles to native machine code (AOT) and executes inline with the C/C++ L1 pipeline. **Note:** Your paper must clearly state that while dApps use Wasm for PHY-layer sub-ms loops, your architecture uses Wasm to fix the Near-RT RIC (10ms-1s) control plane.

---

## Part 4: Shared Data Layer (SDL) — The Extreme Physics Deep Dive

The SDL is the most misunderstood component of the RIC. It is not just "a database." It is the mechanism by which O-RAN achieves **stateless network functions**.

### 4.1 The Stateless Architecture Requirement
In traditional telecom (4G EPC), state (subscriber data, session keys) was stored in the memory of the specific server handling the connection. If the server crashed, the call dropped. 
5G introduced the **Stateless Architecture**. Compute (NFs) and state (UDSF/SDL) are physically separated. If an AMF pod dies, Kubernetes spins up a new one, which instantly pulls the state from the external database and continues as if nothing happened. The Near-RT RIC adopted this.

### 4.2 The SDL Latency Penalty (The Fundamental Physics Problem)
Here is the problem that papers like **CORMO-RAN** gloss over, and why your Wasm architecture is necessary.
- **The Speed of Light in the RIC:** A typical Near-RT RIC loop must complete in **10ms**.
- **The Network Penalty:** To read state from a Redis SDL pod, the xApp must: Serialize request (JSON/Protobuf) → traverse Linux TCP/IP stack → traverse veth pair/CNI → hit Redis pod → query memory → traverse stack back. 
- **The Math:** In a busy K8s cluster, one Redis round-trip takes **0.5ms to 2ms**. 
- **The ML Penalty:** If an xApp relies on a large ML matrix, serializing/deserializing a 10MB state object over TCP to Redis takes **5-15ms**. *You have just blown the entire E2 control loop budget on database overhead.*

### 4.3 How Your Architecture Fixes SDL
Because standard xApps suffer this latency penalty, many xApp developers cheat: they store state locally in the Python container's RAM. But when the pod is upgraded (Rolling Update), that RAM is destroyed, breaking the RAN loop.

**Your Wasm Architecture Solution:**
You replace the network-based Redis SDL for *fast-path state* with an **In-Process Host KV Store**.
- The Wasm xApp calls a host function: `get_ue_state(ue_id)`.
- The Wasm runtime (written in Go/Rust) pauses the Wasm sandbox, executes the host function (which reads from a localized, lock-free hash map in the host's memory), and writes the result directly into the Wasm linear memory via a DMA-like zero-copy memory transfer using **Intel DSA**.
- **Latency:** ~50 nanoseconds. (Orders of magnitude faster than Redis over K8s networking).
- **Hitless Upgrades:** When you swap the hardware DLB pointer to the `v2` Wasm module, the host KV store remains untouched. `v2` instantly accesses the exact same state `v1` was using. Zero network serialization, zero downtime.

---

## Part 5: Conflict Management (Theory vs 2026 Enforcement Paradigm)

### 5.1 The Three Types of Conflicts (The Theory)
1. **Direct Conflicts (Parameter Overlap):** Traffic Steering xApp (Vendor A) tells gNB to set UE-1's transmit power to 20dBm. Energy Saving xApp (Vendor B) tells gNB to set UE-1's transmit power to 10dBm. *Resolution: Simple priority override.*
2. **Indirect Conflicts (Cross-Parameter Interference):** Load Balancing xApp increases Cell Individual Offset (CIO). Coverage Optimization xApp tilts the physical antenna downward. They modified *different parameters*, but combined, they destroy the Handover Success Rate KPI. *Detection: Requires ML (GraphSAGE).*
3. **Implicit Conflicts (Cascading Triggers):** Energy Saving xApp turns off a carrier. Throughput drops. SLA Assurance xApp sees the drop and overrides the Energy Saving xApp. *Detection: Requires pre-deployment sandbox profiling (PACIFISTA).*

### 5.2 The Enforcement Gap (Your Paper's Thesis)
All the detection (GraphSAGE) and resolution (COMIX) logic assumes the xApp *actually sends its messages to the CMF*. 

In the standard OSC RIC, the CMF is just another microservice on the RMR bus.
- **The Intended Flow:** xApp → RMR → CMF → RMR → E2T → gNB.
- **The Exploit (CVE-2023-42358 concept):** A malicious or poorly coded xApp (running in a Linux container) simply opens a raw TCP socket, bypasses RMR entirely, and connects directly to the E2T's IP address. It injects a payload telling the gNB to drop all radio bearers. The CMF never sees it.

### 5.3 Structural vs. Policy Enforcement
- **eZTrust/OZTrust (2024):** Tried to solve this by attaching eBPF probes to the Linux network stack. When an xApp sends a packet, eBPF tags it with context. The receiver verifies the tag. This is *reactive* — it waits for the packet to hit the network stack and then filters it.
- **Your Wasm Sandbox (2026 State-of-the-Art):** This is *structural* enforcement. Wasm has **Capabilities-Based Security**. By default, a Wasm module cannot access the network, file system, or system clock. It has no OS concepts. 
- Therefore, the xApp *cannot physically create a socket to reach the E2T*. The only way it can influence the outside world is by calling the exact host function you provide: `send_e2_control(payload)`. 
- You embed the CMF logic inside that host function. If the payload conflicts with another xApp, the host function returns an error and drops the payload. **Bypass is mathematically impossible.**

---

## Part 6: The Application Lifecycle in Production (Container vs Wasm)

Let's look at how an xApp actually goes from a developer's laptop to controlling a live 5G cell, and why the K8s model fails the determinism test.

### 6.1 The Current Container-Based xApp Lifecycle (The Baseline)
1. **Packaging:** Developer writes xApp (Python/C++), compiles it with the RMR library, bundles it with ML weights, builds a Docker image (often 500MB+), and creates a Helm Chart.
2. **Onboarding:** Operator uploads the Helm Chart to the SMO's xApp Manager.
3. **Deployment (The Cold Start):** 
   - K8s pulls the 500MB image from the registry (Seconds).
   - K8s allocates a Pod, mounts volumes, configures CNI networking (Seconds).
   - Python runtime boots, loads ML weights into memory (Seconds).
   - *Total Time: 10 - 30 seconds.* (During a rolling update, this is the window where the E2 subscription breaks).
4. **Registration:** xApp sends HTTP POST to the RIC's Route Manager: *"I am TrafficSteering-v2, my IP is 10.244.1.55, route E2SM-KPM to me."*
5. **Subscription:** xApp requests data via SubMgr → E2T → gNB.
6. **Termination:** K8s sends SIGTERM. If the xApp doesn't gracefully clean up its subscriptions via SubMgr before dying, the gNB floods the network with orphaned telemetry.

### 6.2 The Wasm-Based Lifecycle (Your Research)
1. **Packaging:** Developer writes xApp in Rust/C/Go, compiles to `wasm32-wasi`. ML weights are stored separately. Total artifact size: ~2-5 MB.
2. **Onboarding:** Wasm binary pushed to an OCI registry.
3. **Deployment (The Microsecond Start):**
   - Wasm Edge runtime pulls the 2MB binary (Milliseconds).
   - Runtime compiles Wasm to native machine code via AOT (Ahead-of-Time).
   - Runtime instantiates the sandbox. Memory is linearly allocated (No OS booting, no CNI networking).
   - *Total Time: ~50 microseconds.*
4. **Registration:** Handled statically via Host Functions. No RMR route propagation required.
5. **Subscription / State Swap:** The hardware DLB pointer is atomically swapped from the old Wasm instance to the new one. The gNB never sees a disconnection.
6. **Execution:** Wasm xApp calls `send_e2_control()` — the host validates and enforces conflict rules instantly.
