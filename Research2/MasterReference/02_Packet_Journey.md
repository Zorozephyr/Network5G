# The Packet Journey — One Packet's Life, End to End

> Follow a single packet from the moment you tap "Play" on YouTube to the moment the video frame appears on your screen. We'll trace it through every protocol, every header, every device — in a 5G network.

---

## The Scenario

You're standing in a factory that has a **private 5G network**. Your phone (UE) is connected to the factory's 5G base station (gNB). You open YouTube. Your phone sends an HTTPS request to Google's server. We follow that request packet through every layer, every tunnel, every device — and then follow the response back.

---

## Part 1: The Downward Journey — From App to Radio Waves

### Step 1: The Application Layer (Layer 7)

Your YouTube app constructs an HTTP/2 GET request:
```
GET /watch?v=dQw4w9WgXcQ HTTP/2
Host: www.youtube.com
```

But this is HTTPS, so **TLS 1.3** (Layer 6) encrypts the entire HTTP payload. After the TLS handshake (which already happened when you opened YouTube), the app encrypts the request using the session key. The plaintext HTTP becomes an opaque encrypted blob.

**What's happened so far:**
```
[TLS-encrypted HTTP request payload]
```

### Step 2: The Transport Layer (Layer 4) — TCP

The encrypted payload is handed to the **TCP** stack. TCP does three things:

1. **Segmentation:** If the payload is larger than the MSS (Maximum Segment Size, typically 1460 bytes), TCP splits it into segments.
2. **Adds a TCP header** (20 bytes minimum):
   - Source port: 49152 (random ephemeral port on your phone)
   - Destination port: 443 (HTTPS)
   - Sequence number: tracks byte position for ordering
   - Acknowledgment number: confirms received data
   - Flags: PSH (push data to app), ACK
   - Window size: flow control (how much data the receiver can buffer)
   - Checksum: integrity check

**What's happened so far:**
```
[TCP Header: src=49152, dst=443, seq=1001, ack=5001] [TLS-encrypted HTTP payload]
```

### Step 3: The Network Layer (Layer 3) — IP

The TCP segment is handed to the **IP** layer. IP adds a header (20 bytes for IPv4):

- **Source IP:** 10.0.1.42 (your phone's private IP, assigned by the SMF via the UPF)
- **Destination IP:** 142.250.80.46 (YouTube's server IP, resolved by DNS earlier)
- **TTL (Time to Live):** 64 (decremented by each router; if it reaches 0, the packet is dropped — prevents infinite loops)
- **Protocol:** 6 (TCP)
- **DSCP:** Could be marked for QoS (e.g., 0x2E for Expedited Forwarding if this is a URLLC slice)

**The packet so far:**
```
[IP Header: src=10.0.1.42, dst=142.250.80.46, TTL=64, proto=TCP]
  [TCP Header: src=49152, dst=443]
    [TLS-encrypted HTTP payload]
```

**Total size so far:** ~20 (IP) + 20 (TCP) + payload ≈ depends on content, let's say 500 bytes total.

### Step 4: The Radio Layer — SDAP, PDCP, RLC, MAC, PHY

Now the packet must be sent over the 5G radio interface. The phone's modem processes it through the **5G NR protocol stack** — five sub-layers, each adding its own header:

#### SDAP (Service Data Adaptation Protocol)
Maps the IP packet to the correct **QoS flow**. The phone may have multiple QoS flows active (one for video, one for signaling). SDAP marks the packet with a QFI (QoS Flow Identifier) and maps it to the correct **DRB (Data Radio Bearer)**.

#### PDCP (Packet Data Convergence Protocol)
- **Header compression:** Uses ROHC (Robust Header Compression) to compress the IP/TCP headers from 40 bytes down to 1-4 bytes. Over radio (where every bit is expensive), this is critical.
- **Ciphering:** Encrypts the packet with the radio encryption key (negotiated during RRC connection setup). This is encryption at the radio layer — separate from TLS encryption at the application layer. Your packet is now double-encrypted.
- **Integrity protection:** Adds a MAC (Message Authentication Code) to detect tampering.
- **Sequence number:** For in-order delivery and duplicate detection during handovers.

#### RLC (Radio Link Control)
- **Segmentation/Reassembly:** If the PDCP PDU is too large for the radio resources allocated, RLC splits it into smaller segments.
- **ARQ (Automatic Repeat Request):** In Acknowledged Mode (AM), RLC retransmits lost segments. In Unacknowledged Mode (UM, used for URLLC), it doesn't — latency matters more than completeness.

#### MAC (Medium Access Control)
- **Multiplexing:** Combines RLC PDUs from different logical channels into one MAC PDU (Transport Block).
- **HARQ (Hybrid ARQ):** Rapid retransmission at the MAC layer. If the gNB can't decode a transport block, it sends a NACK, and the UE retransmits — typically within 1-4 ms. Uses soft combining: the gNB keeps the failed attempt and combines it with the retransmission for better decoding.
- **Scheduling:** The MAC layer determines *when* and *where* the UE transmits, based on the **scheduling grant** received from the gNB's MAC scheduler. The grant specifies: which PRBs (Physical Resource Blocks) to use, which MCS (Modulation and Coding Scheme), which antenna port.

#### PHY (Physical Layer)
- **Channel coding:** Applies LDPC (Low-Density Parity-Check) codes for data and Polar codes for control channels. These add redundancy for error correction.
- **Modulation:** Maps coded bits to symbols — QPSK (2 bits/symbol, robust), 16QAM (4 bits), 64QAM (6 bits), 256QAM (8 bits, max throughput but needs good signal).
- **OFDMA mapping:** Places symbols on specific subcarriers and time slots (the resource grid). Each PRB = 12 subcarriers × 1 slot.
- **MIMO precoding:** The phone has 2-4 antennas. MIMO processing creates spatial beams to direct energy toward the gNB.
- **RF (Radio Frequency):** The digital signal is converted to analog (DAC), upconverted to the carrier frequency (e.g., 3.5 GHz for n78), amplified, and transmitted through the antenna.

**The packet is now radio waves flying through the air.**

---

## Part 2: Through the RAN — RU, DU, CU

### Step 5: The Radio Unit (O-RU)

The O-RU's antenna receives the radio waves. It performs:
- **RF processing:** Low-noise amplification, downconversion from RF to baseband
- **ADC:** Converts analog signal to digital samples (I/Q samples)
- **Low-PHY processing (7.2x split):** FFT (Fast Fourier Transform) to convert time-domain samples to frequency-domain (recovering the OFDM subcarriers), cyclic prefix removal
- **eCPRI transport:** The frequency-domain I/Q samples are packetized using **eCPRI (Enhanced Common Public Radio Interface)** and sent over fiber to the O-DU.

**eCPRI frame:**
```
[Ethernet Header] [eCPRI Header: msg_type=IQ_data, seq_id=X] [I/Q samples for this slot]
```

The fronthaul link (O-RU → O-DU) requires very high bandwidth and strict timing: **25 Gbps** for a typical cell, with latency < 100µs one-way.

### Step 6: The Distributed Unit (O-DU)

The O-DU receives the I/Q samples and performs:
- **High-PHY:** Channel estimation, equalization, MIMO detection (reversing the beamforming), demodulation (symbols → bits), LDPC decoding (removing error correction coding to recover the transport block)
- **MAC:** HARQ processing (send ACK/NACK to UE), demultiplexing the MAC PDU into individual RLC PDUs
- **RLC:** Reassembly of RLC segments back into complete PDCP PDUs

The O-DU is computationally the heaviest part of the RAN — PHY processing (especially LDPC decoding and MIMO detection) requires enormous compute. This is where **Intel ACC100/ACC200 accelerator cards** or **FPGA** offload helps. This is also where **dApps** (E3 interface) would run — sub-1ms MAC/PHY optimization.

**Output:** Complete PDCP PDUs are sent to the O-CU over the **F1 interface** (midhaul). F1 uses GTP-U over UDP/IP.

### Step 7: The Centralized Unit (O-CU)

The O-CU receives the PDCP PDUs and performs:
- **PDCP:** Deciphering (removing radio-layer encryption), header decompression (ROHC → restore full IP/TCP headers), integrity verification, in-order delivery, duplicate elimination
- **RRC (Radio Resource Control):** Not involved in data transfer, but RRC on the CU manages the UE's connection state, security configuration, measurement reports, and handover decisions
- **SDAP:** Maps the QoS flow back to the correct N3 tunnel toward the UPF

**Output:** The original IP packet is now restored:
```
[IP Header: src=10.0.1.42, dst=142.250.80.46] [TCP Header] [TLS payload]
```

The CU encapsulates this in a **GTP-U tunnel** and sends it to the UPF over the **N3 interface** (backhaul):
```
[Outer IP: src=CU_IP, dst=UPF_IP]
  [UDP: src=random, dst=2152 (GTP-U port)]
    [GTP-U Header: TEID=0x12345678, seq=1]
      [Inner IP: src=10.0.1.42, dst=142.250.80.46]
        [TCP Header]
          [TLS payload]
```

**The TEID (0x12345678)** uniquely identifies this UE's PDU session on this tunnel. The UPF uses it to look up which rules (QoS, forwarding, billing) apply to this packet.

---

## Part 3: Through the Core — UPF and Beyond

### Step 8: The Transport Network (Backhaul)

The GTP-U encapsulated packet travels from the CU to the UPF over the operator's transport network. This is where the "traditional networking" protocols live:

**Layer 2:** The packet is an Ethernet frame on the operator's backbone network. **MACsec** may encrypt it at Layer 2 for security on shared fiber infrastructure.

**Layer 3 routing:** 
- Inside the operator's network: **OSPF** or **IS-IS** for intra-domain routing
- For traffic engineering: **Segment Routing (SRv6)** may be used to steer the packet along a deterministic path (important for URLLC slices that need guaranteed latency)
- At network boundaries: **BGP** for inter-domain routing (if the UPF is in a different AS than the CU)

If using **SRv6**, the packet gets an IPv6 Segment Routing Header:
```
[Outer IPv6: src=CU, dst=Segment_1]
  [SRH: segments=[UPF_SID, Segment_2, Segment_1]]
    [UDP]
      [GTP-U: TEID=0x12345678]
        [Inner IP packet]
```
Each router reads the top segment, forwards to it, and pops it. The packet follows the exact path encoded in the SRH.

### Step 9: The UPF (User Plane Function)

The UPF receives the GTP-U packet. Processing depends on the deployment:

**In a DPDK-based production UPF (most common):**
1. **NIC receives packet** via hardware RSS (Receive Side Scaling) which distributes packets across CPU cores based on flow hash
2. **DPDK poll-mode driver** picks up the packet from the NIC ring buffer (no kernel involvement, no interrupts)
3. **GTP-U decapsulation:** Strip outer IP/UDP/GTP-U headers. Read the TEID to identify the PDU session.
4. **PDR matching (Packet Detection Rules):** Look up the TEID + inner IP headers in the session table (installed by SMF via PFCP/N4). The PDR says what to do with this packet.
5. **FAR execution (Forwarding Action Rules):** Forward the inner IP packet toward the destination. Options:
   - **Forward to N6** (toward the internet/data network)
   - **Forward to another UPF** (in an uplink classifier / branching point scenario)
   - **Buffer** (if the UE is in idle mode and needs to be paged)
   - **Drop** (if the session/rule says so)
6. **QER (QoS Enforcement Rules):** Rate limiting, DSCP remarking
7. **URR (Usage Reporting Rules):** Count bytes/packets for billing

**In YOUR eBPF/XDP UPF architecture:**
1. **XDP hook** intercepts the packet at the NIC driver — before the kernel stack
2. eBPF program reads GTP-U TEID, looks up in BPF map
3. For 95% of traffic: strip GTP-U, forward (XDP_REDIRECT) — done in ~1µs
4. For packets needing DPI: write to eBPF ring buffer → Wasm engine processes → routing decision

**After UPF processing, the inner IP packet emerges:**
```
[IP: src=10.0.1.42, dst=142.250.80.46] [TCP] [TLS payload]
```

**NAT** may be applied here — the UPF replaces the UE's private IP (10.0.1.42) with the operator's public IP before sending to the internet. The UPF maintains a NAT table mapping (private IP:port) ↔ (public IP:port).

### Step 10: To the Internet

The packet exits the UPF on the **N6 interface** (toward the Data Network). From here, it's standard internet routing:

1. **Operator's edge router:** Runs BGP peering with other networks. Knows the route to 142.250.80.46 (Google's AS 15169).
2. **Transit providers / peering:** The packet may traverse 1-3 intermediate autonomous systems, each running BGP. At each hop, the MAC address changes (Layer 2), but the IP addresses stay the same (Layer 3).
3. **Google's edge:** Google's ToR (Top-of-Rack) switch receives the packet, forwards it to the YouTube server.

### Step 11: Google's Server

The YouTube server:
1. Strips IP/TCP headers
2. TLS decrypts the payload
3. Processes the HTTP/2 GET request
4. Fetches the video chunk from storage/CDN cache
5. Constructs an HTTP/2 response with video data
6. TLS encrypts it
7. TCP segments it
8. IP routes it back toward 10.0.1.42 (or the NAT'd public IP)

---

## Part 4: The Return Journey — Video Back to Your Phone

The return journey reverses every step:

1. **Google's server → Operator's network:** IP routing via BGP. The destination is the operator's public IP (or the UPF's N6 address).

2. **Operator's edge → UPF:** Routed internally via OSPF/IS-IS/SRv6 to the correct UPF.

3. **UPF processing (downlink):**
   - Reverse NAT: restore destination to 10.0.1.42
   - PDR lookup: match on destination IP → find PDU session → find TEID for the downlink tunnel
   - **GTP-U encapsulation:** Wrap the IP packet in GTP-U with the downlink TEID, outer IP addressed to the gNB's CU
   - QoS enforcement: DSCP marking, rate limiting

4. **UPF → CU (backhaul, N3):** GTP-U tunnel over the transport network (SRv6/MPLS/IP).

5. **CU → DU (midhaul, F1):** CU performs PDCP processing (ciphering, header compression), sends to DU.

6. **DU → RU (fronthaul, eCPRI):** DU performs MAC scheduling (allocates PRBs for this UE based on CQI reports), RLC segmentation, PHY processing (LDPC encoding, modulation, MIMO precoding). Sends I/Q samples to RU via eCPRI.

7. **RU → UE (radio):** RU converts digital to RF, transmits over the air. UE's antenna receives, processes through PHY→MAC→RLC→PDCP→SDAP→IP→TCP→TLS→HTTP, and the video frame appears on your screen.

---

## Part 5: Where Each Protocol Appears (Summary Diagram)

```
YOUR PHONE (UE)
├── App: HTTP/2 + TLS 1.3
├── Transport: TCP (port 443)
├── Network: IP (src=10.0.1.42, dst=142.250.80.46)
├── Radio: SDAP → PDCP (compress+encrypt) → RLC → MAC → PHY
└── Physical: Radio waves (e.g., 3.5 GHz n78)
         │
    ═══ AIR ═══
         │
O-RU (ANTENNA + LOW-PHY)
├── Physical: Antenna, RF, ADC/DAC
├── Low-PHY: FFT, CP removal
└── Transport to DU: eCPRI over Ethernet over fiber
         │
    ═══ FRONTHAUL (fiber, <100µs, 25Gbps) ═══
         │
O-DU (HIGH-PHY + MAC + RLC)
├── High-PHY: LDPC decode, MIMO detect, demodulate
├── MAC: HARQ, demux, scheduling
├── RLC: reassembly
└── Transport to CU: F1 (GTP-U/UDP/IP over Ethernet)
         │
    ═══ MIDHAUL (Ethernet/IP, routed) ═══
         │
O-CU (PDCP + RRC + SDAP)
├── PDCP: decipher, decompress, reorder
├── SDAP: QoS flow → tunnel mapping
└── Transport to UPF: N3 (GTP-U/UDP/IP, TEID=0x12345678)
         │
    ═══ BACKHAUL (Ethernet/IP, OSPF/IS-IS/SRv6/BGP, optionally IPsec) ═══
         │
UPF (USER PLANE FUNCTION)
├── GTP-U decapsulation (strip tunnel headers, read TEID)
├── PDR/FAR matching (session lookup, forwarding decision)
├── NAT (private IP → public IP)
├── QoS enforcement (DSCP, rate limit)
├── Usage reporting (billing)
└── Forward to N6 interface
         │
    ═══ OPERATOR BACKBONE (BGP peering, transit, SRv6) ═══
         │
INTERNET → GOOGLE'S SERVER
├── IP routing (BGP between ASes)
├── TCP reassembly
├── TLS decryption
└── HTTP/2 processing → serve video
```

---

## Part 6: The Headers at Each Stage

At the UPF (the most complex point), the packet has this structure:

```
┌──────────────────────────────────────────────────────────┐
│ LAYER 2: Ethernet Header (14 bytes)                       │
│   Dst MAC: UPF_NIC_MAC                                    │
│   Src MAC: ROUTER_MAC                                     │
│   EtherType: 0x0800 (IPv4)                                │
├──────────────────────────────────────────────────────────┤
│ LAYER 3: Outer IP Header (20 bytes)                       │
│   Src: CU_IP (192.168.50.10)                              │
│   Dst: UPF_IP (192.168.50.20)                             │
│   Protocol: 17 (UDP)                                      │
│   TTL: 62                                                 │
├──────────────────────────────────────────────────────────┤
│ LAYER 4: UDP Header (8 bytes)                             │
│   Src Port: 49200                                         │
│   Dst Port: 2152 (GTP-U)                                  │
├──────────────────────────────────────────────────────────┤
│ GTP-U Header (8+ bytes)                                   │
│   Version: 1                                              │
│   Protocol Type: 1 (GTP)                                  │
│   TEID: 0x12345678  ← THIS is how UPF finds the session  │
│   Sequence Number: 1                                      │
├──────────────────────────────────────────────────────────┤
│ LAYER 3: Inner IP Header (20 bytes) ← The REAL packet     │
│   Src: 10.0.1.42 (UE's IP)                               │
│   Dst: 142.250.80.46 (YouTube)                            │
│   Protocol: 6 (TCP)                                       │
├──────────────────────────────────────────────────────────┤
│ LAYER 4: TCP Header (20 bytes)                            │
│   Src Port: 49152                                         │
│   Dst Port: 443                                           │
│   Seq: 1001, Ack: 5001                                    │
├──────────────────────────────────────────────────────────┤
│ TLS Record (variable)                                     │
│   [Encrypted HTTP/2 GET request]                          │
└──────────────────────────────────────────────────────────┘
```

**Total overhead at UPF:** 14 (Ethernet) + 20 (outer IP) + 8 (UDP) + 8 (GTP-U) + 20 (inner IP) + 20 (TCP) = **90 bytes of headers** before you even get to the application data. For a 1500-byte Ethernet frame, that leaves 1410 bytes for actual payload. This overhead is why GTP-U encapsulation has a measurable cost.

---

## Part 7: Where Does the Near-RT RIC Fit?

The packet journey above is the **data plane** — user traffic. The Near-RT RIC operates on the **control plane** — it doesn't touch user packets. Instead:

1. The gNB periodically sends **E2 Indication messages** to the Near-RT RIC containing KPM metrics: per-UE throughput, RSRP, PRB utilization, CQI, etc.
2. xApps in the Near-RT RIC analyze these metrics and decide actions: "Move UE-42 from cell A to cell B" or "Reduce transmission power on cell C."
3. The xApp sends an **E2 Control message** back to the gNB, which executes the action.

**The E2 message flow (control plane):**
```
gNB → [SCTP] → E2T → [ASN.1 decode] → [RMR / DLB] → xApp
xApp → [send_e2_control()] → [Conflict Manager] → E2T → [SCTP] → gNB
```

In YOUR architecture, the right side becomes:
```
E2T → [DLB dispatch] → [DSA copy] → Wasm xApp → [host function] → CM → E2T
```

The user's YouTube packet never touches the RIC. But the RIC's decisions (which cell, which PRBs, which power level) directly affect the *quality* of that packet's radio transmission.

---

*Next: Document 3 — Protocol Map (the giant reference for "where does protocol X fit?")*
