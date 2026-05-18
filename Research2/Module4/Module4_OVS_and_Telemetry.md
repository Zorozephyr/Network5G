# Module 4C: Open vSwitch (OVS) & Telemetry (OAM, INT)

## 1. Open vSwitch (OVS) — The Virtual Switch

### 1.1 Why OVS Exists

In virtualized/containerized 5G deployments, VMs and Pods need to communicate. OVS is the **de facto standard virtual switch** that connects virtual network functions (VNFs/CNFs) inside a server.

```
Without OVS (direct NIC sharing — no isolation):
  ┌────────┐ ┌────────┐ ┌────────┐
  │ VM 1   │ │ VM 2   │ │ VM 3   │
  └───┬────┘ └───┬────┘ └───┬────┘
      └──────────┴──────────┘
                 │
           Physical NIC  ← No isolation, no policy, chaos

With OVS (programmable switching):
  ┌────────┐ ┌────────┐ ┌────────┐
  │ VM 1   │ │ VM 2   │ │ VM 3   │
  └───┬────┘ └───┬────┘ └───┬────┘
      │          │          │
  ┌───▼──────────▼──────────▼───┐
  │       Open vSwitch          │
  │  • Flow-based forwarding    │
  │  • VXLAN / GRE tunneling    │
  │  • QoS, ACLs, metering      │
  │  • OpenFlow programmable    │
  └──────────────┬──────────────┘
                 │
           Physical NIC
```

### 1.2 OVS Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    OVS Architecture                               │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                     SDN Controller                          │  │
│  │               (OpenDaylight, ONOS, Ryu)                     │  │
│  └──────────────────────┬──────────────────────────────────────┘  │
│                         │ OpenFlow Protocol                       │
│                         │ (TCP port 6653)                         │
│  ┌──────────────────────▼──────────────────────────────────────┐  │
│  │                   ovs-vswitchd                              │  │
│  │                 (Userspace Daemon)                           │  │
│  │                                                             │  │
│  │  • Implements OpenFlow flow tables (the "slow path")        │  │
│  │  • Handles table-miss packets from datapath                 │  │
│  │  • Installs megaflow cache entries in datapath              │  │
│  │  • Manages ports, tunnels, mirrors                          │  │
│  └────────┬────────────────────────────────┬───────────────────┘  │
│           │ OVSDB Protocol                 │ Netlink              │
│  ┌────────▼───────────────┐       ┌────────▼───────────────────┐  │
│  │     ovsdb-server       │       │       Datapath             │  │
│  │  (Config Database)     │       │    (Kernel Module)         │  │
│  │                        │       │                            │  │
│  │  • Bridge definitions  │       │  • Fast-path forwarding    │  │
│  │  • Port configs        │       │  • Exact Match Cache (EMC) │  │
│  │  • Tunnel params       │       │  • Megaflow Cache (dpcls)  │  │
│  │  • Persistent storage  │       │  • Executes cached actions │  │
│  └────────────────────────┘       └────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 1.3 The Packet Flow — Fast Path vs Slow Path

```
Packet Arrives at OVS:
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  Packet → Datapath (Kernel)                                  │
│              │                                               │
│              ▼                                               │
│  ┌─────────────────────┐                                     │
│  │  Exact Match Cache  │  ← O(1) hash lookup, fastest       │
│  │  (EMC)              │                                     │
│  └──────────┬──────────┘                                     │
│        HIT? │                                                │
│        ┌────┴────┐                                           │
│       YES       NO                                           │
│        │         ▼                                           │
│        │  ┌─────────────────────┐                            │
│        │  │  Megaflow Cache     │  ← Wildcard match (TSS)    │
│        │  │  (dpcls)            │                            │
│        │  └──────────┬──────────┘                            │
│        │        HIT? │                                       │
│        │        ┌────┴────┐                                  │
│        │       YES       NO                                  │
│        │        │         ▼                                   │
│        │        │  ┌──────────────────────┐                   │
│        │        │  │  Upcall to           │  ← Context switch│
│        │        │  │  ovs-vswitchd        │    (SLOW PATH)   │
│        │        │  │                      │                   │
│        │        │  │  OpenFlow pipeline   │                   │
│        │        │  │  (Table 0 → N)       │                   │
│        │        │  │                      │                   │
│        │        │  │  Result: install     │                   │
│        │        │  │  megaflow + EMC entry│                   │
│        │        │  └──────────┬───────────┘                   │
│        │        │             │                               │
│        ▼        ▼             ▼                               │
│  ┌──────────────────────────────────┐                        │
│  │    Execute Actions               │                        │
│  │    (forward, modify, drop, etc.) │                        │
│  └──────────────────────────────────┘                        │
└──────────────────────────────────────────────────────────────┘
```

### 1.4 OVS-DPDK — Kernel Bypass for OVS

For 5G workloads, the kernel datapath is too slow. OVS-DPDK moves the entire datapath to userspace:

```
Standard OVS:                           OVS-DPDK:
┌──────────────┐                        ┌──────────────┐
│ ovs-vswitchd │ (userspace)            │ ovs-vswitchd │ (userspace)
│ (slow path)  │                        │ + DPDK PMDs  │
└──────┬───────┘                        │ (fast path!) │
       │ Netlink                        └──────┬───────┘
┌──────▼───────┐                               │ Direct DMA
│ Kernel       │                        ┌──────▼───────┐
│ Datapath     │ (kernel)               │   NIC (VF)   │ (hardware)
│ (fast path)  │                        └──────────────┘
└──────┬───────┘
       │                                No kernel involvement!
┌──────▼───────┐                        PMD threads poll NIC queues
│   NIC        │ (hardware)             continuously in userspace.
└──────────────┘

Performance:
  Kernel OVS:   ~1-2 Mpps
  OVS-DPDK:     ~10-15 Mpps per core
  HW Offload:   Line-rate (100G+)
```

### 1.5 Hardware Offload — OVS on SmartNICs

The ultimate performance tier: offload the entire OVS datapath to SmartNIC hardware.

```
┌──────────────────────────────────────────────────────────────┐
│            OVS Hardware Offload (e.g., NVIDIA ASAP²)         │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  ovs-vswitchd (Host CPU)                               │  │
│  │  • Handles first packet of flow (slow path)            │  │
│  │  • Compiles flow → tc flower rule → rte_flow           │  │
│  │  • Pushes rule to SmartNIC eSwitch                     │  │
│  └───────────────────────┬────────────────────────────────┘  │
│                          │ tc flower / rte_flow              │
│  ┌───────────────────────▼────────────────────────────────┐  │
│  │  SmartNIC eSwitch (Hardware)                           │  │
│  │                                                        │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  HW Flow Table (TCAM)                            │  │  │
│  │  │  • Matches on L2/L3/L4 + tunnel headers          │  │  │
│  │  │  • Actions: forward, encap/decap, NAT, meter     │  │  │
│  │  │  • ALL subsequent packets → processed HERE       │  │  │
│  │  │  • Host CPU never sees fast-path traffic!        │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │                                                        │  │
│  │  VF0 ←→ VM1    VF1 ←→ VM2    Uplink ←→ Network       │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Performance: Line-rate at 100-400 Gbps                      │
│  CPU savings: 80-90% fewer cores needed                      │
└──────────────────────────────────────────────────────────────┘
```

### 1.6 OVS in 5G — Where It Fits

| 5G Component | OVS Role | Mode |
|---|---|---|
| **vCU/vDU Platform** | Connects CNF pods to fronthaul/midhaul NICs | OVS-DPDK or HW offload |
| **UPF** | Virtual switch between N3, N6, N9 interfaces | OVS-DPDK + SmartNIC |
| **MEC Platform** | Tenant isolation, VXLAN overlays | OVS + OpenFlow |
| **5G Core (CP)** | Connects AMF, SMF, UDM microservices | Standard OVS (low throughput) |

---

## 2. Telemetry — OAM & In-Band Network Telemetry

### 2.1 What is OAM?

**OAM = Operations, Administration, and Maintenance** — the framework for monitoring network health, detecting faults, and measuring performance.

```
OAM Functions:
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  O — Operations:     Day-to-day monitoring, config changes   │
│  A — Administration: User management, provisioning, billing  │
│  M — Maintenance:    Fault detection, diagnostics, repair    │
│                                                              │
│  ┌────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │   Fault    │  │ Performance  │  │  Configuration       │  │
│  │  Mgmt (FM)│  │  Mgmt (PM)   │  │  Mgmt (CM)           │  │
│  │            │  │              │  │                      │  │
│  │ • Alarms  │  │ • Latency    │  │ • Provisioning       │  │
│  │ • Traps   │  │ • Loss       │  │ • SW upgrades        │  │
│  │ • Syslog  │  │ • Jitter     │  │ • Topology changes   │  │
│  └────────────┘  └──────────────┘  └──────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 Legacy Telemetry vs Modern Telemetry

```
Legacy (Pull-Based — SNMP Polling):
┌────────────────────────────────────────────────────────────┐
│                                                            │
│  NMS ── GET request ──→ Router ── Response ──→ NMS         │
│         (every 5 min)           (MIB counters)             │
│                                                            │
│  Problems:                                                 │
│  • 5-minute polling interval = blind to microbursts        │
│  • High CPU overhead on network devices                    │
│  • Scales poorly (thousands of OIDs × thousands of nodes)  │
│  • Text-based encoding (inefficient)                       │
└────────────────────────────────────────────────────────────┘

Modern (Push-Based — Streaming Telemetry):
┌────────────────────────────────────────────────────────────┐
│                                                            │
│  Router ── gRPC stream ──→ Collector ──→ Analytics         │
│  (pushes data every                      (Prometheus,      │
│   100ms-1s automatically)                 Kafka, ELK)      │
│                                                            │
│  Benefits:                                                 │
│  • Sub-second granularity (sees microbursts!)               │
│  • Near-zero CPU overhead (hardware counters)              │
│  • Scales to millions of data points                       │
│  • Binary encoding (GPB/protobuf — compact, fast)          │
└────────────────────────────────────────────────────────────┘
```

### 2.3 Model-Driven Telemetry (MDT) Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│               Model-Driven Telemetry Stack                        │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Collector / Analytics Platform                             │  │
│  │  (Prometheus, InfluxDB, Kafka, Cisco Crosswork)             │  │
│  └──────────────────────┬──────────────────────────────────────┘  │
│                         │ gRPC / gNMI / gNOI                      │
│  ┌──────────────────────▼──────────────────────────────────────┐  │
│  │  Transport Protocol                                         │  │
│  │  • gRPC (Google Remote Procedure Call)                       │  │
│  │  • gNMI (gRPC Network Management Interface)                 │  │
│  │  • Encoding: GPB (Google Protocol Buffers) or JSON          │  │
│  └──────────────────────┬──────────────────────────────────────┘  │
│                         │                                         │
│  ┌──────────────────────▼──────────────────────────────────────┐  │
│  │  Data Model (YANG)                                          │  │
│  │  • Defines the schema of telemetry data                     │  │
│  │  • Vendor-neutral (OpenConfig) or vendor-specific           │  │
│  │  • Example paths:                                           │  │
│  │    /interfaces/interface/state/counters/in-octets            │  │
│  │    /network-instances/network-instance/protocols/bgp/...    │  │
│  └──────────────────────┬──────────────────────────────────────┘  │
│                         │                                         │
│  ┌──────────────────────▼──────────────────────────────────────┐  │
│  │  Network Device (Router / Switch)                           │  │
│  │  • Reads HW counters directly                               │  │
│  │  • Streams data at configured interval                      │  │
│  │  • Subscription types: periodic, on-change, target-defined  │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 2.4 In-Band Network Telemetry (INT)

INT embeds telemetry metadata **directly inside data packets** as they traverse the network, providing per-hop, per-packet visibility.

```
┌──────────────────────────────────────────────────────────────────┐
│              In-Band Network Telemetry (INT)                      │
│                                                                   │
│  Source          Switch A         Switch B         Sink            │
│  (Ingress)      (Transit)        (Transit)        (Egress)        │
│                                                                   │
│  ┌─────┐        ┌─────┐         ┌─────┐          ┌─────┐        │
│  │     │        │     │         │     │          │     │        │
│  │ Add │───────→│ Add │────────→│ Add │─────────→│Strip│        │
│  │ INT │        │ own │         │ own │          │ INT │        │
│  │ hdr │        │meta │         │meta │          │ hdr │        │
│  └─────┘        └─────┘         └─────┘          └──┬──┘        │
│                                                      │           │
│  Packet with INT metadata:                           ▼           │
│  ┌──────┬──────┬──────────┬──────────┬───────┐  ┌────────┐      │
│  │ Orig │ INT  │ Switch A │ Switch B │ Orig  │  │Collect-│      │
│  │ Hdrs │ Hdr  │ Metadata │ Metadata │ Data  │  │  or    │      │
│  └──────┴──────┴──────────┴──────────┴───────┘  └────────┘      │
│                                                                   │
│  Each switch inserts:                                             │
│  • Node ID         (which switch am I?)                           │
│  • Ingress/Egress port                                            │
│  • Timestamp       (nanosecond precision)                         │
│  • Queue depth     (congestion indicator)                         │
│  • Latency         (processing time at this hop)                  │
│  • Link utilization                                               │
└──────────────────────────────────────────────────────────────────┘
```

### 2.5 INT for 5G URLLC Monitoring

```
URLLC Packet Journey with INT:

  UE → gNB → [Transport Network] → UPF → DN
                    │
         ┌──────────┴──────────┐
         │  INT-enabled path   │
         │                     │
    ┌────▼────┐  ┌─────────┐  ┌────▼────┐
    │Router 1 │──│Router 2 │──│Router 3 │
    │         │  │         │  │         │
    │ ts: 10μs│  │ ts: 15μs│  │ ts: 8μs │
    │ q: 2%   │  │ q: 45%  │  │ q: 5%   │
    └─────────┘  └─────────┘  └─────────┘
                      ↑
                 Queue at 45%!
                 INT reveals this hop
                 is the latency bottleneck.

  Without INT: You see end-to-end latency = 33μs
  With INT:    You see EXACTLY which hop added delay.
  Action:      Re-route URLLC via Flex-Algo to avoid Router 2.
```

### 2.6 Telemetry Collection Architecture for 5G

```
┌──────────────────────────────────────────────────────────────────┐
│           5G Network Telemetry Architecture                       │
│                                                                   │
│  Data Sources:                                                    │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                   │
│  │ gNB  │ │  DU  │ │  CU  │ │ UPF  │ │Router│                   │
│  │      │ │      │ │      │ │      │ │      │                   │
│  │ KPIs │ │ KPIs │ │ KPIs │ │ KPIs │ │ INT  │                   │
│  └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘ └──┬───┘                   │
│     │        │        │        │        │                        │
│     └────────┴────────┴────────┴────────┘                        │
│                       │ gRPC / gNMI / Kafka                      │
│              ┌────────▼────────────┐                              │
│              │   Message Bus       │                              │
│              │   (Kafka / NATS)    │                              │
│              └────────┬────────────┘                              │
│         ┌─────────────┼─────────────────┐                        │
│         ▼             ▼                 ▼                        │
│  ┌────────────┐ ┌──────────────┐ ┌──────────────┐               │
│  │ Time-Series│ │  Analytics   │ │  Alerting    │               │
│  │ DB         │ │  Engine      │ │  Engine      │               │
│  │(InfluxDB / │ │(Spark / Flink│ │(PagerDuty /  │               │
│  │ Prometheus)│ │ ML models)   │ │ Grafana)     │               │
│  └────────────┘ └──────────────┘ └──────────────┘               │
│                                                                   │
│  Key Metrics Collected:                                           │
│  • Per-slice throughput, latency, packet loss                    │
│  • Per-UE session state and QoS compliance                       │
│  • Transport hop-by-hop latency (via INT)                        │
│  • UPF fast-path hit ratio, memory pool utilization              │
│  • SmartNIC offload statistics, flow table occupancy             │
└──────────────────────────────────────────────────────────────────┘
```

### 2.7 OAM Protocols Summary

| Protocol | Layer | Purpose | 5G Use Case |
|---|---|---|---|
| **BFD** | L3 | Bidirectional Forwarding Detection (fast failure: ~50ms) | SR-MPLS link failure detection |
| **CFM (802.1ag)** | L2 | Connectivity Fault Management | Fronthaul Ethernet OAM |
| **Y.1731** | L2 | Performance monitoring (delay, loss, jitter) | Fronthaul SLA verification |
| **TWAMP** | L3/L4 | Two-Way Active Measurement Protocol | End-to-end latency measurement |
| **gNMI** | App | Streaming telemetry (get/set/subscribe) | All 5G components |
| **INT** | In-band | Per-packet, per-hop metadata | Transport path monitoring |
| **SNMP** | App | Legacy polling (still used for compatibility) | Legacy NMS integration |

---

## 3. Extended Learning Resources

### Must-Read References
1. **"In-Band Network Telemetry (INT) Specification"** — P4.org (defines the INT header format)
2. **OpenConfig YANG Models** — github.com/openconfig/public (vendor-neutral telemetry schemas)
3. **OVS Documentation** — openvswitch.org (architecture guide, DPDK integration)
4. **NVIDIA ASAP² (Accelerated Switching and Packet Processing)** — OVS HW offload whitepaper

### Key Concepts to Remember
- **OVS** = programmable virtual switch, the SDN workhorse (OpenFlow + OVSDB)
- **OVS-DPDK** = kernel bypass for OVS — 10× performance over kernel datapath
- **OVS HW Offload** = push flow rules to SmartNIC — line-rate, zero CPU
- **OAM** = fault detection + performance monitoring + configuration management
- **Streaming Telemetry** (gNMI/gRPC) replaces SNMP polling — sub-second visibility
- **INT** = per-packet, per-hop metadata — pinpoints exactly where latency occurs
- **For URLLC**: INT + Flex-Algo SR = detect bottleneck → reroute in real-time
