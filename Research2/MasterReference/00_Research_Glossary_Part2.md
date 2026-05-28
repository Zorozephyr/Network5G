# Research Glossary — Part 2: Networking, 5G Core, Data Plane & Security

---

## Section 4: Networking Fundamentals

### OSI Model — The 7-Layer Framework

The entire networking world is organized into layers. Everything you encounter maps to one of these:

| Layer | Name | What Lives Here | Key Protocols |
|-------|------|----------------|---------------|
| 7 | Application | HTTP, DNS, SIP | What the user sees |
| 6 | Presentation | TLS/SSL, ASN.1 encoding | Data formatting |
| 5 | Session | SCTP sessions, RPC | Connection management |
| 4 | Transport | TCP, UDP, SCTP, QUIC | End-to-end delivery |
| 3 | Network | IP, ICMP, OSPF, BGP | Routing across networks |
| 2 | Data Link | Ethernet, MAC, VLAN, Wi-Fi | Hop-to-hop delivery |
| 1 | Physical | Fiber, copper, radio waves | Raw bits on wire/air |

**Why it matters:** When your paper says "E2 uses SCTP" — that's Layer 4. When you say "eBPF/XDP intercepts at the NIC driver" — that's Layer 2. When you say "GTP-U tunneling" — that's a Layer 3 tunnel. Understanding layers tells you *where* in the stack something operates.

---

### IP Address (Internet Protocol Address)

**What it is:** The unique numerical identifier assigned to every device on an IP network. IPv4 = 32 bits (e.g., 192.168.1.1), only ~4.3 billion addresses (exhausted). IPv6 = 128 bits (e.g., 2001:0db8::1), practically infinite.

**How it works in 5G:** The UE gets an IP address from the SMF (Session Management Function) when a PDU session is established. The UPF is responsible for routing packets based on this IP. In private 5G, the enterprise controls the IP address pool.

---

### NAT (Network Address Translation)

**What it is:** A technique that maps private IP addresses (10.x.x.x, 192.168.x.x) to a public IP address. Your home router does this — 50 devices share one public IP. The router tracks which internal device made which request using port numbers.

**Why it exists:** IPv4 ran out of addresses. NAT lets millions of devices hide behind one public IP.

**Where it appears in 5G:** The UPF can perform NAT for UE traffic heading to the internet. In private 5G, NAT is often at the enterprise firewall boundary.

---

### MAC Address (Media Access Control Address)

**What it is:** A 48-bit hardware identifier burned into every network interface (NIC, Wi-Fi card). Format: `AA:BB:CC:DD:EE:FF`. Unlike IP addresses (which change), MAC addresses are (mostly) permanent and unique globally.

**Where it operates:** Layer 2 only. MAC addresses are used for communication within the same local network (LAN segment). Once a packet crosses a router, the MAC address changes (router's MAC replaces the source), but the IP address stays the same.

**In 5G:** The UE's radio interface doesn't use Ethernet MAC in the traditional sense — it uses C-RNTI (Cell Radio Network Temporary Identifier) for Layer 2 identification over the air interface.

---

### ARP (Address Resolution Protocol)

**What it is:** The protocol that maps IP addresses to MAC addresses on a local network. When your laptop wants to send a packet to 192.168.1.1, it broadcasts "Who has 192.168.1.1?" and the router replies with its MAC address.

---

### VLAN (Virtual Local Area Network)

**What it is:** A way to logically segment one physical switch into multiple isolated networks. Devices in VLAN 10 can't talk to devices in VLAN 20 without a router, even though they're plugged into the same physical switch. Uses 802.1Q tags in the Ethernet header.

**In 5G:** VLANs are used to isolate fronthaul, midhaul, and backhaul traffic on shared infrastructure. The O-RU, O-DU, and O-CU may share physical switches but operate in different VLANs.

---

### Routing Protocols

**OSPF (Open Shortest Path First):** Interior routing protocol. Used *within* a single organization's network (an Autonomous System). Every router builds a complete map of the network and computes shortest paths using Dijkstra's algorithm. Fast convergence, used inside data centers and enterprise networks.

**BGP (Border Gateway Protocol):** Exterior routing protocol. Used *between* organizations (between Autonomous Systems). BGP is the glue that holds the internet together — it's how AT&T's network knows how to reach Google's network. BGP operates on policy (not just shortest path) — "I prefer to route through this provider because it's cheaper." Very slow convergence (minutes).

**IS-IS (Intermediate System to Intermediate System):** Similar to OSPF but used heavily by large ISPs and telecom operators for their backbone networks. Preferred in telecom because it's protocol-agnostic and scales better for massive networks.

**Where they appear in 5G:** BGP connects the telco's network to the internet and to peering partners. OSPF/IS-IS routes traffic within the telco's internal network (between data centers, cell sites, UPFs). Segment Routing (SRv6) is increasingly replacing MPLS for transport within the telco network.

---

### Tunneling Protocols

**GTP-U (GPRS Tunneling Protocol - User Plane):**
The critical tunnel in mobile networks. When your phone sends a packet, it gets wrapped in a GTP-U header containing a **TEID (Tunnel Endpoint Identifier)** — a 32-bit ID that uniquely identifies the user session's tunnel. The packet travels: UE → gNB → (GTP-U tunnel) → UPF → internet. The UPF strips the GTP-U header and forwards the inner packet.

**Why TEID matters to your research:** In your UPF paper, the eBPF/XDP fast path reads the TEID from the GTP-U header to look up routing rules in a kernel map. In your RIC paper, E2 Subscription IDs are conceptually similar — they route telemetry to the correct xApp.

**IPsec (Internet Protocol Security):**
Encrypts IP packets for secure communication over untrusted networks. Has two modes: Transport (encrypt payload only) and Tunnel (encrypt entire packet, wrap in new IP header). Used for VPNs, site-to-site connections, and 5G backhaul encryption.

**GRE (Generic Routing Encapsulation):**
Simple tunneling — wraps one packet inside another. No encryption. Used to create point-to-point links between routers.

**VXLAN (Virtual Extensible LAN):**
Tunneling for data centers. Encapsulates Layer 2 Ethernet frames in UDP packets, allowing you to create massive virtual networks (16 million VNIs vs. 4096 VLANs) across data center fabrics. Used in NFV/cloud deployments.

---

### MPLS (Multiprotocol Label Switching)

**What it is:** A transport mechanism that switches packets based on short **labels** rather than long IP addresses. Routers at the network edge apply a label; interior routers just read the label and forward — much faster than IP routing. MPLS creates predefined paths (Label Switched Paths) through the network.

**Why it's being replaced:** MPLS requires per-path state on every router. Segment Routing (SR-MPLS and SRv6) encodes the path in the packet header itself, eliminating per-router state. SD-WAN is replacing MPLS for enterprise WAN connectivity because it's cheaper (runs over internet links).

**Your files:** [Module4_SegmentRouting_BGP_SDWAN.md](file:///home/user/Coding/Network5GPractice/Research2/Module4/Module4_SegmentRouting_BGP_SDWAN.md)

---

### Segment Routing (SR-MPLS / SRv6)

**What it is:** The modern replacement for MPLS. Instead of routers maintaining state for every path, the source router encodes the entire path as a list of "segments" (node/link identifiers) in the packet header. Each intermediate router just reads the next segment and forwards.

- **SR-MPLS:** Uses MPLS labels as segments. Backward-compatible with existing MPLS hardware.
- **SRv6:** Uses IPv6 addresses as segments. More flexible, native IPv6, enables network programming (SRv6 Network Programming).

**Where it appears in 5G:** Telecom operators use SRv6 in their transport network (backhaul/midhaul) to create deterministic paths with guaranteed latency for URLLC slices. Instead of "best effort" IP routing, SRv6 ensures the packet follows a specific path through the network.

---

## Section 5: 5G Core Network Functions

### The 5G Core — Service-Based Architecture

The 5G Core is not one monolithic box. It's a set of **Network Functions (NFs)** that communicate via HTTP/2 REST APIs (the Service-Based Interface — SBI). This is a fundamental shift from 4G, where core functions communicated via point-to-point protocols.

```
UE → gNB → (N2: AMF) → (N11: SMF) → (N4: UPF) → Internet
                ↕              ↕
              AUSF           NSSF
              UDM            NRF
               PCF            NEF
```

---

### AMF (Access and Mobility Management Function)

**What it does:** Manages UE registration, authentication, mobility (handovers), and connection management. When you turn on your phone, the AMF is the first NF it talks to. It assigns the GUTI (temporary identity), manages paging (waking up sleeping UEs), and coordinates handovers between gNBs.

**Interface:** N2 (AMF ↔ gNB), N1 (AMF ↔ UE via NAS protocol)

---

### SMF (Session Management Function)

**What it does:** Manages PDU (Protocol Data Unit) sessions — the data pipe between UE and the internet. SMF selects which UPF to use, assigns IP addresses to UEs, configures QoS rules, and establishes/modifies/releases GTP-U tunnels.

**Interface:** N4 (SMF ↔ UPF via PFCP protocol), N11 (SMF ↔ AMF)

**Why it matters to your UPF paper:** SMF tells the UPF what to do via PFCP rules (Packet Detection Rules, Forwarding Action Rules). Your eBPF/XDP fast path in the UPF implements these rules in kernel maps.

---

### UPF (User Plane Function)

**What it does:** The data plane workhorse. All user traffic (YouTube, web browsing, IoT data) flows through the UPF. It performs:
- GTP-U tunnel termination (strip/add GTP headers using TEIDs)
- Packet filtering and inspection
- QoS enforcement (DSCP marking, rate limiting)
- Traffic routing (local breakout vs. central routing)
- Usage reporting (for billing)

**Your first research track** was redesigning the UPF data plane with eBPF/XDP fast path + Wasm exception path for hitless DPI plugin injection.

---

### NSSF (Network Slice Selection Function)

**What it does:** Selects the appropriate network slice for a UE's session. When a UE connects and requests a specific slice (identified by S-NSSAI), the NSSF determines which AMF, SMF, and UPF instances serve that slice.

---

### NRF (Network Repository Function)

**What it does:** Service discovery. It's the "phone book" of the 5G Core. Every NF registers itself with the NRF ("I'm SMF-3, I serve slice X, I'm at IP Y"). When an AMF needs to find an SMF, it queries the NRF.

---

### PCF (Policy Control Function)

**What it does:** Stores and delivers policies — QoS rules, charging rules, access control decisions. When SMF sets up a session, it queries PCF for the policy to apply (e.g., "enterprise users on slice 2 get guaranteed 50 Mbps").

---

### AUSF / UDM (Authentication & Data Management)

**AUSF:** Handles authentication. When UE tries to register, AMF asks AUSF to verify the UE's credentials.

**UDM:** Stores subscriber data — subscription profiles, slice access rights, authentication credentials. It's the database behind AUSF.

---

### NEF (Network Exposure Function)

**What it does:** The API gateway for external applications. If a factory wants to know the location of its IoT devices, it queries the NEF via a REST API. NEF controls what external apps can see and do, preventing direct access to internal NFs.

---

## Section 6: 5G Identifiers

| ID | Full Name | Purpose |
|----|-----------|---------|
| **IMSI** | International Mobile Subscriber Identity | Permanent subscriber ID stored on SIM (15 digits). Uniquely identifies a subscription worldwide. Never sent in clear over 5G (privacy improvement over 4G). |
| **SUPI** | Subscription Permanent Identifier | 5G version of IMSI. Same concept, new name. Contains IMSI or NAI. |
| **SUCI** | Subscription Concealed Identifier | Encrypted version of SUPI. Sent over the air instead of SUPI to prevent IMSI catchers. |
| **GUTI** | Globally Unique Temporary Identifier | Temporary ID assigned by AMF to the UE after registration. Used instead of SUPI for all subsequent communication. Changes periodically. |
| **IMEI** | International Mobile Equipment Identity | 15-digit identifier for the physical device (not the SIM). Used to blacklist stolen phones. |
| **TEID** | Tunnel Endpoint Identifier | 32-bit ID in GTP-U headers identifying a specific tunnel/session. The UPF uses TEIDs to match packets to PDU sessions. |
| **S-NSSAI** | Single Network Slice Selection Assistance Info | Identifies a network slice. Contains SST (Slice/Service Type: 1=eMBB, 2=URLLC, 3=mMTC) + optional SD (Slice Differentiator). |
| **C-RNTI** | Cell Radio Network Temporary Identifier | Layer 2 identity assigned by gNB to UE within a cell. Like a MAC address for the radio. Changes when UE moves to a new cell. |

---

## Section 7: Data Plane & Packet Processing

### XDP (eXpress Data Path)

**What it is:** The earliest hook point in the Linux networking stack. Attached to the NIC driver, XDP processes packets *before* the kernel allocates an `sk_buff` (socket buffer). Actions: `XDP_PASS` (continue to kernel), `XDP_DROP` (discard), `XDP_TX` (bounce back out same NIC), `XDP_REDIRECT` (send to another interface/CPU).

**Your UPF paper:** eBPF/XDP is your fast-path — it intercepts GTP-U packets, reads TEIDs, and forwards 95% of traffic without touching the kernel stack. The remaining 5% (needing DPI) gets redirected to the Wasm engine via ring buffer.

---

### eBPF (extended Berkeley Packet Filter)

**What it is:** A virtual machine inside the Linux kernel that runs sandboxed programs. Originally for packet filtering, now used for tracing, security, networking, and observability. Programs are verified by the kernel before loading (no infinite loops, no unsafe memory access).

**Limitations:** No floating point, restricted loops, limited stack (512 bytes), no unbounded memory allocation. This is why your UPF architecture uses eBPF for simple fast-path decisions but Wasm for complex DPI logic.

---

### AF_XDP

**What it is:** A socket type that allows user-space programs to receive packets directly from XDP, bypassing the rest of the kernel stack. Used in your UPF architecture as the bridge between the eBPF kernel fast-path and the user-space Wasm runtime.

---

## Section 8: Security Terms

### CVE-2023-41628 / CVE-2023-42358

**What they are:** The specific vulnerabilities in the OSC Near-RT RIC that your paper cites:
- **CVE-2023-41628:** A malicious xApp sending out-of-order E2 subscription responses crashes the E2T, causing total RAN control loss (DoS).
- **CVE-2023-42358:** Unauthorized REST API access across xApp boundaries — xApp A can call xApp B's APIs without any authentication.

**Why they matter:** These are your strongest evidence that container-based xApp isolation is broken. Your Wasm architecture neutralizes both: xApps can't reach E2T directly (no network stack), and can't call each other's APIs (only host functions are accessible).

---

### Zero Trust

**What it is:** A security model where nothing is trusted by default — not even devices/services inside the network perimeter. Every request must be authenticated and authorized, regardless of source. "Never trust, always verify."

**In O-RAN:** WG11 published a Zero Trust white paper (2024). Current compliance is "Initial" level. eZTrust/OZTrust implements zero trust for xApps via eBPF packet tagging. Your Wasm sandbox is a more structural form of zero trust — the xApp doesn't need to be "trusted" because it physically can't do anything unauthorized.

---

*This glossary covers the core terms across your entire repo. For protocol details (OSPF algorithms, BGP path selection, IPsec IKE handshake), see Doc 3 (Protocol Map) and Doc 4 (5G Production) — coming next.*
