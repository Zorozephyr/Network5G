# Protocol Map — Where Everything Lives

> "I heard about protocol X — where does it fit?" Look it up here.
> Organized by network domain. Each entry: what it is, what layer, what it connects to, and why it exists.

---

## Domain 1: Physical Layer (Layer 1) — How Bits Move

| Technology | What It Is | Where It's Used | Speed |
|-----------|-----------|----------------|-------|
| **Copper (Cat 5e/6/6a)** | Twisted pair cable, electrical signals | LAN, office, last-mile | 1–10 Gbps, <100m |
| **Fiber optic (SMF)** | Single-mode glass fiber, laser pulses | Backbone, long-haul, fronthaul | 100G–400G, hundreds of km |
| **Fiber optic (MMF)** | Multi-mode glass fiber, LED pulses | Data center interconnects | 10G–100G, <2km |
| **DWDM** | Multiple wavelengths on one fiber | Backbone/long-haul | 100+ channels × 400G = 40+ Tbps per fiber |
| **5G NR Radio (FR1)** | Sub-6 GHz radio (e.g., n78 at 3.5 GHz) | Macro cells, wide coverage | Up to ~1 Gbps per UE |
| **5G NR Radio (FR2)** | mmWave (24-52 GHz) | Dense urban, stadiums, hotspots | Up to ~4 Gbps per UE, very short range |
| **eCPRI** | Enhanced Common Public Radio Interface | O-RU ↔ O-DU fronthaul | 25G Ethernet per cell sector |
| **CPRI** | Legacy Common Public Radio Interface | Traditional RU ↔ BBU | Constant bitrate, very high BW |
| **Wi-Fi radio** | 802.11 (2.4/5/6 GHz, unlicensed) | Indoor, home, enterprise WLAN | Up to 46 Gbps (Wi-Fi 7) |
| **Satellite** | LEO (500-2000km) / GEO (36,000km) radio | Rural, maritime, aviation | LEO: 100-300 Mbps, GEO: 20-100 Mbps |

### Key Physical Layer Concepts

**Modulation:** How bits are encoded onto a signal. Higher-order modulation = more bits per symbol = higher throughput but needs cleaner signal.
- BPSK: 1 bit/symbol (most robust)
- QPSK: 2 bits/symbol
- 16QAM: 4 bits/symbol
- 64QAM: 6 bits/symbol
- 256QAM: 8 bits/symbol (max in 5G, needs excellent SNR)

**Subcarrier Spacing (SCS):** In 5G NR, OFDMA divides the spectrum into subcarriers. The spacing between them is configurable:
- 15 kHz: 1ms slot (used for eMBB, matches LTE)
- 30 kHz: 0.5ms slot
- 60 kHz: 0.25ms slot
- 120 kHz: 0.125ms slot (used for FR2 mmWave, URLLC)

Wider SCS = shorter slot = lower latency (good for URLLC) but less spectrum efficiency.

**PRB (Physical Resource Block):** The minimum allocatable radio resource = 12 subcarriers × 1 slot. The MAC scheduler assigns PRBs to UEs each slot.

---

## Domain 2: Data Link Layer (Layer 2) — Hop-to-Hop Delivery

| Protocol | What It Is | Where It's Used |
|----------|-----------|----------------|
| **Ethernet (IEEE 802.3)** | Framing standard for wired LANs. Carries MAC addresses (src/dst, 48 bits each), EtherType (identifies payload protocol), FCS (checksum). | Everywhere — LAN, data center, fronthaul, backhaul |
| **MAC Address** | 48-bit hardware ID (AA:BB:CC:DD:EE:FF) burned into NIC. Used for local hop delivery. Changes at every router hop. | Every Ethernet device |
| **ARP** | Maps IPv4 address → MAC address on local segment. Broadcasts "Who has 10.0.0.1?" | Every IPv4 LAN |
| **NDP** | IPv6 version of ARP. Uses ICMPv6 Neighbor Solicitation. | Every IPv6 network |
| **VLAN (802.1Q)** | 12-bit tag in Ethernet header creating logical network segments. Max 4096 VLANs. | Switches, enterprise, 5G transport |
| **LLDP** | Link Layer Discovery Protocol. Devices announce their identity/capabilities to neighbors. | Switches, managed networks |
| **MACsec (802.1AE)** | Layer 2 encryption. Encrypts Ethernet frames hop-by-hop. | Secure fronthaul/backhaul, data center |
| **Wi-Fi (802.11)** | Wireless Layer 2 protocol. Uses CSMA/CA (collision avoidance, not detection). Frames carry BSSID, station addresses. | WLAN |
| **PPP** | Point-to-Point Protocol. Legacy Layer 2 for serial/DSL links. | ISP last-mile (legacy) |

### Ethernet Frame Structure
```
[Preamble 8B] [Dst MAC 6B] [Src MAC 6B] [802.1Q Tag 4B] [EtherType 2B] [Payload 46-1500B] [FCS 4B]
```
- EtherType 0x0800 = IPv4
- EtherType 0x86DD = IPv6
- EtherType 0x8100 = VLAN-tagged

### Switching vs Routing
- **Switch (Layer 2):** Forwards frames based on MAC addresses. Learns which MAC is on which port. Operates within one broadcast domain.
- **Router (Layer 3):** Forwards packets based on IP addresses. Connects different broadcast domains/subnets. Decrements TTL, rewrites MAC headers.

---

## Domain 3: Network Layer (Layer 3) — Routing Across Networks

| Protocol | What It Is | Use Case |
|----------|-----------|----------|
| **IPv4** | 32-bit addresses (4.3B total). Header: 20 bytes min. Fields: src/dst IP, TTL, protocol, DSCP, flags, fragmentation. | Still dominant globally |
| **IPv6** | 128-bit addresses (effectively infinite). Simpler header (40 bytes, no fragmentation at routers). Flow label for QoS. | Growing — mandatory for SRv6 |
| **ICMP** | Internet Control Message Protocol. Carries error messages (Destination Unreachable, TTL Exceeded) and diagnostics (ping, traceroute). | Troubleshooting |
| **OSPF** | Interior Gateway Protocol. Link-state routing. All routers in an area share link info, compute shortest paths (Dijkstra). Fast convergence (<1s). | Inside enterprise/operator networks |
| **IS-IS** | Interior Gateway Protocol. Similar to OSPF but protocol-agnostic (not tied to IP). Preferred by large ISPs/telcos. | Telecom backbone |
| **BGP** | Exterior Gateway Protocol. Path-vector routing between Autonomous Systems. Policy-based (not shortest path). Slow convergence (minutes). | Internet inter-domain routing |
| **ECMP** | Equal-Cost Multi-Path. When multiple routes have equal cost, distribute traffic across them. | Data center, backbone load balancing |
| **VRRP/HSRP** | Virtual Router Redundancy Protocol. Multiple routers share a virtual IP — if the primary fails, the backup takes over. | Gateway redundancy |

### IP Addressing Deep Dive

**Private IP ranges (RFC 1918):**
- 10.0.0.0/8 (16M addresses) — used in enterprise, 5G core
- 172.16.0.0/12 (1M addresses)
- 192.168.0.0/16 (65K addresses) — used in homes

**Subnetting:** A /24 means 256 addresses (e.g., 192.168.1.0/24 = 192.168.1.0 to 192.168.1.255). The mask tells you which bits are the network part vs. host part.

**CIDR (Classless Inter-Domain Routing):** Replaced the old Class A/B/C system. Allows any prefix length (/8 to /32), enabling efficient address allocation.

**DSCP (Differentiated Services Code Point):** 6 bits in the IP header for QoS marking. Routers use DSCP to prioritize traffic:
- EF (Expedited Forwarding, 0x2E): Lowest latency (voice, URLLC)
- AF (Assured Forwarding): Multiple levels of guaranteed delivery
- BE (Best Effort, 0x00): Default, no guarantees

---

## Domain 4: Transport Layer (Layer 4) — End-to-End Delivery

| Protocol | What It Is | Key Properties | Use Case |
|----------|-----------|---------------|----------|
| **TCP** | Transmission Control Protocol. Connection-oriented. Reliable, ordered delivery. Flow control (window), congestion control (CUBIC, BBR). | 3-way handshake, retransmission, head-of-line blocking | HTTP, HTTPS, SSH, email |
| **UDP** | User Datagram Protocol. Connectionless. No reliability, no ordering. Just src/dst port + checksum. 8-byte header. | Minimal overhead, fire-and-forget | DNS, VoIP, gaming, GTP-U, QUIC |
| **SCTP** | Stream Control Transmission Protocol. Message-oriented (unlike TCP's byte stream). Multi-homing (multiple IP addresses). Multi-streaming (avoids HOL blocking). | Designed for signaling, resilient | 5G: E2 (RIC↔gNB), S1AP (4G), NGAP (5G AMF↔gNB) |
| **QUIC** | Google's replacement for TCP+TLS. Built on UDP. Encrypted by default (TLS 1.3 built in). 0-RTT connection setup. Multiplexed streams without HOL blocking. | Faster than TCP for web | HTTP/3, modern web apps |

### Why 5G Uses Different Transport Protocols for Different Things
- **User plane (GTP-U):** Uses **UDP** — speed matters, reliability handled by TCP above
- **Control plane (E2, NGAP):** Uses **SCTP** — needs reliable, ordered, multi-stream delivery for signaling
- **SBI (5G Core NFs):** Uses **TCP** (HTTP/2) — REST APIs between NFs
- **Management (O1):** Uses **TCP** (NETCONF/RESTCONF over SSH/HTTPS)

---

## Domain 5: Tunneling — Packets Inside Packets

| Protocol | What It Encapsulates | Header Size | Use Case |
|----------|---------------------|-------------|----------|
| **GTP-U** | IP packets in UDP, identified by TEID | 8-12 bytes | 5G user plane (gNB↔UPF) |
| **IPsec ESP** | IP packets, encrypted + authenticated | 20-40 bytes (with IV, padding) | VPN, secure backhaul |
| **GRE** | Any L3 protocol in IP | 4-8 bytes | Simple tunnels, legacy VPN |
| **VXLAN** | Ethernet frames in UDP | 16 bytes | Data center overlay networks |
| **MPLS** | IP packets with label stack | 4 bytes per label | Telco backbone, L3VPN |
| **SRv6** | IPv6 with Segment Routing Header | 8 + 16 bytes per segment | Modern telco transport, network programming |
| **WireGuard** | IP packets, encrypted (ChaCha20) | 32 bytes | Modern VPN (replacing IPsec for simplicity) |

### Tunneling Visual
```
Original packet:  [IP: src=A, dst=B] [TCP] [Data]

After GTP-U:      [Outer IP: src=gNB, dst=UPF] [UDP:2152] [GTP-U: TEID=X] [IP: A→B] [TCP] [Data]
After IPsec:      [Outer IP: src=VPN_GW1, dst=VPN_GW2] [ESP: encrypted([IP: A→B] [TCP] [Data])]
After VXLAN:      [Outer IP: src=VTEP1, dst=VTEP2] [UDP:4789] [VXLAN: VNI=1000] [Eth] [IP: A→B] [TCP] [Data]
After SRv6:       [IPv6: src=ingress, dst=SID1] [SRH: SID_list] [IP: A→B] [TCP] [Data]
```

---

## Domain 6: Security Protocols

| Protocol | Layer | What It Does | Where Used |
|----------|-------|-------------|------------|
| **TLS 1.3** | L5-6 | Encrypts application data. Certificate-based authentication. 1-RTT handshake. | HTTPS (port 443), SBI |
| **IPsec (IKEv2 + ESP)** | L3 | Encrypts IP packets. IKE negotiates keys, ESP encrypts/authenticates. Transport or tunnel mode. | VPN, 5G backhaul encryption |
| **MACsec (802.1AE)** | L2 | Encrypts Ethernet frames hop-by-hop. Uses AES-GCM. | Secure fronthaul, DC interconnect |
| **SSH** | L5 | Encrypted remote shell + file transfer. Key-based or password auth. | Server management, NETCONF transport |
| **802.1X** | L2 | Port-based network access control. Device must authenticate (via RADIUS) before getting network access. | Enterprise wired/wireless LAN |
| **RADIUS/Diameter** | L7 | Authentication, Authorization, Accounting (AAA). RADIUS for Wi-Fi/VPN, Diameter for 4G/5G. | ISP, enterprise, mobile core |
| **5G NAS Security** | RAN | UE↔AMF: encryption (128-bit NEA) + integrity protection (NIA). Prevents IMSI catching. | Every 5G UE connection |
| **PDCP Security** | RAN | UE↔gNB: ciphering + integrity for user/control plane over radio. | Every 5G radio bearer |

### Encryption Layers in a 5G Packet
A packet can be encrypted at **multiple layers simultaneously:**
1. **TLS** (application layer): protects HTTP content end-to-end (phone ↔ server)
2. **PDCP ciphering** (radio layer): protects over the air (phone ↔ gNB)
3. **IPsec** (network layer): protects backhaul (gNB ↔ UPF)
4. **MACsec** (link layer): protects individual fiber links

---

## Domain 7: Orchestration & Management

| Technology | What It Does | Where Used |
|-----------|-------------|------------|
| **Kubernetes (K8s)** | Container orchestration. Deploys, scales, heals containerized applications. | 5G Core, Near-RT RIC, edge computing |
| **Helm** | K8s package manager. Charts define how to deploy an application. | xApp packaging for Near-RT RIC |
| **Docker/OCI** | Container runtime + image format. Packages app + dependencies. | Every containerized NF |
| **SDN Controller** | Centralized network control. Programs forwarding rules on switches. | Data center networking, some telco |
| **OpenFlow** | SDN protocol between controller and switches. Mostly dead. | Historical, replaced by vendor APIs |
| **NETCONF/YANG** | Network configuration protocol. YANG defines data models, NETCONF is the transport (over SSH). | O1 interface, router/switch config |
| **RESTCONF** | RESTful version of NETCONF (HTTP+JSON instead of SSH+XML). | Modern O1, 5G Core NF config |
| **Ansible/Terraform** | Infrastructure automation. Ansible = config management, Terraform = infrastructure provisioning. | Deploying 5G testbeds, cloud infra |
| **Prometheus + Grafana** | Monitoring stack. Prometheus scrapes metrics, Grafana visualizes dashboards. | RIC observability, 5G Core monitoring |

---

## Domain 8: Enterprise & WAN Technologies

| Technology | What It Does | Scale |
|-----------|-------------|-------|
| **LAN** | Local Area Network. Ethernet switches connecting devices in one building/floor. | 10–1000 devices, <1km |
| **WLAN** | Wireless LAN. Wi-Fi access points providing wireless access. Managed by WLAN controller. | Same as LAN, wireless |
| **WAN** | Wide Area Network. Connects geographically dispersed sites (branches, data centers). | City-to-city, country-wide |
| **SD-WAN** | Software-Defined WAN. Overlays intelligent routing on cheap internet links. Replaces MPLS for enterprises. | Enterprise multi-site |
| **SASE** | Secure Access Service Edge. Combines SD-WAN + cloud security (firewall, ZTNA, CASB) in one cloud-delivered service. | Modern enterprise security |
| **ZTNA** | Zero Trust Network Access. Replaces VPN. Authenticates user+device per-application, not per-network. | Remote work, cloud apps |
| **CBRS** | Citizens Broadband Radio Service. Shared 3.5 GHz spectrum in the US. Enables private LTE/5G without buying licensed spectrum. | Private 5G in US |
| **Private 5G** | Dedicated 5G network for an enterprise (factory, hospital, campus). Local gNB + local UPF + local core or hosted core. | Enterprise, industrial IoT |
| **MEC** | Multi-access Edge Computing. Deploy compute (UPF, applications) close to the RAN for low latency. | Edge sites, factory floor |

---

## Domain 9: O-RAN Specific Interfaces & Components

| Component/Interface | Connects | Direction | Protocol | Purpose |
|---|---|---|---|---|
| **E2** | Near-RT RIC ↔ gNB (O-CU/O-DU) | Bidirectional | SCTP + ASN.1 | RAN telemetry (KPM) + control (RC) |
| **A1** | Non-RT RIC → Near-RT RIC | Southbound | HTTP/REST | Policy delivery to xApps |
| **O1** | SMO ↔ All O-RAN components | Management | NETCONF/RESTCONF | Config, fault, PM |
| **O2** | SMO ↔ O-Cloud (K8s infra) | Management | REST API | Cloud infra lifecycle |
| **E3** | dApps ↔ O-DU (emerging) | Bidirectional | TBD | Sub-ms MAC/PHY control |
| **R1** | rApps ↔ Non-RT RIC framework | Internal | REST API | rApp lifecycle, data access |
| **F1** | O-DU ↔ O-CU | Bidirectional | GTP-U/SCTP/IP | Midhaul data + control |
| **Open Fronthaul** | O-RU ↔ O-DU | Bidirectional | eCPRI/Ethernet | I/Q sample transport (7.2x split) |
| **Xn** | gNB ↔ gNB | Bidirectional | SCTP/IP | Inter-gNB handover, load sharing |
| **N2** | gNB ↔ AMF | Control | SCTP (NGAP) | UE registration, handover, paging |
| **N3** | gNB ↔ UPF | User data | GTP-U/UDP/IP | User plane tunnel (TEID) |
| **N4** | SMF ↔ UPF | Control | PFCP/UDP | Session rules (PDR, FAR, QER) |
| **N6** | UPF ↔ Data Network | User data | IP | Toward internet/enterprise |
| **N9** | UPF ↔ UPF | User data | GTP-U/UDP/IP | Inter-UPF tunnel (UL-CL) |

---

## Quick Lookup: "Where Does X Fit?"

| You're Wondering About... | It's At... | Layer | Used For... |
|---|---|---|---|
| BGP | Operator edge routers ↔ internet | L3 | Inter-domain routing |
| OSPF | Inside the operator's network | L3 | Intra-domain routing |
| IPsec | Backhaul (gNB↔UPF) or VPN | L3 | Encryption |
| MPLS | Operator backbone (being replaced) | L2.5 | Fast label-based forwarding |
| SRv6 | Operator backbone (replacing MPLS) | L3 | Segment-based traffic engineering |
| Wi-Fi | UE access (alternative to 5G) | L1-L2 | Wireless LAN |
| NAT | UPF (N6 interface) or enterprise firewall | L3 | Address translation |
| Firewall | Enterprise edge, between UPF and DN | L3-L7 | Access control, inspection |
| VPN | Enterprise WAN, remote access | L3 | Secure tunnels |
| SD-WAN | Enterprise WAN (replacing MPLS) | L3-L4 | Smart WAN routing |
| TEID | Inside GTP-U header (N3 tunnel) | L3 tunnel | PDU session identifier |
| IMEI | Stored in UE hardware, sent to AMF | Identity | Device identification |
| DNS | UE resolver, or enterprise DNS server | L7 | Name → IP resolution |
| Satellite | Alternative backhaul or direct UE access | L1 | Rural/maritime coverage |
| eCPRI | O-RU ↔ O-DU fiber link | L1-L2 | Fronthaul I/Q transport |
| SCTP | E2 (RIC↔gNB), N2 (gNB↔AMF) | L4 | Signaling transport |
| DPDK | Inside UPF, inside E2T | User-space | Kernel-bypass packet processing |
| eBPF/XDP | Inside UPF (fast path) | Kernel | Programmable packet filtering |
| Kubernetes | Hosting 5G Core + Near-RT RIC | Orchestration | Container lifecycle |

---

*Next: Document 4 — 5G Production (real hardware, real vendors, how 5G actually runs in the real world)*
