# Module 4A: URLLC Requirements & Software vs Hardware Routers (CSR)

## 1. URLLC — Ultra-Reliable Low-Latency Communication

### 1.1 What is URLLC?

URLLC is one of the three foundational 5G service categories defined by 3GPP (alongside eMBB and mMTC). It targets mission-critical applications where **a single missed packet can be catastrophic**.

```
The Three 5G Service Pillars:

            Throughput (Gbps)
                 ▲
                 │
          10Gbps │      ┌─────────┐
                 │      │  eMBB   │  (4K Video, AR/VR)
                 │      │         │
                 │      └─────────┘
                 │
                 │                          ┌─────────┐
                 │                          │  URLLC  │
           1Gbps │                          │ (1ms,   │
                 │                          │  5-nines)│
                 │                          └─────────┘
                 │
                 │  ┌──────────┐
                 │  │  mMTC    │  (IoT Sensors, Smart City)
                 │  │ (1M dev/ │
                 │  │  km²)   │
                 └──┴──────────┴──────────────────────►
                               Latency (ms)
                 10ms         1ms         0.5ms
```

### 1.2 The Hard Numbers (3GPP Specifications)

| KPI | Target | 3GPP Reference |
|---|---|---|
| **User-Plane Latency** | ≤ 1 ms (one-way) | TR 38.913 |
| **Reliability** | 99.999% ("five nines") | TS 22.261 |
| **Availability** | 99.9999% (mission-critical) | TS 22.804 |
| **Packet Error Rate** | ≤ 10⁻⁵ (up to 10⁻⁷) | TR 38.913 |
| **Jitter** | < 1 μs (deterministic) | Implementation target |
| **Survival Time** | 0 ms (for some use cases) | TS 22.104 |

**What "five nines" means in practice**: Out of 100,000 packets, at most **1** can be lost or delivered late. For industrial control at 10⁻⁷, that's 1 in 10 million.

### 1.3 Where Does the 1ms Budget Go?

The end-to-end latency budget must be split across every hop:

```
┌──────────────────────────────────────────────────────────────────┐
│              URLLC 1ms Latency Budget Breakdown                  │
│                                                                  │
│  UE Processing    Radio (Air)     Transport      UPF Processing  │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐   ┌──────────┐    │
│  │  ~100 μs │ →  │ ~250 μs  │ →  │ ~100 μs  │ → │ ~50 μs   │    │
│  │          │    │          │    │          │   │          │    │
│  │ Encoding │    │ TTI +    │    │ Fiber +  │   │ Decap +  │    │
│  │ + PHY    │    │ HARQ     │    │ Switches │   │ Forward  │    │
│  └──────────┘    └──────────┘    └──────────┘   └──────────┘    │
│                                                                  │
│  ◄──────────────── Total ≤ 1000 μs ──────────────────────────►  │
│                                                                  │
│  ⚠ ZERO margin for software stack delays, queueing, or jitter  │
└──────────────────────────────────────────────────────────────────┘
```

**Key insight**: The UPF gets roughly **50 μs** of the total budget. This is why kernel-bypass (DPDK/XDP) and hardware offload are non-negotiable for URLLC.

---

## 2. RAN-Level Techniques Enabling URLLC

### 2.1 Mini-Slot Scheduling

Traditional 5G NR uses **14-symbol slots**. For URLLC, the scheduler can allocate **mini-slots** (2, 4, or 7 symbols) for faster turnaround:

```
Standard Slot (14 symbols @ 30kHz SCS = 0.5 ms):
┌──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┐
│S0│S1│S2│S3│S4│S5│S6│S7│S8│S9│10│11│12│13│  = 0.5 ms
└──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┴──┘

Mini-Slot (2 symbols @ 30kHz SCS ≈ 71 μs):
┌──┬──┐
│S0│S1│  = ~71 μs  ← 7× faster transmission opportunity!
└──┴──┘

Mini-Slot Preemption (Puncturing):
┌──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┐
│  │  │  │  │  │eMBB data (lower priority) │  │
│  │  │  │  │  └──┬──┬──┘  │  │  │  │  │  │
│  │  │  │  │     │▓▓│▓▓│  │  │  │  │  │  │ ← URLLC mini-slot
│  │  │  │  │     │UR│LL│  │  │  │  │  │  │   preempts eMBB!
└──┴──┴──┴──┴─────┴──┴──┴──┴──┴──┴──┴──┴──┘
```

### 2.2 Configured Grant (Grant-Free Access)

The traditional uplink handshake adds ~4ms of latency. Configured Grant eliminates it:

```
Traditional Uplink (Dynamic Grant):
┌────────────────────────────────────────────────────────┐
│  UE                                          gNB      │
│   │                                           │       │
│   │── Scheduling Request (SR) ──────────────→ │       │
│   │                                    (~1ms) │       │
│   │ ←────────────────── Uplink Grant ─────────│       │
│   │                                    (~1ms) │       │
│   │── Data on granted resources ────────────→ │       │
│   │                                    (~1ms) │       │
│   │ ←────────────────── HARQ ACK ─────────────│       │
│   │                                           │       │
│   Total: ~4ms before data is delivered!               │
└────────────────────────────────────────────────────────┘

Configured Grant (Grant-Free):
┌────────────────────────────────────────────────────────┐
│  UE                                          gNB      │
│   │                                           │       │
│   │  (Pre-configured periodic resources)      │       │
│   │                                           │       │
│   │── Data IMMEDIATELY on reserved slot ────→ │       │
│   │                                  (~0.5ms) │       │
│   │ ←────────────────── HARQ ACK ─────────────│       │
│   │                                           │       │
│   Total: ~0.5ms! No handshake needed.                 │
└────────────────────────────────────────────────────────┘
```

**Trade-off**: Configured Grant wastes radio resources if the UE has nothing to send during its reserved slot. This is acceptable for URLLC because reliability > efficiency.

### 2.3 PDCP Duplication

For extreme reliability (99.9999%), 3GPP supports **packet duplication** at the PDCP layer:

```
┌───────────────────────────────────────────────────────────┐
│                   PDCP Duplication                         │
│                                                           │
│  ┌──────┐                                                 │
│  │ PDCP │ ─── Duplicate packet ──→ Leg 1 (Primary Cell)   │
│  │      │                                                 │
│  │      │ ─── Duplicate packet ──→ Leg 2 (Secondary Cell) │
│  └──────┘                                                 │
│                                                           │
│  At the receiver (DU/CU):                                 │
│  • First arriving copy is accepted                        │
│  • Duplicate is discarded                                 │
│  • Even if one radio link fails, data arrives via other   │
│                                                           │
│  Reliability: P(both fail) = P(fail₁) × P(fail₂)         │
│  If each link = 99.9%, combined = 99.9999%                │
└───────────────────────────────────────────────────────────┘
```

---

## 3. UPF Data Plane Requirements for URLLC

### 3.1 The URLLC UPF Architecture

A UPF serving URLLC traffic must be architecturally distinct from a general-purpose eMBB UPF:

```
┌───────────────────────────────────────────────────────────────┐
│                    URLLC-Optimized UPF                         │
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                   FAST PATH (< 5μs)                     │  │
│  │                                                         │  │
│  │  ┌──────────┐   ┌──────────────┐   ┌────────────────┐  │  │
│  │  │ SmartNIC │   │ GTP-U Parse  │   │ Flow Table     │  │  │
│  │  │ Pipeline │ → │ + Decap (HW) │ → │ Lookup (TCAM)  │  │  │
│  │  └──────────┘   └──────────────┘   └───────┬────────┘  │  │
│  │                                            │           │  │
│  │                                    ┌───────▼────────┐  │  │
│  │                                    │ Forward to N6  │  │  │
│  │                                    │ (Direct DMA)   │  │  │
│  │                                    └────────────────┘  │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                 SLOW PATH (Exceptions Only)             │  │
│  │                                                         │  │
│  │  • New PDU session setup                                │  │
│  │  • Policy updates from SMF (PFCP)                       │  │
│  │  • Unknown flows → classify, install fast-path rule     │  │
│  │  • OAM / Telemetry reporting                            │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
│  Deployment: At the FAR EDGE, co-located with gNB / MEC      │
└───────────────────────────────────────────────────────────────┘
```

### 3.2 URLLC UPF: Non-Negotiable Requirements

| Requirement | Why | How |
|---|---|---|
| **Kernel Bypass** | Kernel adds 50+ μs jitter | DPDK PMD or AF_XDP |
| **Dedicated CPU Cores** | Context switches destroy determinism | `isolcpus` + CPU pinning |
| **RT Kernel** | Standard scheduler has unbounded latency | `PREEMPT_RT` patch |
| **Edge Deployment** | Transport latency must be < 100 μs | Co-locate with gNB |
| **HW Offload** | Software can't guarantee sub-5μs | SmartNIC fast path |
| **Priority Queuing** | URLLC must never wait behind eMBB | Strict Priority (SP) scheduling |
| **Packet Duplication** | 99.999% requires redundancy | Dual-path forwarding |

---

## 4. Software CSR vs Hardware Routers

### 4.1 What is a CSR?

**CSR = Cloud Services Router** — a fully virtualized router that runs as a VM or container on COTS (Commercial Off-The-Shelf) x86 servers, as opposed to a dedicated hardware appliance.

```
Hardware Router (Traditional):                    Software Router (CSR):
┌─────────────────────────┐                      ┌─────────────────────────┐
│   Cisco ASR 9000        │                      │   COTS x86 Server       │
│                         │                      │                         │
│  ┌───────────────────┐  │                      │  ┌───────────────────┐  │
│  │ Custom ASIC       │  │                      │  │  VM / Container   │  │
│  │ (Silicon One)     │  │                      │  │  ┌─────────────┐  │  │
│  │                   │  │                      │  │  │ IOS XE /    │  │  │
│  │ • Line-rate FIB   │  │                      │  │  │ IOS XR      │  │  │
│  │ • HW ACLs         │  │                      │  │  │ (Software)  │  │  │
│  │ • HW QoS          │  │                      │  │  └─────────────┘  │  │
│  │ • HW TCAM         │  │                      │  │                   │  │
│  └───────────────────┘  │                      │  │  Generic x86 CPU  │  │
│                         │                      │  │  (all processing  │  │
│  Dedicated chassis,     │                      │  │   done in SW)     │  │
│  power, cooling         │                      │  └───────────────────┘  │
│                         │                      │                         │
│  Cost: $50K - $500K+    │                      │  Cost: $5K - $20K       │
└─────────────────────────┘                      └─────────────────────────┘
```

### 4.2 CSR Product Landscape

| Product | OS Family | Target User | Physical Equivalent |
|---|---|---|---|
| **Cisco CSR 1000v** | IOS XE | Enterprise / Cloud | ASR 1000 / ISR Series |
| **Cisco IOS XRv 9000** | IOS XR (QNX microkernel) | Service Provider | ASR 9000 / NCS Series |
| **Juniper vMX** | Junos | SP / Enterprise | MX Series |
| **VyOS / FRRouting** | Linux-native | Open-source / Lab | N/A |
| **Nokia SR Linux** | Linux-native (containerized) | SP / DC | Nokia 7750 SR |

### 4.3 Architecture: Software vs Hardware Forwarding

```
Hardware Router Forwarding:
┌──────────────────────────────────────────────────────────┐
│                                                          │
│  Packet → Port ASIC → TCAM Lookup → Action Engine → Out │
│                                                          │
│  • TCAM: Ternary Content-Addressable Memory              │
│    - Parallel lookup of ALL entries simultaneously       │
│    - O(1) lookup time regardless of table size           │
│    - Deterministic: always takes same number of cycles   │
│                                                          │
│  • Result: Line-rate forwarding at 100G/400G/800G        │
│            regardless of # of routes or ACL rules        │
└──────────────────────────────────────────────────────────┘

Software Router Forwarding:
┌──────────────────────────────────────────────────────────┐
│                                                          │
│  Packet → NIC → DMA → CPU Cache → FIB Lookup (SW) → Out │
│                                                          │
│  • FIB stored in DRAM (not TCAM)                         │
│    - Longest-prefix match via trie / hash table          │
│    - O(W) lookup (W = prefix length, ~32 for IPv4)       │
│    - Performance degrades with table size                │
│                                                          │
│  • ACLs processed sequentially by CPU                    │
│  • QoS enforcement done in software scheduler            │
│                                                          │
│  • Result: 1-40 Gbps (depends on core count + features)  │
│            Performance varies with traffic profile       │
└──────────────────────────────────────────────────────────┘
```

### 4.4 When to Use Software vs Hardware Routers

```
┌──────────────────────────────────────────────────────────────────┐
│                   Decision Matrix                                 │
│                                                                   │
│  Throughput Requirement                                           │
│       ▲                                                           │
│       │                                                           │
│ 400G+ │  ┌───────────────────────────────────────┐                │
│       │  │        HARDWARE ONLY                   │                │
│       │  │  (ASR 9000, NCS 5500, Juniper MX)      │                │
│       │  └───────────────────────────────────────┘                │
│       │                                                           │
│  100G │  ┌───────────────────────────────────────┐                │
│       │  │    HARDWARE or DPU-ACCELERATED CSR     │                │
│       │  │  (SmartNIC offload + software control) │                │
│       │  └───────────────────────────────────────┘                │
│       │                                                           │
│  10G  │  ┌───────────────────────────────────────┐                │
│       │  │        SOFTWARE CSR (Pure VM/CNF)      │                │
│       │  │  (CSR 1000v, IOS XRv, vMX, FRR)        │                │
│       │  └───────────────────────────────────────┘                │
│       │                                                           │
│       └──────────────────────────────────────────────►            │
│            Flexibility / Cloud-Native / Agility                   │
└──────────────────────────────────────────────────────────────────┘
```

### 4.5 Software CSR vs Hardware Router — Head-to-Head

| Feature | Hardware Router (ASR 9000) | Software CSR (IOS XRv 9000) |
|---|---|---|
| **Forwarding Plane** | Custom ASIC / TCAM | x86 CPU + DPDK |
| **Throughput** | 100G – 800G line-rate | 1G – 40G (CPU-bound) |
| **Latency** | < 1 μs (deterministic) | 10-100 μs (variable) |
| **Routing Table Scale** | Millions of routes (TCAM) | Hundreds of thousands (DRAM) |
| **ACL Performance** | Line-rate (parallel TCAM) | Degrades with # of rules |
| **QoS** | HW queues (8-16 per port) | SW scheduler (less precise) |
| **Cost** | $50K-$500K+ | $5K-$20K (VM license + server) |
| **Deployment Speed** | Weeks (order, ship, rack) | Minutes (spin up VM) |
| **Elasticity** | Fixed capacity | Scale up/down dynamically |
| **Feature Parity** | Full feature set | 90-95% of features |
| **Use Case** | Core / Peering / Backbone | Edge / Branch / Cloud gateway |

### 4.6 The Convergence: DPU-Accelerated Software Routers

The future blurs the line between HW and SW routers:

```
┌──────────────────────────────────────────────────────────────┐
│          DPU-Accelerated Software Router (Future)             │
│                                                               │
│  ┌──────────────────────────────────────┐                     │
│  │         Software Control Plane       │                     │
│  │  (IOS XR / SR Linux on ARM or x86)  │                     │
│  │  • BGP, OSPF, IS-IS                 │                     │
│  │  • Segment Routing policies         │                     │
│  │  • Telemetry & OAM                  │                     │
│  └────────────────┬─────────────────────┘                     │
│                   │ Programs HW via P4 / rte_flow             │
│  ┌────────────────▼─────────────────────┐                     │
│  │          DPU / SmartNIC              │                     │
│  │   Hardware Data Plane                │                     │
│  │  • P4 Match-Action Pipeline          │                     │
│  │  • Line-rate forwarding (200-400G)   │                     │
│  │  • HW ACLs, QoS, Encap/Decap        │                     │
│  └──────────────────────────────────────┘                     │
│                                                               │
│  Result: Software agility + Hardware performance              │
└──────────────────────────────────────────────────────────────┘
```

---

## 5. Relevance to 5G Transport

### 5.1 URLLC + CSR in the 5G Transport Network

```
┌─────────────────────────────────────────────────────────────────┐
│              5G Transport Network Tiers                          │
│                                                                  │
│  Cell Site        Far Edge           Regional DC      Core DC   │
│  ┌──────┐        ┌──────────┐       ┌──────────┐    ┌────────┐ │
│  │ gNB  │        │ MEC +    │       │  vCU +   │    │  5GC   │ │
│  │      │ ──────→│ URLLC UPF│ ─────→│  eMBB UPF│───→│  AMF   │ │
│  │      │  FH    │          │  MH   │          │ BH │  SMF   │ │
│  └──────┘        └──────────┘       └──────────┘    └────────┘ │
│                                                                  │
│  Router Type:    SW CSR            SW CSR or         HW Router  │
│                  (Low cost,         HW Router         (ASR 9000) │
│                  fast deploy)      (depends on                   │
│                                     scale)                       │
│                                                                  │
│  URLLC traffic stays at the Far Edge (< 1ms round-trip)         │
│  eMBB traffic can traverse to Regional/Core DC                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 6. Extended Learning Resources

### Must-Read References
1. **3GPP TR 38.913** — Study on 5G NR scenarios and requirements (URLLC KPIs)
2. **3GPP TS 22.261** — Service requirements for 5G system (reliability targets)
3. **3GPP TS 22.104** — Service requirements for cyber-physical control (industrial URLLC)
4. **Cisco CSR 1000v Data Sheet** — Software router capabilities and performance benchmarks
5. **Nokia SR Linux Architecture Guide** — Cloud-native routing OS design

### Key Concepts to Remember
- URLLC = **1 ms latency + 99.999% reliability** — these are hard targets, not aspirational
- **Mini-slot + Configured Grant** at the RAN saves 3-4 ms of radio handshake
- The UPF budget for URLLC is **≤ 50 μs** — kernel-bypass is mandatory
- **Software CSR** = cloud-native, cost-effective, but CPU-bound (< 40G)
- **Hardware Router** = line-rate, deterministic, but expensive and rigid
- **DPU-accelerated CSR** = the future convergence of both worlds
