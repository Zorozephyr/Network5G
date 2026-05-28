# 5G Production — How 5G Actually Runs in the Real World

> Bridging the gap between academic architecture diagrams and the physical reality of production 5G networks.

---

## Part 1: The Physical Hardware Stack

### 1.1 What's Inside a Cell Site (Base Station)

A modern 5G cell site contains:

**Radio Unit (RU):**
- **What it is physically:** A weatherproof metal box mounted on a tower, rooftop, or wall. Contains antennas, RF transceivers, power amplifiers, ADC/DAC.
- **Vendors:** Ericsson AIR 6449/6419, Nokia AirScale mMIMO, Samsung 5G Radio, Fujitsu, NEC
- **Massive MIMO:** Modern RUs have 32T32R or 64T64R antenna arrays (32/64 transmit, 32/64 receive elements). Each element can be independently controlled for beamforming — steering radio beams toward individual UEs.
- **Power:** A single 64T64R mMIMO RU draws 1-2 kW. Energy saving (via xApps turning off unused antenna elements or reducing power during low-traffic periods) is a major operational priority.
- **Cost:** $15,000–$50,000 per RU depending on MIMO configuration

**Distributed Unit (DU):**
- **What it is physically:** A 1U or 2U rackmount server at the cell site or a nearby edge facility. Runs L1 (PHY) and L2 (MAC/RLC) processing.
- **Hardware:** Dell PowerEdge XR7620/XR8620, HPE ProLiant DL110, Supermicro edge servers
- **CPU:** Intel Xeon-D or Xeon SP (for L2), paired with:
  - **Intel ACC100/ACC200 vRAN accelerator:** PCIe card that offloads FEC (Forward Error Correction — LDPC encode/decode). Without this, LDPC processing alone can consume 60-80% of CPU cycles.
  - **FPGA (Intel Agilex, AMD/Xilinx):** Alternative to ACC for PHY acceleration. More flexible but harder to program.
- **Memory:** 64-256 GB DDR5 (L1/PHY processing is memory-bandwidth intensive)
- **NIC:** Intel E810 (100GbE, supports eCPRI timestamping for fronthaul synchronization)
- **OS:** Wind River Linux, Red Hat Enterprise Linux (RHEL), Ubuntu Core
- **Timing:** GPS receiver + PTP (Precision Time Protocol, IEEE 1588) for microsecond-level synchronization. The DU must be time-synchronized with the RU to within ±1.5µs for correct OFDMA operation.

**Centralized Unit (CU):**
- **What it is physically:** A server at a regional data center or central office. Runs L3 (PDCP, RRC, SDAP). Less latency-sensitive than DU.
- **Hardware:** Standard COTS servers (Dell, HPE, Supermicro)
- **Deployment:** Often co-located with the Near-RT RIC and UPF at a regional edge site.

### 1.2 What's in the Data Center (Core Network)

**Servers:**
- Dell PowerEdge R760/R660, HPE ProLiant DL360/DL380, Supermicro SYS-2029U
- CPU: Intel Xeon Scalable (Sapphire Rapids, Emerald Rapids) or AMD EPYC (Genoa, Turin)
- 128-512 GB RAM, NVMe SSDs
- 2× 25GbE or 100GbE NICs

**NICs for Data Plane:**
- **Intel E810:** 100GbE, supports DPDK, SR-IOV, ADQ (Application Device Queues). The workhorse NIC for 5G UPF and vRAN.
- **NVIDIA ConnectX-7:** 200/400GbE, supports DPDK, SR-IOV, RDMA, in-NIC flow steering. Used for high-throughput UPFs and AI workloads.
- **Broadcom P2100G:** 100GbE, SmartNIC with offload engines.

**Switches:**
- **Leaf/Spine architecture:** Modern data centers use a two-tier Clos topology (leaf switches connect servers, spine switches interconnect leafs).
- **Arista 7050X4/7280R3:** 25/100/400GbE, BGP EVPN/VXLAN
- **Cisco Nexus 9300/9500:** Enterprise and telco data center
- **Nokia SR Linux:** Telecom-focused, native SRv6 support

**Load Balancers / Firewalls:**
- F5 BIG-IP, A10 Networks: Load balance across UPF instances
- Palo Alto PA-5400, Fortinet FortiGate: NGFW at network boundaries

### 1.3 The Transport Network Between Sites

**Fronthaul (RU ↔ DU):**
- **Distance:** 0-20 km (often same building or nearby edge site)
- **Technology:** Dark fiber, 25GbE Ethernet, eCPRI
- **Latency requirement:** < 100µs one-way (critical for HARQ timing)
- **Hardware:** Often point-to-point fiber with eCPRI switches (Cisco NCS 540, Nokia 7705 SAR-H)

**Midhaul (DU ↔ CU):**
- **Distance:** 5-40 km
- **Technology:** Ethernet/IP, routed network, can tolerate some jitter
- **Latency requirement:** < 1-5 ms

**Backhaul (CU/gNB ↔ Core):**
- **Distance:** 10-200+ km
- **Technology:** MPLS, SRv6, IP/Ethernet over fiber. Some rural sites use microwave backhaul.
- **Latency requirement:** < 10-20 ms (for non-URLLC)
- **Encryption:** IPsec or MACsec for security

---

## Part 2: Network Functions in Production

### 2.1 Who Builds What

| Vendor | Strengths | Products |
|--------|----------|----------|
| **Ericsson** | Largest RAN vendor globally (40% market share). End-to-end 5G. | Cloud RAN, Ericsson Intelligent Automation Platform (EIAP), Packet Core (UPF, AMF, SMF) |
| **Nokia** | #2 RAN vendor. Strong in enterprise private 5G. | AirScale RAN, AirFrame data center, MantaRay RIC, Core network |
| **Samsung** | Strong in US (Verizon, AT&T). Growing global share. | 5G RAN (vRAN, O-RAN), Samsung Core, Samsung RIC |
| **Huawei** | #1 in China, restricted in US/Europe. | Full 5G stack (RAN, Core, transport) |
| **Mavenir** | Open RAN pure-play. Cloud-native, disaggregated. | O-RAN-compliant DU/CU, RIC, Core NFs |
| **Parallel Wireless** | Open RAN for emerging markets. | O-RAN DU/CU, unified controller |
| **VMware (Broadcom)** | Telco cloud platform. | Telco Cloud Platform (runs NFs), RIC |
| **Wind River** | Operating system and cloud platform for RAN. | Wind River Studio (O-RAN-optimized OS + K8s) |
| **Red Hat (IBM)** | OpenShift for telco. Certified K8s for 5G. | OpenShift Container Platform (used by CORMO-RAN, AT&T) |
| **Rakuten Symphony** | Built Rakuten's 5G network. Now sells as platform. | Symworld (cloud-native RAN + Core + Automation) |
| **Druid Software** | Private 5G specialist. | Raemis private 5G core |
| **Affirmed Networks (Microsoft)** | Azure-hosted 5G Core. | Azure Private 5G Core (AMF, SMF, UPF as Azure service) |
| **AWS** | Cloud 5G. | AWS Private 5G (turnkey private network) |
| **Free5GC / Open5GS** | Open-source 5G Core. | Academic/test deployments |
| **OpenAirInterface (OAI)** | Open-source RAN + Core. | gNB, UPF, FlexRIC (used by BubbleRAN) |

### 2.2 Each NF in Production Detail

**AMF — Access and Mobility Management:**
- Deployed as: 3-5 pod replicas in K8s, active-active for HA
- Handles: 10,000–100,000 concurrent UE registrations per instance
- Scaling trigger: Number of registered UEs exceeds threshold
- Stateless design: Session state stored in external DB (e.g., Redis, etcd) for pod failover
- Protocol stack: NGAP (N2, over SCTP to gNB), NAS (N1, to UE via gNB), HTTP/2 (SBI to other NFs)

**SMF — Session Management:**
- Deployed as: Multiple replicas, often co-located with AMF
- Handles: PDU session creation/modification/release
- Critical decision: Selects which UPF to use based on: UE location, slice, data network name (DNN), operator policy
- Protocol: PFCP (N4, over UDP to UPF) — Packet Forwarding Control Protocol. Installs PDR/FAR/QER rules in the UPF.

**UPF — User Plane Function:**
- **Deployment tiers:**
  - **Central UPF:** Handles 100+ Gbps, deployed in core data center. Uses DPDK on high-core-count servers (Intel Xeon SP, 32-64 cores). May use Intel QAT for IPsec offload.
  - **Edge UPF (MEC):** Handles 10-40 Gbps, deployed at regional edge. Provides low-latency local breakout for enterprise applications.
  - **Far-edge UPF:** Handles 1-5 Gbps, deployed at or near the cell site. Used for private 5G and URLLC applications.
- **Major UPF implementations:**
  - Ericsson UPF: DPDK-based, proprietary
  - Nokia MX-GW: DPDK-based, supports SR-IOV
  - Affirmed Networks (Microsoft): Cloud-native UPF
  - free5GC/Open5GS UPF: Open-source, good for testing
  - YOUR research: eBPF/XDP + Wasm exception path (for far-edge/private 5G sweet spot)

**NSSF — Slice Selection:**
- Lightweight NF. Queried during UE registration to determine which slice(s) the UE is allowed to access.
- In practice, many operators embed NSSF logic directly into AMF rather than running it as a separate NF.

**NRF — Service Discovery:**
- The "service registry." Every NF registers with NRF on startup.
- Think of it as a Kubernetes Service but for 5G Core NFs.
- Uses OAuth2 tokens for inter-NF authentication.

**PCF — Policy Control:**
- Stores operator policies: QoS profiles per slice, charging rules, access restrictions.
- Queries UDR (Unified Data Repository) for subscriber-specific policies.
- Pushes policies to SMF (which configures UPF accordingly).

---

## Part 3: Deployment Models

### 3.1 Traditional RAN (Pre-O-RAN)
```
[Monolithic gNB: RU + DU + CU in one box] → [Core Network]
```
- Everything from one vendor (Ericsson, Nokia, or Huawei)
- No xApps, no RIC, no disaggregation
- Still the majority of deployed 5G networks worldwide (2026)

### 3.2 Disaggregated RAN (O-RAN)
```
[O-RU] →eCPRI→ [O-DU] →F1→ [O-CU] →E2→ [Near-RT RIC]
  Vendor A        Vendor B       Vendor C       Vendor D
```
- Mix-and-match vendors
- xApps for AI-driven optimization
- **Reality check:** Multi-vendor O-RAN is deployed by Rakuten, Dish, Vodafone (trials), but most operators use single-vendor O-RAN stacks

### 3.3 Cloud RAN (C-RAN / vRAN)
```
[O-RU] →fiber→ [Centralized DU+CU pool in data center]
```
- Multiple cell sites share a centralized DU/CU pool
- Enables statistical multiplexing (fewer servers needed than cells)
- Requires very low-latency, high-bandwidth fronthaul
- Used by Rakuten, Dish Network

### 3.4 Private 5G
```
[Enterprise O-RU] → [Local DU/CU] → [Local UPF] → [Enterprise Network]
                                           ↕
                                    [Hosted/Local 5G Core: AMF, SMF]
```
- All traffic stays on-premise (data sovereignty)
- Enterprise controls the network (QoS, slicing, security)
- Uses CBRS (US) or local licensed spectrum
- Vendors: Nokia DAC (Digital Automation Cloud), Ericsson Private 5G, Microsoft Azure Private 5G, AWS Private 5G, Druid Raemis

---

## Part 4: What Real Operators Deploy

### Rakuten Mobile (Japan) — The O-RAN Pioneer
- **Architecture:** Fully cloud-native, disaggregated O-RAN
- **RAN:** Altiostar (now Fujitsu) O-DU/O-CU, NEC/Nokia O-RU
- **Core:** Rakuten Symphony cloud-native core on K8s
- **RIC:** In-house Near-RT RIC + vendor xApps
- **Infrastructure:** Red Hat OpenShift, Intel Xeon servers, Wind River OS
- **Lesson learned:** Multi-vendor integration was extremely costly. Rakuten now sells its platform (Symworld) to other operators.

### AT&T (USA) — Single-Vendor O-RAN
- **Architecture:** O-RAN specifications, but single-vendor (Ericsson)
- **RIC:** Ericsson EIAP (includes xApps for traffic steering, energy saving)
- **Infrastructure:** Red Hat OpenShift, Dell/HPE servers
- **Key insight:** AT&T chose O-RAN for the architecture (disaggregation, RIC-based optimization) but not for multi-vendor mixing.

### Dish Network (USA) — Greenfield Cloud-Native
- **Architecture:** Built from scratch, no legacy 4G. Fully cloud-native, O-RAN.
- **RAN:** Fujitsu O-RU, Mavenir O-DU/O-CU
- **Core:** Mavenir cloud-native core
- **Cloud:** AWS Outposts at cell sites
- **Significance:** First operator to deploy a nationwide 5G network entirely in the cloud

### Jio (India) — Standalone 5G at Scale
- **Architecture:** 5G SA (Standalone), no dependency on 4G core
- **RAN:** Ericsson and Nokia (split by region), plus in-house Jio Platforms
- **Scale:** 300,000+ base stations, covering 1.4 billion population
- **Innovation:** Jio Platforms developing in-house 5G stack

### Vodafone (Europe) — Cautious O-RAN Trials
- **UK:** Samsung-only O-RAN deployment
- **Italy:** Nokia-only O-RAN deployment  
- **Germany:** O-RAN trial with multiple vendors (NEC, Samsung, Mavenir)
- **RIC:** Various vendor RICs in trials, exploring xApp ecosystem
- **Position paper:** Published detailed analysis of O-RU/O-DU integration challenges (cited by WA-RAN)

---

## Part 5: Real xApp/rApp Use Cases in Production

### Deployed xApp Examples

**Traffic Steering xApp:**
- **What it does:** Monitors per-cell load (PRB utilization) via KPM E2 Indications. When a cell is overloaded (>80% PRB usage), identifies UEs at cell edge that could be served by a neighbor cell with lower load. Sends E2 Control message to adjust A3 event parameters (handover thresholds) or directly triggers handover.
- **Deployed by:** Ericsson (RSAC xApp), Rimedo Labs, VIAVI
- **Control loop:** 100ms–1s

**Energy Saving xApp:**
- **What it does:** During low-traffic periods (night), detects underutilized cells. Sends E2 Control to reduce MIMO layers (64T → 8T), lower transmit power, or shut down entire carriers while maintaining coverage via macro cells. Monitors KPMs to ensure QoS doesn't degrade below threshold.
- **Impact:** 15-30% energy reduction in commercial deployments
- **Deployed by:** Ericsson, Nokia, multiple startups

**Anomaly Detection xApp:**
- **What it does:** ML model trained on KPM history. Detects anomalies: sudden throughput drops, excessive handover rates (ping-pong), unexpected interference. Triggers alerts or automated mitigation.
- **E2SM:** Uses KPM for monitoring, RC for control actions

### Deployed rApp Examples

**SLA Assurance rApp:**
- **What it does:** Monitors end-to-end slice performance (throughput, latency) via O1 PM data. If a slice violates its SLA (e.g., eMBB slice drops below guaranteed throughput), sends A1 policy to Near-RT RIC xApps to prioritize that slice's resources.
- **Timescale:** >1s (monitoring + policy decision)

**ML Model Training rApp:**
- **What it does:** Collects historical KPM data via O1, trains ML models (e.g., traffic prediction, handover prediction), and pushes trained model weights to xApps via A1 enrichment information.
- **Timescale:** Hours to days (training), then real-time (inference by xApp)

---

*Next: Document 5 — O-RAN Internals (E2T, RMR, SDL, xApp lifecycle, conflict management — the deep-dive into what your paper redesigns)*
