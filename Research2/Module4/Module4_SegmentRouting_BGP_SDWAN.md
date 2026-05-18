# Module 4B: Segment Routing, BGP & SD-WAN Deep Dive

## 1. The 5G Transport Problem

The 5G RAN is disaggregated (Module 3). The RU, DU, CU, and UPF are physically separated. Connecting them requires a **transport network** that can:
- Guarantee sub-millisecond latency for URLLC (fronthaul/midhaul)
- Deliver high throughput for eMBB (backhaul)
- Provide strict isolation for network slicing
- Scale dynamically as cells are added

```
5G Transport Network Segments:

  Cell Site      Aggregation       Edge DC         Regional DC      Core DC
  ┌──────┐       ┌──────┐        ┌──────┐        ┌──────┐        ┌──────┐
  │  RU  │──FH──→│  DU  │──MH──→│  CU  │──BH──→│  UPF │──N6──→│  DN  │
  └──────┘       └──────┘        └──────┘        └──────┘        └──────┘

  FH = Fronthaul  (eCPRI,  < 100μs,  highest priority)
  MH = Midhaul    (F1,     < 1ms,    high priority)
  BH = Backhaul   (N3/N9,  < 10ms,   moderate priority)
  N6 = Data Netw  (IP,     best-effort for most traffic)

  ◄── The TRANSPORT NETWORK (routers, switches, fiber) connects all of these ──►
```

**The core question**: How do you build a transport network that satisfies ALL these diverse requirements simultaneously?

**The answer**: **Segment Routing** (for path programming) + **BGP** (for service overlay and control).

---

## 2. Segment Routing (SR) — The Underlay

### 2.1 The Evolution from Traditional MPLS

Traditional MPLS networks require complex signaling protocols:

```
Traditional MPLS (Complex):
┌───────────────────────────────────────────────────────────────┐
│                                                               │
│  Protocols required:                                          │
│  ├── OSPF / IS-IS     (IGP for topology discovery)            │
│  ├── LDP              (Label distribution — hop by hop)       │
│  ├── RSVP-TE          (Traffic Engineering — explicit paths)  │
│  ├── BGP              (Service overlay — VPNs)                │
│  └── BFD              (Fast failure detection)                │
│                                                               │
│  Every router maintains per-tunnel state.                     │
│  100K tunnels = 100K state entries on EVERY transit router.   │
│  Operational nightmare.                                       │
└───────────────────────────────────────────────────────────────┘

Segment Routing (Simplified):
┌───────────────────────────────────────────────────────────────┐
│                                                               │
│  Protocols required:                                          │
│  ├── OSPF / IS-IS     (IGP with SR extensions — that's it!)  │
│  ├── BGP              (Service overlay)                       │
│  └── BFD              (Fast failure detection)                │
│                                                               │
│  NO LDP. NO RSVP-TE.                                          │
│  Path is encoded in the PACKET HEADER by the source.          │
│  Transit routers maintain ZERO per-tunnel state.              │
│  Massive operational simplification.                          │
└───────────────────────────────────────────────────────────────┘
```

### 2.2 How Segment Routing Works

The source router encodes the entire path as an **ordered list of segments** (instructions) in the packet header. Each segment is a simple instruction like "go to Node X" or "use Link Y".

```
Segment Routing Path Example:

  Source wants to reach D, via B then C (for traffic engineering):

  Source                                           Destination
    A ──────── B ──────── C ──────── D
               │                    │
               └──── E ────── F ────┘

  Traditional MPLS: RSVP-TE must signal state on A, B, C, D
  
  Segment Routing:
  ┌─────────────────────────────────────────────────────────┐
  │  At Source A, packet header is:                         │
  │                                                         │
  │  ┌─────────────────────────┐                            │
  │  │ Segment List: [B, C, D] │  ← Entire path encoded!   │
  │  │ Active Segment: B       │                            │
  │  └─────────────────────────┘                            │
  │                                                         │
  │  At Router B:                                           │
  │  • Pops "B" from list                                   │
  │  • Active segment → C                                   │
  │  • Forwards toward C                                    │
  │                                                         │
  │  At Router C:                                           │
  │  • Pops "C" from list                                   │
  │  • Active segment → D                                   │
  │  • Forwards toward D                                    │
  │                                                         │
  │  At Router D:                                           │
  │  • Final segment reached → deliver to application       │
  └─────────────────────────────────────────────────────────┘
```

### 2.3 Segment Types

| Segment Type | Name | What It Does | Identifier |
|---|---|---|---|
| **Prefix SID** | Node Segment | "Go to this node via shortest path" | Global ID (e.g., 16001) |
| **Adjacency SID** | Link Segment | "Use this specific link" | Local ID (per interface) |
| **Binding SID** | Policy Segment | "Apply this pre-defined policy/path" | Local ID |
| **Flex-Algo SID** | Constraint Segment | "Use path optimized for latency/TE" | Global ID + Algorithm |

### 2.4 SR-MPLS vs SRv6

There are two data plane instantiations of Segment Routing:

```
SR-MPLS (Segment Routing over MPLS):
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Uses existing MPLS label stack as the segment list.         │
│  Each segment = 32-bit MPLS label.                           │
│                                                              │
│  ┌──────┬──────┬──────┬───────────────────┐                  │
│  │Label │Label │Label │    Payload        │                  │
│  │ 16001│ 24003│ 16005│    (IP packet)    │                  │
│  └──┬───┴──┬───┴──┬───┴───────────────────┘                  │
│     │      │      └─── Destination Node SID                  │
│     │      └────────── Adjacency SID (specific link)         │
│     └───────────────── First-hop Node SID                    │
│                                                              │
│  Pros: Works with existing MPLS hardware (brownfield)        │
│  Cons: Limited programmability, label stack depth limits      │
└──────────────────────────────────────────────────────────────┘

SRv6 (Segment Routing over IPv6):
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Uses IPv6 extension headers (SRH) as the segment list.     │
│  Each segment = 128-bit IPv6 address.                        │
│                                                              │
│  ┌──────────┬─────────────────────────────┬──────────────┐   │
│  │ IPv6 Hdr │  Segment Routing Header     │   Payload    │   │
│  │ DA=Seg[0]│  ┌─────────────────────────┐│              │   │
│  │          │  │ Segments Left: 2        ││              │   │
│  │          │  │ Seg[0]: fd00::1:100     ││              │   │
│  │          │  │ Seg[1]: fd00::2:200     ││              │   │
│  │          │  │ Seg[2]: fd00::3:300     ││              │   │
│  │          │  └─────────────────────────┘│              │   │
│  └──────────┴─────────────────────────────┴──────────────┘   │
│                                                              │
│  Pros: Native IPv6, programmable (SRv6 Network Programming) │
│        Cloud-native, no MPLS infrastructure needed           │
│  Cons: Larger header overhead (128-bit vs 32-bit per segment)│
│        Requires IPv6 everywhere                              │
└──────────────────────────────────────────────────────────────┘
```

### 2.5 Flexible Algorithm (Flex-Algo)

Flex-Algo allows multiple "planes" on the same physical topology, each optimized for different constraints:

```
Physical Topology (single network):
    A ─── B ─── C ─── D
    │     │     │     │
    E ─── F ─── G ─── H

Flex-Algo 128 (Minimize Latency — for URLLC):
    A ─── B ─── C ─── D      ← Uses lowest-latency links
                              ← Avoids congested paths

Flex-Algo 129 (Maximize Bandwidth — for eMBB):
    A                 D
    │                 │
    E ─── F ─── G ─── H      ← Uses highest-capacity links

Flex-Algo 130 (Disjoint Path — for Resilience):
    A ─── B                   ← Completely separate from Algo 128
          │     
          F ─── G ─── H ─── D

Result: One physical network, three virtual topologies.
Each 5G slice uses a different Flex-Algo!
```

### 2.6 TI-LFA (Topology Independent Loop-Free Alternate)

When a link or node fails, SR can reroute in **< 50 ms** without any signaling:

```
Before Failure:
    A ──── B ──── C ──── D
           │             │
           E ──── F ──── G

    Traffic: A → B → C → D

Link B-C Fails:
    A ──── B ──╳── C ──── D      ← Link down!
           │             │
           E ──── F ──── G

    B immediately uses pre-computed backup:
    Traffic: A → B → E → F → G → D   (< 50ms switchover)

    No signaling needed — B had the backup path PRE-COMPUTED
    using Segment Routing labels.
```

---

## 3. BGP — The Service Overlay

### 3.1 BGP's Role in 5G Transport

BGP is NOT just for internet routing. In 5G transport, BGP serves as the **universal service layer**:

```
┌──────────────────────────────────────────────────────────┐
│             BGP Functions in 5G Transport                 │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  1. L3VPN / EVPN   (Service Isolation / Slicing)   │  │
│  │     Creates isolated virtual networks per slice     │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  2. BGP-LS   (Topology Export to SDN Controller)   │  │
│  │     Controller sees entire network graph            │  │
│  │     Computes optimal paths for each SLA             │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  3. BGP-CT   (Classful Transport for SLA Mapping)  │  │
│  │     Maps services to specific SR transport classes  │  │
│  │     E.g., URLLC → low-latency tunnel               │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  4. BGP-LU   (Labeled Unicast for Inter-Domain)    │  │
│  │     Stitches SR paths across domain boundaries     │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### 3.2 Network Slicing with SR + BGP

```
┌──────────────────────────────────────────────────────────────────┐
│          Network Slicing: SR Underlay + BGP Overlay              │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐        │
│  │  BGP L3VPN Instance: "URLLC-Slice"                    │        │
│  │  Route-Target: 65000:100                              │        │
│  │  Color: 100 (mapped to SR Flex-Algo 128 = Low-Lat)    │        │
│  │                                                        │        │
│  │  gNB ──→ DU (edge) ──→ URLLC UPF (edge)               │        │
│  │          All traffic stays at the edge!                │        │
│  └──────────────────────────────────────────────────────┘        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────┐        │
│  │  BGP L3VPN Instance: "eMBB-Slice"                     │        │
│  │  Route-Target: 65000:200                              │        │
│  │  Color: 200 (mapped to SR Flex-Algo 129 = High-BW)    │        │
│  │                                                        │        │
│  │  gNB ──→ DU ──→ CU (regional) ──→ eMBB UPF (core)    │        │
│  │          Traffic traverses full transport              │        │
│  └──────────────────────────────────────────────────────┘        │
│                                                                  │
│  Both slices share the SAME physical routers and fiber!          │
│  Isolation is achieved through SR path steering + BGP VRFs.      │
└──────────────────────────────────────────────────────────────────┘
```

### 3.3 BGP-LS and SDN Controller Integration

```
┌──────────────────────────────────────────────────────────────────┐
│                SDN-Controlled 5G Transport                       │
│                                                                  │
│               ┌────────────────────────┐                         │
│               │    SDN Controller       │                         │
│               │  (PCE / Crosswork)      │                         │
│               │                         │                         │
│               │  • Receives topology    │                         │
│               │    via BGP-LS           │                         │
│               │  • Computes TE paths    │                         │
│               │  • Programs SR policies │                         │
│               └───────┬────────────────┘                         │
│                       │ BGP-LS (topology)                        │
│                       │ PCEP / Netconf (policy push)             │
│         ┌─────────────┼─────────────────────────┐                │
│         │             │                         │                │
│    ┌────▼────┐   ┌────▼────┐   ┌────▼────┐   ┌────▼────┐       │
│    │Router A │───│Router B │───│Router C │───│Router D │       │
│    │ (PE)    │   │ (P)     │   │ (P)     │   │ (PE)    │       │
│    │         │   │         │   │         │   │         │       │
│    │ BGP-LS  │   │ SR-MPLS │   │ SR-MPLS │   │ BGP-LS  │       │
│    │ speaker │   │ transit │   │ transit │   │ speaker │       │
│    └─────────┘   └─────────┘   └─────────┘   └─────────┘       │
│                                                                  │
│  PE = Provider Edge (runs BGP + SR)                              │
│  P  = Provider (transit, runs SR only — no BGP state!)           │
└──────────────────────────────────────────────────────────────────┘
```

---

## 4. SD-WAN — Software-Defined Wide Area Network

### 4.1 What is SD-WAN?

SD-WAN is a **software overlay** that decouples the WAN control plane from the physical transport, allowing enterprises to use multiple link types (MPLS, broadband, LTE/5G) through a single intelligent fabric.

```
Traditional WAN (Hub-and-Spoke MPLS):
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Branch 1 ─── MPLS ─── HQ Data Center ─── MPLS ─── Branch 2│
│                              │                               │
│                         Cloud/SaaS ← ALL traffic backhauled  │
│                              ↑                               │
│                         Bottleneck!                           │
│                                                              │
│  Problem: Cloud traffic (Office 365, AWS) must hairpin       │
│  through the DC. Adds latency, wastes MPLS bandwidth.        │
└──────────────────────────────────────────────────────────────┘

SD-WAN (Application-Aware):
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Branch 1 ─┬─ MPLS ──────── HQ Data Center                  │
│            ├─ Broadband ──→ Cloud/SaaS (Direct!)             │
│            └─ 5G/LTE ────→ Backup (Failover)                 │
│                                                              │
│  SD-WAN controller decides:                                  │
│  • ERP traffic → MPLS (low latency, guaranteed)              │
│  • Office 365  → Broadband (direct-to-cloud)                 │
│  • Backup      → 5G (always-on resilience)                   │
└──────────────────────────────────────────────────────────────┘
```

### 4.2 SD-WAN Architecture (Four Planes)

```
┌──────────────────────────────────────────────────────────────────┐
│                    SD-WAN Architecture                            │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │                  MANAGEMENT PLANE                          │   │
│  │                    (vManage)                                │   │
│  │  • Single-pane-of-glass dashboard                          │   │
│  │  • Configuration templates                                 │   │
│  │  • Monitoring, troubleshooting, analytics                  │   │
│  └──────────────────────┬─────────────────────────────────────┘   │
│                         │                                         │
│  ┌──────────────────────▼─────────────────────────────────────┐   │
│  │                   CONTROL PLANE                             │   │
│  │                    (vSmart)                                  │   │
│  │  • Centralized routing intelligence                         │   │
│  │  • OMP (Overlay Management Protocol) — distributes routes   │   │
│  │  • Policy engine — traffic steering, SLA enforcement        │   │
│  │  • Data policy, App-route policy                            │   │
│  └──────────────────────┬─────────────────────────────────────┘   │
│                         │ OMP                                     │
│  ┌──────────────────────▼─────────────────────────────────────┐   │
│  │                   ORCHESTRATION PLANE                       │   │
│  │                    (vBond)                                   │   │
│  │  • Authentication & trust establishment                     │   │
│  │  • NAT traversal (STUN-like)                                │   │
│  │  • Initial device onboarding                                │   │
│  └──────────────────────┬─────────────────────────────────────┘   │
│                         │                                         │
│  ┌──────────────────────▼─────────────────────────────────────┐   │
│  │                     DATA PLANE                              │   │
│  │                   (WAN Edge)                                 │   │
│  │  • Encapsulates traffic in IPsec tunnels                    │   │
│  │  • Enforces QoS and security policies                       │   │
│  │  • BFD probes measure link health (loss, latency, jitter)   │   │
│  │  • Application-aware routing (DPI identifies apps)          │   │
│  └────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

### 4.3 SD-WAN vs MPLS — Head-to-Head

| Feature | Traditional MPLS | SD-WAN |
|---|---|---|
| **Architecture** | Private carrier-managed circuits | Software overlay on any transport |
| **Transport** | Single (MPLS only) | Multiple (MPLS + broadband + 5G) |
| **SLA Guarantee** | Carrier-guaranteed, contractual | Application-aware, measured in real-time |
| **Cloud Access** | Backhauled through DC | Direct-to-cloud breakout |
| **Security** | Private isolation (no encryption) | End-to-end IPsec encryption |
| **Cost** | $$$$ (per-circuit, per-Mbps pricing) | $$ (leverages cheap broadband/5G) |
| **Deployment** | Weeks-months (carrier provisioning) | Hours-days (zero-touch provisioning) |
| **Visibility** | Limited (carrier manages) | Full application-level visibility |
| **Failover** | Slow (convergence: seconds-minutes) | Fast (sub-second with BFD) |
| **Scalability** | Hard to add sites | Easy — spin up new edge device |

### 4.4 SD-WAN and 5G Convergence

```
┌──────────────────────────────────────────────────────────────────┐
│           5G-Integrated SD-WAN Architecture                       │
│                                                                   │
│  Enterprise Branch                                                │
│  ┌──────────────────────────────────────────┐                     │
│  │              SD-WAN Edge                  │                     │
│  │                                           │                     │
│  │  ┌─────────┐ ┌──────────┐ ┌───────────┐  │                     │
│  │  │ MPLS    │ │Broadband │ │ 5G Module │  │                     │
│  │  │ WAN     │ │ Internet │ │ (SA/NSA)  │  │                     │
│  │  └────┬────┘ └────┬─────┘ └────┬──────┘  │                     │
│  │       │           │            │          │                     │
│  │  ┌────▼───────────▼────────────▼───────┐  │                     │
│  │  │     Application-Aware Steering       │  │                     │
│  │  │                                      │  │                     │
│  │  │  ERP → MPLS (guaranteed SLA)         │  │                     │
│  │  │  O365 → Broadband (direct cloud)     │  │                     │
│  │  │  IoT → 5G Slice (URLLC)              │  │                     │
│  │  │  Backup → Any available path          │  │                     │
│  │  └──────────────────────────────────────┘  │                     │
│  └──────────────────────────────────────────┘                     │
│                                                                   │
│  5G Network Slicing + SD-WAN = Per-Application SLA over wireless  │
└──────────────────────────────────────────────────────────────────┘
```

### 4.5 The Full Picture: SR + BGP + SD-WAN in 5G

```
┌──────────────────────────────────────────────────────────────────────┐
│                5G End-to-End Transport Stack                          │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │  APPLICATION / SERVICE LAYER                                    │  │
│  │  SD-WAN policies, 5G Network Slicing, QoS requirements          │  │
│  └─────────────────────────────┬───────────────────────────────────┘  │
│                                │                                      │
│  ┌─────────────────────────────▼───────────────────────────────────┐  │
│  │  OVERLAY LAYER (BGP)                                            │  │
│  │  L3VPN / EVPN instances per slice                               │  │
│  │  BGP-CT maps services to SR "colors" (latency, bandwidth)       │  │
│  └─────────────────────────────┬───────────────────────────────────┘  │
│                                │                                      │
│  ┌─────────────────────────────▼───────────────────────────────────┐  │
│  │  UNDERLAY LAYER (Segment Routing)                               │  │
│  │  SR-MPLS or SRv6 provides programmable paths                    │  │
│  │  Flex-Algo creates virtual topologies per constraint             │  │
│  │  TI-LFA provides < 50ms failover                                │  │
│  └─────────────────────────────┬───────────────────────────────────┘  │
│                                │                                      │
│  ┌─────────────────────────────▼───────────────────────────────────┐  │
│  │  PHYSICAL LAYER                                                 │  │
│  │  Fiber, Ethernet, Routers (HW or SW CSR)                        │  │
│  └─────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 5. Extended Learning Resources

### Must-Read References
1. **IETF RFC 8402** — Segment Routing Architecture (the foundational RFC)
2. **IETF RFC 8986** — SRv6 Network Programming (SRH specification)
3. **Cisco Segment Routing Documentation** — IOS XR configuration guides
4. **"Segment Routing Part I & II"** (Clarence Filsfils, Cisco Press) — The definitive SR book

### Key Concepts to Remember
- **Segment Routing** eliminates LDP and RSVP-TE — source-routed, stateless transit
- **SR-MPLS** = brownfield (existing MPLS HW); **SRv6** = greenfield (native IPv6)
- **Flex-Algo** = multiple virtual topologies on one physical network (key for slicing!)
- **TI-LFA** = sub-50ms failover without signaling
- **BGP-LS** exports topology to SDN controller for intelligent path computation
- **BGP-CT** maps services to transport "colors" (latency class, bandwidth class)
- **SD-WAN** decouples control from transport — uses any WAN link intelligently
- **5G + SD-WAN** = per-application steering across MPLS, broadband, and 5G slices
