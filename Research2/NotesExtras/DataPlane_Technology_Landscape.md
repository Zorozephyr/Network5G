# Data Plane Technology Landscape: Software Stack → DPDK → SmartNIC → DPU

## A Comprehensive Analysis of What Is Used Where and Why

---

## 1. The Four Tiers of Data Plane Processing

```
┌─────────────────────────────────────────────────────────────────────────┐
│              Data Plane Technology Spectrum (2026)                       │
│                                                                         │
│  Tier 1            Tier 2            Tier 3             Tier 4          │
│  Software Stack    DPDK/VPP          SmartNIC           DPU/IPU        │
│  ──────────────    ────────          ────────           ───────         │
│  Linux Kernel      User-space        HW pipeline +     Full SoC:       │
│  eBPF/XDP          kernel bypass     P4/FPGA/ASIC      ARM cores +     │
│  AF_XDP            Poll Mode Drv     offload engines   accelerators +  │
│  Netfilter/TC      Hugepages/Ring    RSS/Flow steering own OS          │
│                    Zero-copy                                            │
│                                                                         │
│  ◄── Flexibility / Programmability                                      │
│                                          Raw Performance / Isolation ──►│
│  ◄── Lower Cost / OpEx                                                  │
│                                           Higher Cost / CapEx ────────►│
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Generational Evolution: 3G → 4G → 5G

### 2.1 The 3G Era: Proprietary Hardware (SGSN/GGSN)

In 3G UMTS networks, all data plane processing was done on **proprietary, purpose-built hardware appliances**.

```
3G Packet Core:
┌──────────┐  GTP-U (Gn)  ┌──────────┐   SGi   ┌──────────┐
│   SGSN   │─────────────→│   GGSN   │────────→│ Internet │
│ (Serving)│              │(Gateway) │         └──────────┘
└──────────┘              └──────────┘
     │                         │
     │  Both are monolithic,   │
     │  vendor-locked hardware │
     │  appliances (Ericsson,  │
     │  Huawei, Nokia/Siemens) │
```

| Aspect | 3G Reality |
| :--- | :--- |
| **Hardware** | Custom ASICs, proprietary backplanes (ATM/TDM) |
| **Software** | Vendor-specific RTOS, not Linux |
| **Data Plane** | Fixed-function silicon, not programmable |
| **Scaling** | Buy a bigger box (vertical scaling only) |
| **Flexibility** | Zero — new features require new hardware |
| **Vendors** | Ericsson, Huawei, Nokia-Siemens, Cisco (GGSN) |

**Key Takeaway:** There was no "Tier" choice. Everything was Tier 4-equivalent (proprietary SoC), but with zero programmability. The industry was fully hardware-locked.

---

### 2.2 The 4G Era: The NFV Revolution (SGW/PGW)

4G LTE introduced **CUPS (Control & User Plane Separation)** in 3GPP Release 14, splitting the monolithic gateways:
- SGW → SGW-C (Control) + SGW-U (User Plane)
- PGW → PGW-C (Control) + PGW-U (User Plane)

This enabled the **NFV (Network Functions Virtualization)** revolution: running network functions as software on COTS x86 servers.

```
4G EPC with CUPS:
┌──────────┐                 ┌──────────────────┐
│  SGW-C   │───── PFCP ─────→│     SGW-U        │
│  PGW-C   │                │     PGW-U        │
│ (Control)│                │ (User Plane)     │
└──────────┘                │                  │
                            │  DPDK + VPP      │
                            │  on COTS x86     │
                            └──────────────────┘
```

**This is where DPDK became the industry standard for telecom data planes.**

| Aspect | 4G Reality (Post-CUPS) |
| :--- | :--- |
| **Dominant Tier** | **Tier 2 (DPDK)** on COTS servers |
| **Why DPDK?** | Kernel stack couldn't hit 10-40 Gbps GTP-U throughput |
| **Key Framework** | FD.io VPP (Vector Packet Processing) on top of DPDK |
| **Deployment** | VMs on VMware ESXi / KVM with SR-IOV passthrough |
| **Vendors** | Cisco (StarOS), Ericsson (vEPC), Samsung, Mavenir |

**Key Takeaway:** 4G was the transition era. Operators moved from proprietary appliances to DPDK-based VNFs, but still deployed them in traditional VM-centric infrastructure.

---

### 2.3 The 5G Era: Heterogeneous, Cloud-Native (UPF)

5G fully embraces cloud-native architecture. The UPF is designed from the ground up as a containerized, microservice-based function. **All four tiers are actively used**, often simultaneously in a single deployment.

```
5G UPF — Multi-Tier Architecture:

┌─────────────────────────────────────────────────────────────┐
│                    5G UPF Node                              │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Tier 4: DPU (BlueField / Pensando)                   │   │
│  │ • Hardware trust boundary for multi-tenancy           │   │
│  │ • OVS offload for pod-to-pod traffic                 │   │
│  │ • IPsec / MACsec line-rate encryption                │   │
│  └──────────────────────────────────────────────────────┘   │
│                          ↕                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Tier 3: SmartNIC (P4-programmable)                   │   │
│  │ • GTP-U encap/decap at wire speed                    │   │
│  │ • RSS on inner IP / TEID for flow distribution       │   │
│  │ • Hardware flow table for known-good sessions        │   │
│  └──────────────────────────────────────────────────────┘   │
│                          ↕                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Tier 2: DPDK / VPP (User-space fast path)            │   │
│  │ • Complex PDR/FAR rule matching                      │   │
│  │ • QoS enforcement, traffic shaping per slice         │   │
│  │ • UL Classifier / Branching Point logic              │   │
│  └──────────────────────────────────────────────────────┘   │
│                          ↕                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Tier 1: eBPF/XDP / AF_XDP (Cloud-native path)       │   │
│  │ • DDoS mitigation at NIC driver level                │   │
│  │ • Lightweight flow steering & load balancing         │   │
│  │ • Observability (per-flow telemetry via BPF maps)    │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. The Master Use Case Map

### 3.1 Telecom Use Cases

| Use Case | Dominant Tier | Why? | Key Products/Vendors |
| :--- | :--- | :--- | :--- |
| **3G SGSN/GGSN** | Proprietary HW | Legacy; no software option existed | Ericsson, Huawei, Nokia |
| **4G SGW-U/PGW-U** | Tier 2 (DPDK/VPP) | CUPS enabled SW-based UPF; DPDK hit 10-40G | Cisco StarOS, Mavenir, Samsung |
| **5G UPF (Central)** | Tier 2+3 (DPDK + SmartNIC) | High throughput (100G+); GTP-U offload to HW | Ericsson, Nokia, Mavenir + Intel E810 |
| **5G UPF (Edge/MEC)** | Tier 1+2 (eBPF + DPDK) | Cloud-native K8s; low footprint; flexibility | free5GC, Open5GS, Cilium-based UPFs |
| **5G UPF (Multi-tenant)** | Tier 4 (DPU) | Hardware isolation between tenant slices | NVIDIA BlueField-3 + vendor UPF |
| **vDU (L1 PHY)** | Tier 2 (DPDK) + HW Accel | Real-time eCPRI; needs DPDK + FEC offload | Intel FlexRAN, Qualcomm, Samsung |
| **vCU (RRC/PDCP)** | Tier 1 (Kernel) | Not real-time; standard networking sufficient | Ericsson, Nokia, Parallel Wireless |
| **IMS / SBC** | Tier 2 (DPDK) | SIP/RTP at scale requires kernel bypass | Oracle SBC, Orvio (Orvio uses VPP) |
| **Cell Site Router (HW)** | Proprietary ASIC | Fixed-function; deterministic; outdoor-grade | Cisco NCS 540, Nokia 7705 SAR |
| **Cell Site Router (SW)** | Tier 2 (DPDK) | vCSR on COTS; used where HW CSR is overkill | Cisco IOS XRv, Arrcus ArcOS |

### 3.2 Enterprise Use Cases

| Use Case | Dominant Tier | Why? | Key Products/Vendors |
| :--- | :--- | :--- | :--- |
| **SD-WAN Appliance** | Tier 2 (DPDK) | IPsec + DPI + path selection need kernel bypass | VMware SD-WAN, Fortinet, Cisco Viptela |
| **Enterprise Firewall (HW)** | Proprietary ASIC | Line-rate inspection requires fixed silicon | Palo Alto, Fortinet FortiGate (NP7) |
| **Enterprise Firewall (SW/VM)** | Tier 2 (DPDK) | Virtual firewall on cloud needs fast path | Fortinet vSPU, Palo Alto VM-Series |
| **Enterprise Router (HW)** | Proprietary ASIC | Deterministic forwarding at edge | Cisco Catalyst, Juniper MX |
| **Enterprise Router (SW)** | Tier 1 (Kernel/FRR) | Low-cost; open-source; runs on any x86 | FRRouting, VyOS, Cisco CSR 1000v |
| **WAN Optimization** | Tier 2 (DPDK) | TCP acceleration, dedup, compression at speed | Riverbed SteelHead, Silver Peak |
| **Private 5G UPF** | Tier 1+2 (eBPF/DPDK) | Small footprint; single-box deployment | Blue Arcus, Druid, Celona |

### 3.3 Cloud / Hyperscaler Use Cases

| Use Case | Dominant Tier | Why? | Key Products/Vendors |
| :--- | :--- | :--- | :--- |
| **Virtual Switch (OVS)** | Tier 3+4 (SmartNIC/DPU) | OVS offload eliminates host CPU overhead | NVIDIA BlueField OVS, AWS Nitro |
| **Bare-Metal Provisioning** | Tier 4 (DPU) | DPU manages network boot & storage without host OS | AWS Nitro, Google Titanium |
| **Tenant Isolation** | Tier 4 (DPU) | Hardware trust boundary; host CPU cannot bypass | All hyperscalers (custom DPUs) |
| **Storage (NVMe-oF)** | Tier 4 (DPU) | RDMA + NVMe over Fabric offloaded to DPU | NVIDIA SNAP, Pensando DSC |
| **Load Balancer (L4)** | Tier 1+3 (XDP + SmartNIC) | Stateless LB at NIC driver level; millions of CPS | Meta's Katran (XDP), Cloudflare |
| **Service Mesh (sidecar)** | Tier 1 (eBPF) | Per-pod L7 proxy replaced by in-kernel eBPF | Cilium, Istio Ambient Mesh |

### 3.4 Other / Specialized Use Cases

| Use Case | Dominant Tier | Why? | Key Products/Vendors |
| :--- | :--- | :--- | :--- |
| **DDoS Mitigation** | Tier 1+3 (XDP + SmartNIC) | Drop malicious packets before they reach CPU | Cloudflare, Fastly, Akamai |
| **High-Freq Trading (HFT)** | Tier 3 (FPGA SmartNIC) | Sub-microsecond deterministic latency | Xilinx Alveo, Solarflare |
| **CDN / Edge Caching** | Tier 1+2 (eBPF + DPDK) | Serve cached content at kernel level; bypass app | Netflix (FreeBSD kqueue), Varnish |
| **AI/HPC Networking** | Tier 4 (SuperNIC/DPU) | GPU-to-GPU RDMA; congestion control offload | NVIDIA ConnectX-7, Spectrum-X |
| **Telemetry / Monitoring** | Tier 1 (eBPF) | In-kernel per-packet/flow visibility; zero overhead | Cilium Hubble, Pixie, Datadog |
| **Intrusion Detection** | Tier 2+3 (DPDK + SmartNIC) | Deep Packet Inspection at wire speed | Suricata (DPDK mode), Snort 3 |

---

## 4. Decision Framework: How to Choose

```
                        START
                          │
                    ┌─────▼─────┐
                    │ Need >50G │
                    │ throughput?│
                    └─────┬─────┘
                     Yes  │  No
              ┌───────────┴───────────┐
              ▼                       ▼
    ┌──────────────┐         ┌──────────────┐
    │ Is the task  │         │ Cloud-native │
    │ repetitive & │         │ / K8s env?   │
    │ fixed-func?  │         └──────┬───────┘
    └──────┬───────┘           Yes  │  No
      Yes  │  No          ┌────────┴────────┐
    ┌──────┴──────┐       ▼                 ▼
    ▼             ▼   Tier 1             Tier 2
  Tier 3       Tier 2  (eBPF/XDP/        (DPDK/VPP)
  (SmartNIC)   (DPDK   AF_XDP)
               +SmartNIC)
              
    ┌─────────────────────┐
    │ Need HW-enforced    │
    │ multi-tenant        │
    │ isolation?          │
    └─────────┬───────────┘
         Yes  │
              ▼
           Tier 4
           (DPU)
```

### Quick Decision Table

| Requirement | Recommended Tier |
| :--- | :--- |
| Max flexibility, low cost, cloud-native | **Tier 1** (eBPF/XDP) |
| Proven high-perf (10-100G), complex packet logic | **Tier 2** (DPDK/VPP) |
| Line-rate offload of fixed functions (GTP-U, ACL) | **Tier 3** (SmartNIC) |
| Multi-tenant isolation, infra offload, storage+net | **Tier 4** (DPU) |
| Deterministic sub-μs latency (HFT, URLLC) | **Tier 3** (FPGA SmartNIC) |
| AI/GPU cluster networking | **Tier 4** (SuperNIC) |
| DDoS / L4 Load Balancing | **Tier 1** (XDP) + **Tier 3** |
| Legacy 3G/4G migration | **Tier 2** (DPDK) on COTS |

---

## 5. Industry Trends (2025-2026)

### 5.1 The eBPF/AF_XDP Rise
- **AF_XDP** is being actively evaluated as a **DPDK replacement** for cloud-native UPFs. It provides near-DPDK performance while using standard Linux NIC drivers (no PMD lock-in).
- DPDK itself now supports AF_XDP as a backend PMD, enabling coexistence.
- Cilium's eBPF datapath is being adopted for L4 load balancing at Meta, Google, and Cloudflare scale.

### 5.2 DPU Standardization
- ~50% of cloud service providers now use DPUs for workload optimization.
- AWS Nitro, Google Titanium, and Azure custom DPUs have proven the model.
- NVIDIA BlueField-3 is the most widely deployed commercial DPU.
- The **OPI (Open Programmable Infrastructure)** project is standardizing DPU APIs.

### 5.3 SmartNIC Programmability
- P4-programmable SmartNICs are replacing fixed-function NICs in telecom.
- Intel DDP (Dynamic Device Personalization) enables runtime GTP-U parsing on E810 NICs.
- AMD Pensando (Elba/Salina) and NVIDIA ConnectX-7 dominate the market.

### 5.4 The Convergence
The clear trend is **convergence**: a single platform combining eBPF for observability, DPDK/VPP for complex packet logic, SmartNIC for wire-rate offload, and DPU for infrastructure isolation. Vendors are building unified stacks that span all four tiers.

---

## 6. Key Research Papers (2024-2026)

| Paper | Venue | Key Contribution |
| :--- | :--- | :--- |
| **Alkali** | NSDI '25 | Target-independent SmartNIC programming framework; within 9.8% of hand-tuned perf |
| **Astraea** | SIGCOMM '25 | Performance isolation for multi-tenant DPU workloads in public clouds |
| **Buddy (Communication Offloading on DPUs)** | arXiv 2026 | Quantitative "fire-and-forget" async model on BlueField-3; identifies DRAM bottleneck |
| **DPU-Bench** | LANL 2025 | Micro-benchmark suite for DPU communication offload efficiency |
| **Stateless Firewalling Evaluation** | USC 2025 | Compares host DPDK vs. SmartNIC vs. P4-switch for firewall throughput/latency |
| **iPipe** | SIGCOMM '19 | Foundational framework for building SmartNIC-accelerated applications |
| **SPRIGHT** | SIGCOMM '22 | eBPF-based serverless framework for 5G edge computing |

---

## 7. Vendor Landscape Summary

```
┌─────────────────────────────────────────────────────────────────┐
│                  Data Plane Vendor Map (2026)                    │
│                                                                 │
│  TIER 1 (Software Stack):                                       │
│  ├── Linux Foundation: eBPF, XDP, AF_XDP                       │
│  ├── Isovalent/Cisco: Cilium (eBPF-based CNI + L4 LB)          │
│  └── Meta: Katran (XDP-based L4 load balancer)                 │
│                                                                 │
│  TIER 2 (DPDK / User-Space):                                   │
│  ├── Linux Foundation: DPDK                                    │
│  ├── FD.io: VPP (Vector Packet Processing)                     │
│  ├── Travelping: UPG-VPP (open-source 5G UPF)                 │
│  └── Vendors: Mavenir, Ericsson, Samsung (commercial UPFs)     │
│                                                                 │
│  TIER 3 (SmartNIC):                                             │
│  ├── Intel: E810 (with DDP for GTP-U), Mt. Evans IPU           │
│  ├── AMD/Pensando: Elba (2nd gen), Salina (3rd gen)            │
│  ├── NVIDIA: ConnectX-7 (with P4 FlexIO)                      │
│  └── Xilinx/AMD: Alveo (FPGA-based, for HFT/custom)           │
│                                                                 │
│  TIER 4 (DPU / IPU):                                           │
│  ├── NVIDIA: BlueField-3 (400G, 16x ARM A78)                  │
│  ├── Intel: IPU E2100 (Mt. Evans) / E2200 (Mt. Morgan)        │
│  ├── AMD/Pensando: Pollara 400 (AI-optimized)                 │
│  ├── AWS: Nitro (custom, internal)                             │
│  ├── Google: Titanium (custom, internal)                       │
│  └── Microsoft Azure: Custom SmartNIC/DPU (internal)           │
└─────────────────────────────────────────────────────────────────┘
```
