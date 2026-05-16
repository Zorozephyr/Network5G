# Module 1B: Super NICs, DPUs, & RDMA — Deep Dive

## 1. Understanding the Naming: NIC vs SmartNIC vs DPU vs SuperNIC

These terms are often confused. Here's the precise hierarchy:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Network Adapter Spectrum                     │
│                                                                 │
│  Traditional     SmartNIC        DPU              SuperNIC      │
│  NIC                                                            │
│  ──────────     ────────        ───              ────────       │
│  Data movement  Data movement   Full SoC with    Streamlined    │
│  only           + HW offloads   ARM cores +      for 1:1        │
│                 (checksum,      accelerators +   GPU:NIC ratio  │
│                  RSS, VXLAN)    runs own OS      AI networking  │
│                                                                 │
│  ◄── Simplicity                          Complexity ──►         │
│  ◄── Lower Cost                          Higher Cost ──►        │
│  ◄── Less Flexible                    More Flexible ──►         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Data Processing Units (DPUs) — The "Third Processor"

### 2.1 Why DPUs Exist

The core problem DPUs solve:

```
In a traditional server:
  CPU must handle:
  ├── Application workloads (what you're paying for)
  ├── Network stack processing (overhead)
  ├── Storage I/O (overhead)  
  ├── Security/encryption (overhead)
  ├── Virtual switching (overhead)
  └── Management/telemetry (overhead)

  Infrastructure overhead: 20-60% of CPU cycles!

With a DPU:
  Host CPU handles:
  └── Application workloads ONLY ✓

  DPU handles:
  ├── Network stack processing
  ├── Storage I/O
  ├── Security/encryption
  ├── Virtual switching
  └── Management/telemetry
```

### 2.2 NVIDIA BlueField DPU Architecture

NVIDIA's BlueField is the most widely deployed DPU family.

#### BlueField-3 Architecture:
```
┌──────────────────────────────────────────────────────────┐
│                   BlueField-3 DPU                        │
│                                                          │
│  ┌─────────────────────┐  ┌────────────────────────────┐ │
│  │   ConnectX-7         │  │  ARM Subsystem             │ │
│  │   Networking Engine  │  │                            │ │
│  │                      │  │  16× ARM Cortex-A78 cores  │ │
│  │  • 400 Gbps          │  │  @ 3.0 GHz                │ │
│  │  • RDMA (IB + RoCE)  │  │                            │ │
│  │  • GPUDirect RDMA    │  │  Runs Ubuntu/RHEL Linux    │ │
│  │  • OVS HW offload    │  │  DOCA SDK applications     │ │
│  │  • eSwitch           │  │                            │ │
│  │  • SR-IOV (PCIe VFs) │  │  16 GB on-chip DDR5       │ │
│  └─────────────────────┘  └────────────────────────────┘ │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │              Hardware Accelerators                   │ │
│  │                                                     │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │ │
│  │  │ Crypto   │ │ RegEx    │ │ Compress │ │ IPsec  │ │ │
│  │  │ Engine   │ │ Engine   │ │ Engine   │ │ Engine │ │ │
│  │  │ AES/SHA  │ │ DPI/IDS  │ │ LZ4/zlib │ │ Inline │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └────────┘ │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌───────────────────┐  ┌──────────────────────────────┐ │
│  │ PCIe Gen5 ×16     │  │ NVMe SNAP                    │ │
│  │ to Host CPU       │  │ (Storage emulation)           │ │
│  └───────────────────┘  └──────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

#### DOCA (Data Center Infrastructure on a Chip Architecture):
DOCA is NVIDIA's SDK for programming BlueField DPUs:

```
Application Layer:
┌──────────┬──────────┬──────────┬──────────┐
│ OVS      │ SPDK     │ IPsec    │ Custom   │
│ Offload  │ Storage  │ Gateway  │ Apps     │
└────┬─────┴────┬─────┴────┬─────┴────┬─────┘
     │          │          │          │
DOCA Libraries:
┌────┴─────┬────┴─────┬────┴─────┬────┴─────┐
│ DOCA     │ DOCA     │ DOCA     │ DOCA     │
│ Flow     │ Crypto   │ DMA      │ GPUNet   │
│ (pipeline│ (offload │ (memory  │ (GPU     │
│  accel)  │  crypto) │  ops)    │  direct) │
└────┬─────┴────┬─────┴────┬─────┴────┬─────┘
     │          │          │          │
Hardware:
┌────┴──────────┴──────────┴──────────┴─────┐
│         BlueField DPU Silicon              │
└────────────────────────────────────────────┘
```

### 2.3 DPU Security Model — Infrastructure Isolation

A critical DPU feature for cloud/telco:

```
┌──────────────────────────────────────────┐
│              Physical Server             │
│                                          │
│  ┌────────────────────┐                  │
│  │   Tenant VM/Pod    │  ← Untrusted     │
│  │   (Application)    │     workload      │
│  └────────┬───────────┘                  │
│           │ PCIe VF (SR-IOV)             │
│  ═════════╪══════════════════════════    │
│           │  TRUST BOUNDARY              │
│  ═════════╪══════════════════════════    │
│  ┌────────┴───────────────────────────┐  │
│  │           DPU                       │  │
│  │  • Enforces network policies       │  │
│  │  • Encrypts all traffic            │  │ ← Trusted
│  │  • Monitors for intrusion          │  │    infrastructure
│  │  • Manages storage access          │  │
│  │  • Tenant CANNOT bypass DPU        │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

The host CPU (and the tenant running on it) has NO access to the DPU's ARM cores or management plane. This is a hardware-enforced trust boundary.

---

## 3. Super NICs — Purpose-Built for AI

### 3.1 What Makes a SuperNIC Different from a DPU?

| Feature | DPU (BlueField) | SuperNIC (ConnectX SuperNIC) |
|---------|-----------------|------------------------------|
| **Goal** | General infrastructure offload | AI network acceleration only |
| **ARM Cores** | Yes (16×) | No — stripped out |
| **Programmability** | Full Linux OS | Minimal — firmware only |
| **Deployment** | 1 DPU per server | 1 SuperNIC per GPU (1:1 ratio) |
| **Key Features** | vSwitch, storage, security | Congestion control, adaptive routing |
| **Complexity** | High | Low (streamlined) |

### 3.2 Why 1:1 GPU:NIC Ratio?

In modern AI training clusters:

```
Traditional (1 NIC shared):
  ┌────────┐
  │ NIC    │──── Network
  └───┬────┘
      │ PCIe (shared)
  ┌───┴────┐┌────────┐┌────────┐┌────────┐
  │ GPU 0  ││ GPU 1  ││ GPU 2  ││ GPU 3  │
  └────────┘└────────┘└────────┘└────────┘
  
  Problem: NIC is bottleneck. GPUs compete for bandwidth.
  GPU utilization drops to 60-70%.

SuperNIC (1:1 ratio):
  ┌────────┐┌────────┐┌────────┐┌────────┐
  │SuperNIC││SuperNIC││SuperNIC││SuperNIC│──── Network
  └───┬────┘└───┬────┘└───┬────┘└───┬────┘
      │ PCIe    │ PCIe    │ PCIe    │ PCIe (dedicated)
  ┌───┴────┐┌───┴────┐┌───┴────┐┌───┴────┐
  │ GPU 0  ││ GPU 1  ││ GPU 2  ││ GPU 3  │
  └────────┘└────────┘└────────┘└────────┘
  
  Each GPU has dedicated network bandwidth.
  GPU utilization: 95%+
```

### 3.3 NVIDIA Spectrum-X Platform

Spectrum-X = SuperNICs + Spectrum-4 Ethernet Switches, co-designed for AI:

Key innovations:
- **Adaptive Routing**: Switch dynamically picks least-congested path
- **Packet Spraying + Reordering**: Spray packets across all paths, SuperNIC reassembles in order
- **RoCE Optimization**: Makes Ethernet perform like InfiniBand for RDMA workloads

---

## 4. RDMA — Remote Direct Memory Access (Deep Dive)

### 4.1 Why RDMA Exists

Traditional TCP/IP networking has massive overhead:

```
Traditional TCP Send:
  App buffer (user space)
    → copy to kernel buffer         ← CPU copy #1
    → TCP segmentation              ← CPU processing
    → IP header addition            ← CPU processing  
    → Checksum computation          ← CPU processing
    → copy to NIC DMA buffer        ← CPU copy #2
    → Context switch (user→kernel)  ← OS overhead
    → Interrupt handling            ← CPU overhead

  Total: 2 memory copies, multiple context switches, 10-50μs latency

RDMA Send:
  App buffer (user space)
    → Post WQE to Send Queue        ← Single register write
    → NIC DMA reads directly from app buffer
    → NIC handles segmentation, headers, checksums
    → Done

  Total: 0 memory copies, 0 context switches, 1-2μs latency
```

### 4.2 RDMA Operations

RDMA supports several operation types:

```
┌─────────────────────────────────────────────────────┐
│                  RDMA Operations                     │
├─────────────────────────────────────────────────────┤
│                                                     │
│  TWO-SIDED (like traditional send/recv):            │
│  ┌────────┐         ┌────────┐                      │
│  │ Node A │──SEND──→│ Node B │                      │
│  │        │←─RECV───│        │                      │
│  └────────┘         └────────┘                      │
│  Both sides must participate.                       │
│  Receiver posts receive buffer in advance.          │
│                                                     │
│  ONE-SIDED (true remote memory access):             │
│  ┌────────┐  RDMA   ┌────────┐                      │
│  │ Node A │──WRITE─→│ Node B │  B's CPU not involved│
│  │        │         │ memory │                      │
│  └────────┘         └────────┘                      │
│                                                     │
│  ┌────────┐  RDMA   ┌────────┐                      │
│  │ Node A │←─READ───│ Node B │  B's CPU not involved│
│  │        │         │ memory │                      │
│  └────────┘         └────────┘                      │
│                                                     │
│  ATOMIC:                                            │
│  Compare-and-Swap, Fetch-and-Add on remote memory   │
│  Used for distributed locking / synchronization     │
└─────────────────────────────────────────────────────┘
```

### 4.3 RDMA Software Architecture

```
┌─────────────────────────────────────────────┐
│              User Application               │
│                                             │
│  ibv_post_send()    ibv_post_recv()         │
│  ibv_poll_cq()      ibv_reg_mr()            │
└──────────┬──────────────────────────────────┘
           │ (Direct HW access via mmap'd doorbell)
           │ NO KERNEL INVOLVEMENT for data path
           │
┌──────────┴──────────────────────────────────┐
│           libibverbs (User-Space Library)    │
│                                             │
│  Abstracts vendor-specific HW details       │
│  Provides unified "verbs" API               │
└──────────┬──────────────────────────────────┘
           │
           │  mmap'd registers (doorbells)
           │  + shared memory (queues)
           │
┌──────────┴──────────────────────────────────┐
│           RDMA NIC (RNIC) Hardware           │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌───────────┐ │
│  │ Send     │  │ Receive  │  │Completion │ │
│  │ Queue(SQ)│  │ Queue(RQ)│  │ Queue(CQ) │ │
│  └──────────┘  └──────────┘  └───────────┘ │
│                                             │
│  ┌──────────────────────────────────────┐   │
│  │ Transport Engine                      │   │
│  │ (Segmentation, ACKs, Retransmission)  │   │
│  └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

### 4.4 Memory Registration (Critical Concept)

Before RDMA can access memory, it must be **registered**:

```
Step 1: Application allocates buffer
  void *buf = malloc(1MB);

Step 2: Register with RDMA subsystem
  struct ibv_mr *mr = ibv_reg_mr(pd, buf, 1MB, access_flags);
  
  What happens internally:
  ├── Kernel pins pages (prevents swapping to disk)
  ├── Creates mapping: virtual address → physical address  
  ├── Generates Memory Key pair:
  │   ├── lkey (local key) — used by local NIC
  │   └── rkey (remote key) — shared with remote node for one-sided ops
  └── Loads mapping into NIC's Translation Table

Step 3: Remote node can now do:
  RDMA_WRITE(remote_addr=buf, rkey=mr->rkey, data=...)
  // Writes directly to buf WITHOUT remote CPU involvement
```

**Why this matters**: Memory registration is EXPENSIVE (takes milliseconds). For high-performance systems, you pre-register large buffers and manage them carefully.

### 4.5 Queue Pair (QP) and Connection Types

```
RDMA Connection Types:

RC (Reliable Connection):
  • Point-to-point, ordered, reliable delivery
  • Like TCP — guarantees delivery with retransmission
  • Most common for storage (NVMe-oF) and general RDMA

UC (Unreliable Connection):
  • Point-to-point, ordered, NO retransmission
  • Lower overhead than RC
  • Used when app handles reliability

UD (Unreliable Datagram):
  • One-to-many (multicast possible)
  • Like UDP — no guarantees
  • Used for discovery, management

MRC (Multipath Reliable Connection) — NEW:
  • Extension of RC
  • Single connection spans MULTIPLE physical paths
  • Dynamic load balancing across paths
  • Key for AI training networks (NVIDIA Spectrum-X)
```

### 4.6 InfiniBand vs RoCEv2 — Architectural Deep Dive

```
InfiniBand Network Stack:
┌─────────────┐
│ Application │
├─────────────┤
│ IB Verbs    │  ← Unified RDMA API
├─────────────┤
│ IB Transport│  ← Reliable delivery, flow control
├─────────────┤
│ IB Network  │  ← IB routing (LID-based within subnet)
├─────────────┤
│ IB Link     │  ← Credit-based flow control (NATIVELY lossless)
├─────────────┤
│ IB Physical │  ← Dedicated IB cables & switches
└─────────────┘

RoCEv2 Network Stack:
┌─────────────┐
│ Application │
├─────────────┤
│ IB Verbs    │  ← Same unified RDMA API!
├─────────────┤
│ IB Transport│  ← Same reliable delivery
├─────────────┤
│ UDP/IP      │  ← Standard IP routing (routable across L3!)
├─────────────┤
│ Ethernet    │  ← REQUIRES PFC + ECN for lossless behavior
├─────────────┤
│ Eth Physical│  ← Standard Ethernet cables & switches
└─────────────┘
```

#### Why Lossless Matters for RDMA:

```
If a packet is dropped in RDMA:

1. The "go-back-N" retransmission kicks in
2. ENTIRE message (potentially MB of data) must be retransmitted
3. This causes massive latency spikes (tail latency)
4. In AI training: one slow node slows ALL nodes (synchronous training)

Solution for RoCEv2:
┌─────────────────────────────────────────┐
│ PFC (Priority-based Flow Control)       │
│                                         │
│ When switch buffer > threshold:         │
│   Switch → sends PAUSE frame upstream   │
│   Sender → stops transmitting           │
│                                         │
│ Problem: Can cause "PFC storms"         │
│ (cascading pauses across network)       │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ ECN (Explicit Congestion Notification)  │
│                                         │
│ When switch detects congestion:         │
│   Switch → marks packet with CE bit     │
│   Receiver → sends CNP to sender        │
│   Sender → reduces transmission rate    │
│                                         │
│ More graceful than PFC.                 │
│ DCQCN = RoCEv2 congestion control algo  │
└─────────────────────────────────────────┘
```

### 4.7 GPUDirect RDMA

This is the pinnacle of zero-copy networking:

```
Without GPUDirect RDMA:
  GPU Memory → PCIe → System RAM → PCIe → NIC → Network
  (2 PCIe traversals, 1 memory copy, CPU involved)

With GPUDirect RDMA:
  GPU Memory → PCIe → NIC → Network  
  (1 PCIe traversal, 0 copies, 0 CPU involvement)

  The NIC reads DIRECTLY from GPU HBM (High Bandwidth Memory)
  Latency reduction: 30-40%
  Throughput increase: Up to 2×
```

---

## 5. SR-IOV — The Bridge Between Physical and Virtual

### 5.1 SR-IOV Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Physical Server                        │
│                                                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                 │
│  │  VM 1    │ │  VM 2    │ │  VM 3    │                 │
│  │          │ │          │ │          │                 │
│  │ VF0 drv  │ │ VF1 drv  │ │ VF2 drv  │ Guest drivers  │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘                 │
│       │ PCIe       │ PCIe       │ PCIe  (passthrough)    │
│  ═════╪════════════╪════════════╪════════════════════    │
│       │            │            │        IOMMU boundary   │
│  ═════╪════════════╪════════════╪════════════════════    │
│  ┌────┴────────────┴────────────┴─────────────────────┐  │
│  │              Physical NIC (PF + VFs)                │  │
│  │                                                     │  │
│  │  ┌────┐  ┌────┐  ┌────┐  ┌────┐                    │  │
│  │  │ PF │  │ VF0│  │ VF1│  │ VF2│                    │  │
│  │  │    │  │    │  │    │  │    │                    │  │
│  │  │Mgmt│  │Data│  │Data│  │Data│                    │  │
│  │  │Full│  │Lite│  │Lite│  │Lite│                    │  │
│  │  └────┘  └────┘  └────┘  └────┘                    │  │
│  │                                                     │  │
│  │  ┌────────────────────────────────────────────┐     │  │
│  │  │         L2 Switching / Classification       │     │  │
│  │  │         (MAC/VLAN based forwarding)         │     │  │
│  │  └────────────────────────────────────────────┘     │  │
│  │                                                     │  │
│  │  ════════════════════════════════                   │  │
│  │              Physical Port(s)                       │  │
│  └─────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

### 5.2 PF vs VF in Detail

**Physical Function (PF)**:
- Full PCIe configuration space
- Can create/destroy VFs (`echo 4 > /sys/class/net/eth0/device/sriov_numvfs`)
- Manages the NIC's internal L2 switch (MAC/VLAN assignment per VF)
- Controlled by the hypervisor/host OS

**Virtual Function (VF)**:
- Minimal PCIe config space (lightweight)
- Has its own set of TX/RX queues and interrupt vectors
- Gets its own MAC address, VLAN tag
- Assigned to VM/container via VFIO/IOMMU passthrough
- DMA goes directly to/from VM memory — hypervisor is NOT in the data path

### 5.3 SR-IOV + DPDK for 5G UPF

```
Typical 5G UPF deployment:

  ┌─────────────────────────────────┐
  │        UPF Pod/VM               │
  │                                 │
  │  ┌───────────────────────────┐  │
  │  │    DPDK Application       │  │
  │  │    (UPF Fast Path)        │  │
  │  │                           │  │
  │  │  Poll Mode Driver (PMD)   │  │
  │  │  ├── Polls VF RX queue    │  │
  │  │  ├── Zero-copy processing │  │
  │  │  └── Writes to VF TX queue│  │
  │  └───────────┬───────────────┘  │
  │              │                  │
  │  ┌───────────┴───────────────┐  │
  │  │  VF (SR-IOV passthrough)  │  │
  │  │  via VFIO-PCI driver      │  │
  │  └───────────┬───────────────┘  │
  └──────────────┼──────────────────┘
                 │  Direct PCIe / DMA
  ┌──────────────┴──────────────────┐
  │        Physical SmartNIC        │
  │   ┌──────────────────────┐      │
  │   │ VF0 → N3 interface   │      │
  │   │ VF1 → N6 interface   │      │
  │   │ VF2 → N4 interface   │      │
  │   └──────────────────────┘      │
  └─────────────────────────────────┘
```

---

## 6. Extended Learning Resources

### Deep Reading
1. **"RDMA over Commodity Ethernet at Scale"** (SIGCOMM 2016, Microsoft) — How Azure deployed RoCEv2
2. **"IRN: Improved RDMA NIC"** — Designing RDMA that works well even WITH packet loss
3. **NVIDIA DOCA Developer Guide**: https://docs.nvidia.com/doca/
4. **InfiniBand Architecture Specification Volume 1** (IBTA) — The authoritative reference

### Hands-On
- **rdma-core library** (GitHub): Build RDMA apps with libibverbs
- **perftest tools**: `ib_send_bw`, `ib_read_lat` — Benchmark RDMA performance
- **SoftRoCE (rxe)**: Linux kernel module that emulates RoCE — practice RDMA without special hardware
- **SR-IOV Lab**: Enable VFs on any Intel/Mellanox NIC: `echo N > sriov_numvfs`

### Videos
- **"RDMA and High-Performance Networking"** — USENIX NSDI talks
- **"BlueField DPU Architecture"** — NVIDIA GTC recordings
- **"Understanding PFC and ECN for RoCE"** — Mellanox/NVIDIA webinars
