# Module 2B: Linux Kernel Datapath & eBPF/XDP Deep Dive

## 1. Introduction to the Linux Datapath

While kernel bypass (DPDK) is popular for bare-metal performance, the modern Linux kernel datapath—supercharged by eBPF and XDP—has evolved to offer incredibly competitive performance while retaining the benefits of the kernel's rich networking stack (routing, conntrack, security).

Understanding the journey of a packet through the kernel is fundamental to designing a cloud-native 5G UPF.

---

## 2. The Packet Journey: From Wire to Application

Here is the detailed flow of an ingress packet through the Linux kernel.

```
┌───────────────────────────────────────────────────────────────┐
│                     1. HARDWARE LAYER                         │
│                                                               │
│  [Wire] → NIC MAC → NIC DMA Engine → Host RAM (Ring Buffer)   │
│                                 ↓                             │
│                           Hard IRQ (Interrupt)                │
└─────────────────────────────────┼─────────────────────────────┘
                                  │
┌─────────────────────────────────┼─────────────────────────────┐
│                     2. DRIVER & XDP LAYER                     │
│                                 ↓                             │
│  NAPI (Polling) → NIC Driver parses descriptor                │
│                                 ↓                             │
│                      ┌─────────────────────┐                  │
│                      │      XDP HOOK       │  ← eBPF Program  │
│                      │ (Raw packet buffer) │    (Fastest)     │
│                      └──────────┬──────────┘                  │
│                                 ↓                             │
│  Kernel allocates `sk_buff` (metadata structure)              │
└─────────────────────────────────┼─────────────────────────────┘
                                  │
┌─────────────────────────────────┼─────────────────────────────┐
│                     3. TC (TRAFFIC CONTROL) LAYER             │
│                                 ↓                             │
│                      ┌─────────────────────┐                  │
│                      │    TC INGRESS HOOK  │  ← eBPF Program  │
│                      │ (`cls_bpf` / qdisc) │    (Metadata)    │
│                      └──────────┬──────────┘                  │
└─────────────────────────────────┼─────────────────────────────┘
                                  │
┌─────────────────────────────────┼─────────────────────────────┐
│                     4. IP STACK & NETFILTER LAYER             │
│                                 ↓                             │
│  IP Sanity Checks (Checksum, Length)                          │
│                                 ↓                             │
│  [Netfilter: PREROUTING] (DNAT, Raw packet mangling)          │
│                                 ↓                             │
│  Routing Decision (Is this for local host or forward?)        │
│          ↙                                        ↘           │
│  [Netfilter: INPUT]                       [Netfilter: FORWARD]│
│          ↓                                        ↓           │
└──────────┼────────────────────────────────────────┼───────────┘
           │                                        │
┌──────────┼──────────────┐             ┌───────────┼───────────┐
│ 5. TRANSPORT & SOCKET   │             │ 6. EGRESS PATH        │
│          ↓              │             │           ↓           │
│ TCP/UDP Processing      │             │ [Netfilter: POSTROUT] │
│          ↓              │             │           ↓           │
│ Socket Receive Queue    │             │ TC Egress Hook        │
│          ↓              │             │           ↓           │
│ User Space App (read()) │             │ NIC Driver → Wire     │
└─────────────────────────┘             └───────────────────────┘
```

---

## 3. Deep Dive: XDP (eXpress Data Path)

XDP is the earliest possible hook point in the Linux networking stack. It executes eBPF programs directly inside the NIC driver, *before* the kernel allocates an `sk_buff`.

### 3.1 XDP Architecture and Constraints

*   **Data Structure:** Operates on `struct xdp_md`, which provides direct pointers to the raw packet payload in memory.
*   **Performance:** Ultra-high. By avoiding `sk_buff` allocation, XDP can drop or redirect packets at near-hardware speeds (millions of packets per second).
*   **Limitation:** It lacks access to kernel metadata (like socket state or routing tables) because that state hasn't been created for the packet yet.

### 3.2 XDP Action Verdicts

An XDP program inspects a packet and returns one of five verdicts:

1.  `XDP_DROP`: Drop packet immediately (Ideal for DDoS mitigation).
2.  `XDP_PASS`: Pass packet to the normal kernel network stack (allocate `sk_buff`).
3.  `XDP_TX`: Bounce the packet back out the *same* NIC it arrived on.
4.  `XDP_REDIRECT`: Send the packet to a *different* NIC or a special CPU map (AF_XDP).
5.  `XDP_ABORTED`: Drop packet and log an error.

### 3.3 AF_XDP for UPF

**AF_XDP** is an Address Family that allows XDP to redirect raw packets directly to a user-space memory buffer, completely bypassing the kernel stack. It offers performance rivaling DPDK but without requiring proprietary NIC drivers (PMDs); it relies on standard Linux drivers. Many modern UPF implementations are exploring AF_XDP as a more cloud-native alternative to DPDK.

---

## 4. Deep Dive: TC (Traffic Control) and `cls_bpf`

The TC hook is deeper in the stack. By the time a packet reaches TC, the driver has allocated the `sk_buff`.

### 4.1 TC vs. XDP

| Feature | XDP | TC (`cls_bpf`) |
| :--- | :--- | :--- |
| **Hook Point** | Inside NIC driver | Kernel network stack |
| **Context** | Raw memory (`xdp_md`) | Parsed metadata (`__sk_buff`) |
| **Direction** | Ingress only | Ingress and Egress |
| **Visibility**| L2/L3/L4 headers only | Full kernel state (Conntrack, Sockets) |
| **Best For** | DDoS, Fast Load Balancing | QoS, Shaping, Stateful Firewalling |

### 4.2 TC eBPF Architecture

TC eBPF programs attach to a specialized queueing discipline (qdisc) called `clsact`. 
Instead of complex legacy TC chains, modern deployments use **Direct Action (da)** mode. The eBPF program returns a verdict (e.g., `TC_ACT_OK` to pass, `TC_ACT_SHOT` to drop, `TC_ACT_REDIRECT` to forward) that the kernel executes immediately.

---

## 5. Netfilter and the GTP-U Tunnel Processing

Netfilter is the framework behind `iptables` and `nftables`. It provides five main hooks: `PREROUTING`, `INPUT`, `FORWARD`, `OUTPUT`, and `POSTROUTING`.

### 5.1 Kernel GTP Module (`gtp.ko`)

Linux has a built-in kernel module for GTP-U processing. 
*   **Control Plane (GTP-C):** Handled in user-space (e.g., OpenGGSN, free5GC SMF).
*   **User Plane (GTP-U):** The userspace daemon creates a GTP netdevice and uses Netlink to populate a hash table in the kernel with tunnel rules (TEID to IP mappings).
*   **Fast Path:** When a GTP-U UDP packet arrives, the kernel module intercepts it, strips the UDP/GTP headers (decapsulation), and injects the inner IP packet back into the network stack for routing.

### 5.2 Hooking GTP Traffic

If you need to write custom rules for a UPF:

1.  **Before Decapsulation (`PREROUTING`):**
    *   Hook here if you want to filter based on the *outer* transport IP or the raw GTP-U UDP port (2152).
2.  **After Decapsulation (`FORWARD`):**
    *   If your system is routing the decapsulated traffic, the *inner* IP packet will hit the `FORWARD` hook. Hook here to apply policies based on the UE's (User Equipment) actual IP address.

---

## 6. Synergy: Combining XDP, TC, and Kernel Routing

A modern, eBPF-based 5G UPF (like those built with Cilium's datapath philosophy) combines these layers:

1.  **XDP (Ingress Fast Path):** Inspects incoming packets. If a packet belongs to a known GTP-U fast-path flow, XDP decapsulates it and uses `XDP_REDIRECT` to send it immediately to the egress interface.
2.  **Metadata Passing:** If XDP cannot process the packet (e.g., a fragmented packet or a complex rule), it passes it up (`XDP_PASS`), optionally storing custom metadata in the packet's `data_meta` area.
3.  **TC (Stateful Processing):** The TC eBPF program reads the metadata left by XDP, accesses the kernel's connection tracking, applies QoS rules, and routes the packet.
4.  **Kernel IP Stack:** Handles edge cases, local socket delivery, and routing table lookups that eBPF delegates.
