# DPU Hardware Deep Dive: What Is Actually Hardware vs. Software?

**Date:** June 2026  
**Purpose:** Resolving the most common confusion about DPU architecture

---

## Q1: What Do We Mean by "Hardware"? Don't the ARM Cores Run Linux?

### The Critical Distinction You Must Understand

A BlueField-3 DPU is NOT one thing. It is **three different types of compute** fused onto one chip. Each has a fundamentally different nature:

```
┌─────────────────────────────────────────────────────────────────────┐
│                   NVIDIA BlueField-3 DPU (One Physical Chip)        │
│                                                                     │
│  ┌─────────────────────┐  ┌──────────────────┐  ┌───────────────┐  │
│  │   COMPONENT 1:      │  │  COMPONENT 2:    │  │ COMPONENT 3:  │  │
│  │   eSwitch +         │  │  16× ARM         │  │ DPA           │  │
│  │   ConnectX-7 ASIC   │  │  Cortex-A78      │  │ (Data Path    │  │
│  │                     │  │  Cores           │  │  Accelerator) │  │
│  │                     │  │                  │  │               │  │
│  │  ┌───────────────┐  │  │  ┌────────────┐  │  │ 16 cores,     │  │
│  │  │ Flow tables   │  │  │  │ Ubuntu     │  │  │ 256 threads   │  │
│  │  │ Token buckets │  │  │  │ Linux      │  │  │               │  │
│  │  │ Match-action  │  │  │  │ 24.04      │  │  │ Programmable  │  │
│  │  │ HW counters   │  │  │  │            │  │  │ but NOT a     │  │
│  │  └───────────────┘  │  │  │ DOCA SDK   │  │  │ general CPU   │  │
│  │                     │  │  │ OVS        │  │  │               │  │
│  │  NO operating       │  │  │ Your AI    │  │  │ Think:        │  │
│  │  system.            │  │  │ code runs  │  │  │ "hardware     │  │
│  │  No software.       │  │  │ HERE       │  │  │  threads"     │  │
│  │  Pure silicon       │  │  │            │  │  │               │  │
│  │  logic.             │  │  └────────────┘  │  └───────────────┘  │
│  │                     │  │                  │                      │
│  │  THIS is "hardware" │  │  This is         │                      │
│  │                     │  │  SOFTWARE on      │                      │
│  │                     │  │  dedicated HW     │                      │
│  └─────────────────────┘  └──────────────────┘                      │
│                                                                     │
│  Also on chip: 32 GB DDR5 RAM, 8 MB L2 cache, PCIe Gen5 switch,   │
│  crypto engines, compression engines, BMC                           │
└─────────────────────────────────────────────────────────────────────┘
```

### Let's Be Precise About Each Component

#### Component 1: eSwitch + ConnectX-7 ASIC — THIS IS TRUE HARDWARE

- **What it is:** A fixed-function switching ASIC — transistors wired to perform packet matching, counting, and forwarding
- **Does it run an OS?** **NO.** It has no operating system. It has no software. It is silicon logic gates.
- **How it works:** You program it by writing **rules** (flow table entries) into its memory. After that, it processes packets automatically, at wire speed, forever, with zero software involvement per-packet.
- **Speed:** 400 Gbps line rate, ~100-500 ns per packet
- **Analogy:** A traffic light. You set the timing once (green for 30 seconds, red for 45 seconds), and then it runs on its own. It doesn't need a computer to keep working. The eSwitch is the same — you set "URLLC = 200 Mbps max" and it enforces that rule on every packet without asking anyone.

#### Component 2: ARM Cortex-A78 Cores — THIS IS SOFTWARE ON DEDICATED HARDWARE

- **What it is:** 16 general-purpose CPU cores, exactly like what's in a high-end Android phone
- **Does it run an OS?** **YES. Full Ubuntu Linux 24.04.** Complete with kernel, systemd, SSH, apt, everything.
- **How it works:** You write C/Python/Rust programs, compile them, and run them on this Linux system. Your AI model (the TCN) runs here as a regular Linux process.
- **Speed:** Same as any ARM CPU — microseconds to milliseconds per task
- **Analogy:** A small server computer that happens to be soldered onto the same chip as the network card. It IS a computer. It runs software.

#### Component 3: DPA (Data Path Accelerator) — PROGRAMMABLE HARDWARE

- **What it is:** 16 specialized cores with 256 hardware threads, designed for packet processing tasks
- **Does it run an OS?** **No.** It runs small programs loaded via DOCA, but it's not a general-purpose CPU.
- **Analogy:** Like a GPU's CUDA cores — programmable, but purpose-built for specific workloads

### So When We Say "Slice Management in Hardware," What EXACTLY Do We Mean?

We mean **two different things happening in two different places:**

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   DECISION (Software — ARM Cores)         ENFORCEMENT (Hardware) │
│   ─────────────────────────                ──────────────────── │
│                                                                  │
│   "Should URLLC get 200 Mbps?"            "URLLC = 200 Mbps,    │
│   → AI model runs                          enforce on every      │
│   → Checks uncertainty                     packet at wire speed" │
│   → Compares against 5QI                                         │
│   → Makes reserve/forgo decision           This part is HARDWARE │
│                                            (eSwitch ASIC)        │
│   This part is SOFTWARE                                          │
│   (Linux process on ARM)                   It runs even if the   │
│   It runs ~every 1-10 ms                   ARM cores CRASH       │
│                                                                  │
│              │                                    ▲              │
│              │ writes new token bucket value       │              │
│              └────────────────────────────────────┘              │
│                (memory-mapped register write, ~100 ns)           │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**The honest breakdown:**

| Aspect | Where It Happens | Hardware or Software? |
|---|---|---|
| AI inference (TCN model) | ARM cores (Linux process) | **Software** |
| Decision logic (reserve/forgo) | ARM cores (Linux process) | **Software** |
| Telemetry counter updates | eSwitch ASIC | **Hardware** |
| Token bucket enforcement | eSwitch ASIC | **Hardware** |
| Per-packet rate limiting | eSwitch ASIC | **Hardware** |
| Flow classification (QFI matching) | eSwitch ASIC | **Hardware** |
| Model updates & drift detection | ARM cores (Linux process) | **Software** |
| SLA policy table storage | ARM core memory (DDR5) | **Software** |

**The paper's claim is NOT "everything runs in hardware."** The claim is:

> "The AI decision runs on dedicated software (ARM cores) that is **physically isolated** from the host CPU, and the enforcement runs in actual hardware (eSwitch ASIC) that operates at wire speed independently of any software."

The novelty is the **combination**: dedicated compute for the AI + hardware enforcement for the policy + zero host CPU involvement. Not that everything is "hardware."

---

## Q2: What About GPU + DPU? Can't We Run the Model Better?

### Three Possible Architectures

#### Option A: DPU Only (Your Current Design)

```
┌─────────────────────────┐
│    BlueField-3 DPU      │
│                         │
│  eSwitch ←→ ARM cores   │
│  (enforce)   (AI model) │
│                         │
│  Everything on one card │
└─────────────────────────┘

Inference latency: ~5-15 µs (estimated, INT8 TCN on ARM)
PCIe hops: ZERO
Total loop: ~8-15 µs
```

**Pro:** Absolute minimum latency. Zero PCIe transfer. Self-contained.  
**Con:** ARM cores are weak. Limited to small models (~5K-50K parameters). No transformer, no attention mechanism, no large GRU.

#### Option B: DPU + Separate GPU (GPU in the Server)

```
┌──────────────────────┐     PCIe Gen5 bus      ┌──────────────────┐
│   BlueField-3 DPU    │ ◄──────────────────── ►│   NVIDIA A100    │
│                      │    ~2-10 µs per hop     │   GPU            │
│  eSwitch → telemetry │                         │                  │
│  (enforce)  (collect) │──► feature vector ────►│  AI model runs   │
│            ◄──────────│◄── prediction ◄────────│  HERE            │
│  (apply new rate)     │                        │  (FP16, big      │
│                       │                        │   model OK)      │
└──────────────────────┘                         └──────────────────┘

Inference latency: ~0.1-1 ms (GPU is fast at inference)
PCIe hops: TWO (DPU→GPU, GPU→DPU)
PCIe overhead: ~2-10 µs per hop = 4-20 µs total
Total loop: ~10-30 µs (if small model) to ~100+ µs (if large model)
```

**Pro:** Can run much larger models — transformers, attention, even small LLMs.  
**Con:** Adds PCIe round-trip latency. Defeats the "zero host involvement" claim because the data LEAVES the DPU card. Also requires GPU availability and scheduling.

**The critical problem:** The PCIe hop alone (~4-20 µs round trip) can **double** your total loop time. The whole point of the DPU architecture is eliminating PCIe transfers. Adding a GPU reintroduces the exact bottleneck you're trying to eliminate.

#### Option C: Converged Accelerator (DPU + GPU on Same Card)

```
┌─────────────────────────────────────────────┐
│      NVIDIA Converged Accelerator Card       │
│      (e.g., A100X / A30X / future BF-4+GPU) │
│                                              │
│  ┌─────────────┐   on-card    ┌───────────┐ │
│  │ BlueField   │   PCIe      │  GPU       │ │
│  │ DPU         │◄──switch──► │  (A100/    │ │
│  │             │  (~1-2 µs)  │   A30)     │ │
│  │ eSwitch     │              │            │ │
│  │ ARM cores   │              │            │ │
│  └─────────────┘              └───────────┘ │
│                                              │
│  PCIe hop is ON-CARD (shorter, dedicated)    │
│  Does NOT go through the server's PCIe bus   │
└─────────────────────────────────────────────┘

Inference latency: ~0.1-0.5 ms (GPU is fast)
PCIe hops: TWO, but ON-CARD (much shorter path)
PCIe overhead: ~1-2 µs per hop = 2-4 µs total
Total loop: ~10-20 µs (plausible!)
```

**Pro:** Bigger model than ARM-only, and the on-card PCIe switch avoids the server bus bottleneck. GPUDirect RDMA allows the DPU to write directly into GPU memory.  
**Con:** Expensive. Limited availability. The converged cards (A100X, A30X) were niche products. Also, the GPU has its own scheduling — you'd need to guarantee that your inference kernel isn't queued behind other GPU work.

### Which Should You Choose for the Paper?

| Criterion | DPU Only | DPU + Server GPU | Converged Card |
|---|---|---|---|
| Minimum latency | **Best (~8-15 µs)** | Worst (~30-100+ µs) | Good (~10-20 µs) |
| Model size | Small only (~5K params) | Unlimited | Large OK |
| "Zero host CPU" claim | **Clean** | Broken (GPU = host resource) | Clean (on-card) |
| Hardware availability | BlueField-3 (available now) | BF-3 + any GPU | A100X (discontinued?) |
| Paper narrative | **Strongest** | Weakest | Strong but harder to get |

**Recommendation for your paper: DPU Only (Option A).** Here's why:

1. The paper's central claim is **sub-20 µs, zero-host-CPU control.** Adding a GPU muddies this claim.
2. The TCN model (~5K parameters, INT8) is specifically designed to fit on ARM cores. You chose TCN *because* it's ARM-friendly. If you add a GPU, you'd use a different model — which is a different paper.
3. The converged accelerator (Option C) is a legitimate **future work** section topic: *"On future DPU+GPU converged platforms, our architecture can scale to transformer-based world models while maintaining sub-20 µs loops."*

---

## Q3: Where Does the Packet Pass Through? Where Are Rules Set?

### The Physical Packet Path (Every Step, Every Component)

Let's trace a single GTP-U packet from the moment it hits the DPU's network port:

```
PHYSICAL JOURNEY OF ONE PACKET
═══════════════════════════════

Step 1: ARRIVAL
─────────────────────────────────────────────────────────
  Fiber optic cable → SFP transceiver → SerDes PHY
  │
  │ Electrical signal converted to digital bits
  │ Speed: 400 Gbps wire speed
  ▼

Step 2: ConnectX-7 NIC CORE (still hardware)
─────────────────────────────────────────────────────────
  │ CRC check, frame validation
  │ Ethernet header parsed
  │ Is this an Ethernet frame? → Yes → continue
  ▼

Step 3: eSwitch INGRESS PIPELINE (the key hardware component)
─────────────────────────────────────────────────────────
  The eSwitch has a PIPELINE of match-action stages.
  Think of it like an assembly line in a factory:

  ┌────────────────────────────────────────────────────────┐
  │ STAGE 1: OUTER HEADER MATCH                            │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ Match: dst_MAC, VLAN, outer_IP_src, outer_IP_dst │   │
  │ │ Action: identify which tunnel this belongs to     │   │
  │ └──────────────────────────────────────────────────┘   │
  │                         │                              │
  │ STAGE 2: GTP-U DECAPSULATION                           │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ Match: UDP port 2152 (GTP-U)                     │   │
  │ │ Action: strip outer IP + UDP + GTP headers       │   │
  │ │         extract TEID (Tunnel Endpoint ID)        │   │
  │ │         extract QFI from GTP extension header    │   │
  │ └──────────────────────────────────────────────────┘   │
  │                         │                              │
  │ STAGE 3: FLOW CLASSIFICATION                           │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ Match: QFI value → lookup in 5QI mapping table   │   │
  │ │ Result: slice_id, priority, 5QI class            │   │
  │ │                                                  │   │
  │ │ Example:                                         │   │
  │ │   QFI=5 → 5QI=82 → URLLC slice → priority=0    │   │
  │ │   QFI=1 → 5QI=9  → eMBB slice  → priority=2    │   │
  │ └──────────────────────────────────────────────────┘   │
  │                         │                              │
  │ STAGE 4: HARDWARE COUNTER UPDATE                       │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ Atomically increment (in SRAM, per-slice):       │   │
  │ │   byte_count[slice_id] += packet_length          │   │
  │ │   pkt_count[slice_id]  += 1                      │   │
  │ │   last_ts[slice_id]    = hardware_clock          │   │
  │ │   queue_depth[slice_id] = current_occupancy      │   │
  │ │                                                  │   │
  │ │ These counters are what the ARM cores READ       │   │
  │ │ to build the AI feature vector.                  │   │
  │ └──────────────────────────────────────────────────┘   │
  │                         │                              │
  │ STAGE 5: TOKEN BUCKET CHECK (QoS ENFORCEMENT)          │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ For this packet's slice_id, check:               │   │
  │ │                                                  │   │
  │ │   IF tokens_available >= packet_length:           │   │
  │ │     → PASS (deduct tokens, forward packet)       │   │
  │ │   ELSE:                                          │   │
  │ │     → DROP or QUEUE (depending on priority)      │   │
  │ │                                                  │   │
  │ │ The token bucket refill rate IS the bandwidth    │   │
  │ │ allocation. THIS is what the AI changes.         │   │
  │ │                                                  │   │
  │ │ committed_rate = tokens added per second         │   │
  │ │   200 Mbps = 25,000,000 bytes/sec = 25 tokens/µs│   │
  │ └──────────────────────────────────────────────────┘   │
  │                         │                              │
  │ STAGE 6: FORWARDING                                    │
  │ ┌──────────────────────────────────────────────────┐   │
  │ │ Forward to: host (PCIe), ARM cores, or next port │   │
  │ └──────────────────────────────────────────────────┘   │
  └────────────────────────────────────────────────────────┘

  Total time through pipeline: ~100-500 nanoseconds
  ARM core involvement: ZERO
  Host CPU involvement: ZERO
```

### Where Are the Rules Stored?

```
┌──────────────────────────────────────────────────────────────┐
│                    eSwitch MEMORY MAP                         │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  FLOW TABLE (TCAM / exact-match hash table)          │    │
│  │  ─────────────────────────────────────────────       │    │
│  │  Entry 0: QFI=5, 5QI=82 → slice_id=0 (URLLC)       │    │
│  │  Entry 1: QFI=1, 5QI=9  → slice_id=1 (eMBB)        │    │
│  │  Entry 2: QFI=3, 5QI=69 → slice_id=2 (mIoT)        │    │
│  │  ...up to thousands of entries                       │    │
│  │                                                      │    │
│  │  WHO WRITES THESE: ARM cores via DOCA Flow API       │    │
│  │  WHEN: at slice creation time (not per-packet)       │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  TOKEN BUCKET TABLE (per-slice QoS enforcement)      │    │
│  │  ─────────────────────────────────────────────       │    │
│  │  Slice 0 (URLLC): CIR=200 Mbps, PIR=250 Mbps       │    │
│  │  Slice 1 (eMBB):  CIR=400 Mbps, PIR=500 Mbps       │    │
│  │  Slice 2 (mIoT):  CIR=50 Mbps,  PIR=60 Mbps        │    │
│  │                                                      │    │
│  │  WHO WRITES THESE: ARM cores (AI decision engine)    │    │
│  │  WHEN: every 1-10 ms (after each AI inference)       │    │
│  │  HOW: memory-mapped register write (~100 ns)         │    │
│  │                                                      │    │
│  │  ★ THIS IS THE CRITICAL WRITE ★                      │    │
│  │  When the AI says "increase URLLC to 200 Mbps",      │    │
│  │  it writes "200000000" to this register.              │    │
│  │  The eSwitch immediately starts enforcing the         │    │
│  │  new rate on the VERY NEXT PACKET.                    │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  HARDWARE COUNTERS (per-slice telemetry)             │    │
│  │  ─────────────────────────────────────────────       │    │
│  │  Slice 0: bytes=1,847,293, pkts=12,847, drops=0     │    │
│  │  Slice 1: bytes=48,293,128, pkts=38,471, drops=23   │    │
│  │  Slice 2: bytes=3,847,291, pkts=9,384, drops=0      │    │
│  │                                                      │    │
│  │  WHO WRITES THESE: eSwitch hardware (per-packet)     │    │
│  │  WHO READS THESE: ARM cores (to build AI features)   │    │
│  │  HOW: DMA copy from eSwitch SRAM to ARM memory       │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### The Complete Loop (Putting It All Together)

```
THE FULL CLOSED LOOP — WHERE EVERYTHING HAPPENS
════════════════════════════════════════════════

     HARDWARE WORLD (eSwitch ASIC)              SOFTWARE WORLD (ARM + Linux)
     ─────────────────────────────              ──────────────────────────────

     Packets arrive at 400 Gbps                 
              │                                 
              ▼                                 
     ┌─────────────────────┐                    
     │ Classify by QFI     │                    
     │ Update HW counters  │──── DMA every ──── ► ┌──────────────────────┐
     │ Enforce token bucket │     1-10 ms          │ Read counter snapshot │
     │ Forward packet       │                      │ Build feature vector  │
     └─────────────────────┘                       │ [throughput, pkt_rate,│
              │                                    │  queue_depth, ...]    │
              │ packet continues                   └──────────┬───────────┘
              │ to host/network                                │
              ▼                                                ▼
     (packet is already gone                    ┌──────────────────────┐
      by the time the AI                        │ TCN Model (INT8)     │
      even starts thinking)                     │ Input: 32×5 features │
                                                │ Output: ŷ, σ²        │
                                                │ Time: ~5-15 µs       │
                                                └──────────┬───────────┘
                                                           │
                                                           ▼
                                                ┌──────────────────────┐
                                                │ Decision Engine      │
                                                │ Reserve / Hold / Forgo│
                                                │ Check 5QI constraints│
                                                │ Time: ~1 µs         │
                                                └──────────┬───────────┘
                                                           │
                                                           │ new token
                                                           │ bucket value
                                                           │
     ┌─────────────────────┐                               │
     │ eSwitch token bucket│ ◄──── register write ─────────┘
     │ UPDATED             │       (~100 ns)
     │                     │
     │ NEXT packet gets    │
     │ new rate limit      │
     └─────────────────────┘

     ◄──────────── TOTAL LOOP: ~8-15 µs ────────────────►
```

### Key Insight: The AI Doesn't See Individual Packets

This is important to understand: the AI model does NOT make a decision per-packet. The timeline:

```
Time:  0 µs     100 ns    200 ns    ...    10,000 µs (10 ms)
       │         │         │                │
       packet 1  packet 2  packet 3  ...    AI wakes up,
       arrives   arrives   arrives          reads counter snapshot,
       │         │         │                runs inference,
       ▼         ▼         ▼                writes new token bucket
       
       eSwitch handles     eSwitch handles  ← ARM cores handle
       THESE at wire       THESE at wire       THIS periodically
       speed, per-packet   speed, per-packet
```

In 10 ms at 400 Gbps, roughly **3.3 million packets** pass through the eSwitch. The AI runs ONCE and updates the token bucket. The eSwitch then enforces that new rate on the next 3.3 million packets.

**The AI is the brain that decides the rules. The eSwitch is the muscle that enforces them — on every packet, at wire speed, without waiting for the brain.**

---

## Summary: What You Should Say in Your Paper

Do NOT say: *"We run slice management in hardware."*

DO say: *"We propose a DPU-resident architecture where AI-driven slice decisions execute on dedicated ARM cores physically isolated from the host CPU, and the resulting QoS policies are enforced by the DPU's eSwitch ASIC at hardware line-rate (<100 ns per-packet enforcement), with zero host CPU involvement in the entire telemetry-inference-actuation loop."*

This is precise, defensible, and correctly distinguishes what is hardware (eSwitch enforcement) from what is software-on-dedicated-hardware (AI inference on ARM cores).

---

*Prepared June 2026. Hardware specifications from: NVIDIA BlueField-3 datasheet, Hot Chips 2023 presentation, DOCA SDK 2.x documentation, PNY product specifications.*
