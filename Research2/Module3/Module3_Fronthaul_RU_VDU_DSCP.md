# Module 3B: 5G Fronthaul — RU to vDU and DSCP QoS

## 1. The Fronthaul Network

The network segment connecting the Radio Unit (RU) on the cell tower to the Virtualized Distributed Unit (vDU) is called the **Fronthaul**.

### 1.1 The Shift from CPRI to eCPRI

*   **CPRI (4G):** A synchronous, constant-bit-rate, point-to-point protocol. It essentially transmitted raw radio waveforms over dark fiber. It was incredibly inefficient and required massive bandwidth that scaled with the number of antennas, not the actual user traffic.
*   **eCPRI (5G):** Enhanced CPRI is a **packet-based** protocol that runs over standard Ethernet or IP. By moving some of the lower-level PHY processing (like FFT/iFFT) into the RU (the "Split 7.2" architecture), the bandwidth required between the RU and DU is drastically reduced, and the bandwidth now scales dynamically with actual user traffic.

Because eCPRI uses standard Ethernet/IP switches rather than dedicated dark fiber, it introduces a major challenge: **Network Congestion and Jitter.**

---

## 2. Fronthaul QoS Requirements

The 5G Radio expects data exactly when it needs it. If a packet from the vDU arrives at the RU late, that radio frame is missed, resulting in dropped calls or degraded throughput.

The Fronthaul network typically carries four distinct types of traffic planes, each with strict requirements:

| Traffic Plane | Purpose | Sensitivity | Max Latency |
| :--- | :--- | :--- | :--- |
| **U-Plane (User)** | Actual user data (IQ samples) | Extremely high | ~100-250 μs |
| **C-Plane (Control)** | Scheduling info (tells RU what to transmit) | Extremely high | ~100-250 μs |
| **S-Plane (Sync)** | Precision Time Protocol (PTP 1588) | Jitter sensitive | strict nanoseconds |
| **M-Plane (Mgmt)** | OAM, firmware updates, stats | Low (Best effort) | > 10 ms |

---

## 3. Implementing QoS with DSCP

Because the fronthaul uses standard IP networks, we must use standard IP Quality of Service (QoS) mechanisms to ensure U-Plane and C-Plane traffic is never delayed by a large M-Plane firmware download.

### 3.1 Differentiated Services Code Point (DSCP)

DSCP is a 6-bit field in the IPv4 (Type of Service) or IPv6 (Traffic Class) header used to classify packets.

When the vDU generates a packet for the RU, the vDU software must mark the IP header with a specific DSCP value. Every Ethernet switch and router between the vDU and the RU looks at this DSCP value to determine which hardware queue to place the packet into.

### 3.2 Typical Fronthaul DSCP Mappings

While exact mappings can be vendor-specific, O-RAN and industry guidelines recommend strict separation:

```
┌─────────────────┐
│      vDU        │
│                 │
│  [C-Plane Gen]──┼─── marks DSCP 46 (EF) ──┐
│                 │                         │
│  [U-Plane Gen]──┼─── marks DSCP 46 (EF) ──┼───→ To Fronthaul Switch
│                 │                         │
│  [S-Plane PTP]──┼─── marks DSCP 48 (CS6) ─│
│                 │                         │
│  [M-Plane OAM]──┼─── marks DSCP 0  (BE) ──┘
└─────────────────┘
```

*   **EF (Expedited Forwarding - DSCP 46):** Used for U-Plane and C-Plane. The fronthaul switches must map DSCP 46 to a **Strict Priority (SP)** hardware queue. If a packet is in this queue, the switch drops whatever else it is doing and forwards this packet immediately.
*   **CS6 (Class Selector 6 - DSCP 48):** Often used for S-Plane (Synchronization). Timing packets are critical; if they are delayed by jitter, the cell tower loses sync with the network, and the radio shuts down to prevent interference.
*   **BE (Best Effort - DSCP 0):** Used for M-Plane. Handled whenever the network has free capacity.

*(Note: In purely switched Layer-2 networks, the equivalent marking is the IEEE 802.1p Priority Code Point (PCP) in the VLAN tag, where C/U-Plane is often mapped to PCP Priority 7).*

---

## 4. The vDU Hardware Pipeline for QoS

How does the vDU ensure packets are marked correctly and transmitted with zero latency?

```
┌────────────────────────────────────────────────────────┐
│                        vDU Server                      │
│                                                        │
│  ┌──────────────┐     ┌──────────────┐                 │
│  │ MAC Scheduler│     │  L1 PHY App  │                 │
│  │ (User Space) │────→│ (User Space) │                 │
│  └──────────────┘     └──────┬───────┘                 │
│                              │                         │
│                  (Constructs eCPRI payload)            │
│                              │                         │
│  ┌───────────────────────────▼──────────────────────┐  │
│  │                     DPDK PMD                     │  │
│  │ • Adds IPv4/v6 Header                            │  │
│  │ • Sets DSCP = 46 (EF) in IP Header               │  │
│  │ • Adds Ethernet/VLAN Header (Sets PCP = 7)       │  │
│  │ • Pushes pointer to NIC TX Ring                  │  │
│  └───────────────────────────┬──────────────────────┘  │
│                              │ Direct DMA              │
│  ┌───────────────────────────▼──────────────────────┐  │
│  │                  Fronthaul NIC                   │  │
│  │ (Hardware QoS queues ensure EF packets bypass    │  │
│  │  any background OS traffic leaving the server)   │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### 4.1 Why DPDK is Mandatory Here
If the vDU used the standard Linux kernel network stack, the kernel scheduler might pause the transmission of a critical C-Plane packet for a few milliseconds to process a background task. In a 5G fronthaul, a 2-millisecond delay means the packet is permanently lost to the radio. DPDK's polling model and strict core pinning guarantee that the DSCP-marked packet hits the physical wire the microsecond it is ready.
