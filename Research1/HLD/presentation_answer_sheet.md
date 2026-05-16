# NaaS 5G UPF — Presentation Answer Sheet

Prepared for your presentation. Each answer is structured as: **the question → a clear, confident answer → a one-liner you can say out loud.**

---

## Q5. Why WebAssembly over native code (like a .so shared library)?

If you just wanted to run custom logic, you could let tenants upload a compiled `.so` (shared library) and `dlopen()` it into the Rust daemon. It would be faster. So why Wasm?

**Three reasons:**

### A. Safety / Sandboxing
A native `.so` runs with the **same privileges as your Rust daemon**. A bug (or a malicious actor) in that shared library can:
- Corrupt the daemon's memory (including the AF_XDP UMEM, crashing the entire data plane).
- Call arbitrary system calls (read files, open network connections, escalate privileges).
- Segfault and crash the entire process, killing all tenant processing.

WebAssembly runs in a **sandboxed linear memory**. It literally *cannot* address memory outside its own sandbox. It has no access to system calls, the filesystem, or the network unless you explicitly grant it via the host ABI. A buggy Wasm plugin can only hurt itself.

### B. Portability
A `.so` is compiled for a specific CPU architecture (x86_64, ARM64) and a specific OS (Linux). If the telecom runs mixed hardware, they need multiple builds.

A `.wasm` binary is architecture-independent. Compile once, run anywhere — x86 server, ARM edge node, any OS. The Wasm runtime (WasmEdge) handles the translation.

### C. Deterministic Resource Control
With native code, you cannot easily limit how many CPU instructions a function executes. You can set a wall-clock timeout, but that's coarse and unreliable.

With Wasm, the runtime supports **fuel metering** — you can set a precise instruction budget (e.g., 500,000 instructions). The runtime counts every instruction and terminates execution the moment the budget is exhausted. This is impossible to do with native `.so` files without OS-level hacks.

> **Say this:** *"Native shared libraries are faster, but they run with full process privileges. A single buggy tenant plugin could crash our entire data plane. WebAssembly gives us a strict sandbox with deterministic resource limits — the tenant's code literally cannot see or touch anything outside its own memory."*

---

## Q10. What is the drain window? What happens if a v1 invocation outlasts it?

This question is really about three interconnected safety mechanisms. Let me explain each and how they work together.

### The Drain Window
When you upgrade from plugin v1 to v2, the atomic BPF map pointer swap happens instantly — the *very next packet* goes to v2. But what about packets that were *already inside v1* when the swap happened?

The **drain window** (default: 100ms) is a grace period. After the swap, v1 is kept alive for 100ms to let any in-flight invocations finish. The Rust daemon tracks in-flight invocations via an atomic counter. Once the counter hits zero AND the window has elapsed, v1 is safely unloaded.

### Fuel Metering
Each Wasm invocation gets a fixed **instruction budget** (e.g., 500,000 instructions). The Wasm runtime counts every instruction. If the plugin enters an infinite loop or is just too computationally expensive, it burns through its fuel and the runtime kills it. The Rust daemon catches the error, treats it as `PASS` (fail-open), and forwards the packet unchanged.

This is an **internal sandbox mechanism** — it measures computation, not wall-clock time.

### Execution Timeout (Watchdog)
The watchdog is a **wall-clock timer** (default: 500μs) running in the Rust daemon, independent of the Wasm runtime. It exists as a second line of defence. If fuel metering somehow fails to stop the execution (e.g., a runtime bug), the watchdog fires, kills the invocation, releases the packet as `PASS`, and logs an anomaly.

### How They Work Together

```
Packet arrives → Wasm invocation starts
  │
  ├─ Normal case: Plugin finishes in ~10μs → verdict returned → done
  │
  ├─ Bad plugin (infinite loop): 
  │     Fuel runs out at ~50μs → runtime kills it → PASS (fail-open)
  │
  ├─ Catastrophic case (runtime bug, fuel doesn't work):
  │     Watchdog fires at 500μs → daemon kills it → PASS (fail-open)
  │
  └─ During upgrade (v1 still processing):
        Drain window (100ms) keeps v1 alive
        100ms >> 500μs timeout, so v1 invocations always finish
        If somehow they don't → v1 is force-killed → at most 1 packet lost per TEID
```

### The Math Behind the Drain Window
The default drain window is **100ms**. The worst-case execution time for a single Wasm invocation is **500μs** (the watchdog timeout). That means:

```
100ms / 500μs = 200× safety margin
```

Even if a plugin takes the absolute maximum time, the drain window is 200 times longer. This is why the HLD states a "2000× safety margin" — assuming realistic P99.9 execution of ~50μs rather than the hard 500μs cap.

> **Say this:** *"Fuel metering prevents infinite loops inside the sandbox. The watchdog prevents the sandbox itself from hanging. And the drain window ensures that during a live upgrade, in-flight invocations under the old plugin version always have enough time to finish. These three mechanisms layer on top of each other — belt, suspenders, and a safety net."*

---

## Q11. You do a BPF map lookup on every packet. At 10M pps, isn't that significant?

This is a great challenge, but the answer is firmly no.

**`BPF_MAP_TYPE_HASH` is a kernel hash table with O(1) average-case lookup time.** The key is a 4-byte TEID (a simple `__u32` integer). Hashing a 4-byte integer and checking one hash bucket is an operation that takes roughly **50–100 nanoseconds**.

At 10 million packets per second:
```
10,000,000 pps × 100ns = 1 second of CPU time per second
```

That sounds like a lot, but a modern server has 16–64 CPU cores. One core can handle ~10M lookups/sec on its own. And with RSS (Receive Side Scaling), the NIC distributes packets across multiple cores, so each core handles a fraction of the total.

For context, **eUPF** (an existing open-source eBPF-based UPF) already does this exact pattern — BPF map lookup on every packet — and achieves multi-gigabit throughput in production.

Additionally, the BPF map lookup happens at the **XDP hook, before `sk_buff` allocation**. The cost of `sk_buff` allocation (~300–500ns) dwarfs the map lookup cost. So even on the fast path, the XDP lookup adds negligible overhead compared to what the kernel does immediately after.

> **Say this:** *"The BPF hash map lookup is O(1) and takes roughly 50–100 nanoseconds for a 4-byte key. At 10 million packets per second, that's trivially handled by a single CPU core. This is a well-proven pattern — eUPF already does it in production at multi-gigabit rates."*

---

## Q12. Why not DPDK? It's faster.

Yes, DPDK is faster in raw throughput. But speed is not the only design criterion. Here's why XDP was chosen:

### 1. CPU Allocation
DPDK requires **dedicated CPU cores** running poll-mode drivers at 100% utilization, even when there are zero packets to process. In a Kubernetes cluster, this conflicts with the scheduler — K8s expects to manage CPU resources, but DPDK cores are invisible to it.

XDP is **event-driven**. It only consumes CPU when packets actually arrive. It coexists with the K8s resource model.

### 2. Kernel Coexistence (The Killer Reason)
Your architecture's entire value proposition is **augmenting existing UPFs without modification**. XDP can sit in front of Open5GS and silently intercept packets. The UPF doesn't even know XDP exists.

DPDK **takes over the NIC entirely**. The kernel loses access. You would have to rewrite the entire UPF to be a DPDK application. That defeats the purpose.

### 3. BPF Map Accessibility
The hitless upgrade mechanism depends on `bpf_map_update_elem()` — atomically swapping pointers in a kernel BPF map. This is a native kernel operation.

With DPDK, there are no BPF maps. You would need to invent a custom IPC mechanism for atomic state updates between the control plane and the data plane, which is significantly more complex and error-prone.

### 4. The 99/1 Split Makes DPDK Overkill
Your XDP program handles 99% of traffic with a single hash lookup and `XDP_PASS`. Only 1% goes through the exception path. The bottleneck is the Wasm execution, not the packet interception. Using DPDK to make the interception faster doesn't help when the slow part is downstream.

> **Say this:** *"DPDK is faster, but it requires exclusive NIC ownership and dedicated CPU cores. Our architecture's key value is augmenting existing UPFs without modification — XDP lets us do that. DPDK would force us to rewrite the entire UPF, which is exactly the vendor lock-in problem we're trying to solve."*

---

## Q13. Most traffic is HTTPS (encrypted). Doesn't that make your DPI useless?

This is the most common pushback you'll get. Here's the honest, layered answer:

### What you CANNOT do
You cannot read or modify the encrypted L7 payload of HTTPS/TLS traffic. Pattern matching on ciphertext is meaningless. The `PASS_MOD` verdict (inline payload modification) is effectively useless on encrypted payloads.

### What you CAN still do (and it's a lot)

| Visible Metadata | Use Case |
|:---|:---|
| **TEID** | Always in the clear (GTP-U header). DROP, METER, TAG all work. |
| **TLS SNI** | During TLS handshake, the server name (e.g., `youtube.com`) is sent in **plaintext**. Your plugin can classify traffic by destination service. |
| **Inner 5-tuple** | Source/Dest IP, ports, protocol. Port 443 = HTTPS, port 53 = DNS, etc. |
| **Packet size distributions** | Video streaming vs. IoT telemetry have very different patterns, even encrypted. |
| **DNS queries** | If not using DoH/DoT, DNS is plaintext. You see every domain the user resolves. |

### The Enterprise/IoT Angle
Many enterprise and IoT protocols are **not encrypted**. Industrial sensors, MQTT telemetry, legacy SCADA — these often transmit in plaintext over private 5G networks. This is a primary target market for your architecture.

### The Broader Truth
This limitation applies to **every DPI system in existence**, including million-dollar commercial UPFs from Ericsson and Nokia. It's not a weakness of your architecture — it's a fundamental property of encryption.

> **Say this:** *"This is not a limitation of our architecture — it's a limitation of encryption itself. No DPI system can inspect encrypted payloads without key access. However, our system still provides full TEID-level control, TLS SNI-based traffic classification, and metadata-based analysis. And for enterprise private 5G and IoT deployments — which are our primary target — many protocols are still unencrypted."*

---

## Q14. How is this different from Istio's Proxy-Wasm? Aren't you reinventing the wheel?

This is a critical differentiation question. The short answer: **they operate at completely different layers of the network and cannot do each other's jobs.**

| Dimension | Proxy-Wasm (Envoy/Istio) | Your Architecture |
|:---|:---|:---|
| **Where it intercepts** | L7 HTTP proxy in user-space | L2/L3 XDP hook in the kernel driver |
| **What it sees** | HTTP requests and responses | Raw GTP-U encapsulated packets |
| **Protocol awareness** | HTTP/gRPC | GTP-U / TEID |
| **Upgrade speed** | Envoy listener drain: ~seconds | BPF map pointer swap: ~nanoseconds |
| **Use case** | Service mesh traffic management | Telecom data plane packet processing |

### The fundamental gap
Envoy/Proxy-Wasm sits as an HTTP proxy. It sees clean, decoded HTTP headers and bodies. It has **zero awareness of GTP-U tunneling**. If you point Envoy at a 5G UPF's N3 interface, it would see UDP packets on port 2152 containing binary GTP-U data. It would have no idea what to do with them.

Your system cracks open the GTP-U tunnel at the kernel level, extracts the TEID, and hands the inner L7 payload to the Wasm plugin. Proxy-Wasm simply cannot do this.

### They're complementary, not competing
Proxy-Wasm handles east-west HTTP traffic between microservices. Your system handles north-south GTP-U traffic from the radio network. A telecom could use both simultaneously.

> **Say this:** *"Proxy-Wasm operates as an L7 HTTP proxy — it understands HTTP headers and gRPC. Our system operates at L2/L3 inside the kernel, processing raw GTP-U tunneled packets. Envoy cannot parse GTP-U headers or extract TEIDs. These are complementary systems targeting completely different protocol layers."*

---

## Q16. veth reinjection goes through the full kernel stack. Doesn't that defeat the purpose of XDP?

This is a fair concern, and the answer is about understanding **which traffic pays this cost.**

### The cost is real
Yes, when a modified packet (`PASS_MOD`) is reinjected via the veth pair, it traverses the full kernel networking stack on the receiving side — qdisc, Netfilter, IP routing. This adds roughly **2–5 microseconds** per packet.

### But only 1% of traffic pays it
The veth cost applies **only** to exception-path packets — the ~1% of traffic that needed custom Wasm processing. The 99% fast-path traffic never touches the veth. It goes through `XDP_PASS` directly to the normal UPF.

### The alternative is much worse
To avoid the veth overhead, you would need the UPF itself to have an AF_XDP receive socket, so you could inject packets directly into it without going through the kernel stack. But that requires **modifying the UPF's source code** — which breaks your "augment without modification" guarantee.

### Perspective on the 2–5μs cost
The Wasm execution itself takes ~10–50μs. The AF_XDP transfer takes ~1–2μs. The veth overhead (2–5μs) is a **small fraction of the total exception-path latency** (~15–60μs total). And this total is still orders of magnitude faster than the alternative: waiting months for a vendor binary update.

```
Exception-path budget breakdown (approximate):
  XDP → AF_XDP transfer:     ~1–2μs
  Payload copy into Wasm:     ~0.5–1μs
  Wasm DPI execution:         ~10–50μs
  Payload copy back:          ~0.5–1μs
  veth reinjection:           ~2–5μs    ← this is the "cost"
  ─────────────────────────────────────
  Total:                      ~15–60μs per exception-path packet
```

> **Say this:** *"The veth overhead is real — about 2 to 5 microseconds per packet. But it only affects the 1% of traffic on the exception path. For that 1%, the total end-to-end latency is around 15 to 60 microseconds, of which veth is a small fraction. It's a pragmatic Phase 1 trade-off that lets us work with unmodified UPFs. Phase 2 explores direct AF_XDP reinjection to eliminate it."*

---

## Q17. Single-threaded Rust daemon — how does this scale?

### Phase 1 is not about scale. It's about correctness.
Phase 1 uses a single-threaded Rust daemon with a single AF_XDP RX queue deliberately. The goal is to **isolate variables** for accurate measurement of:
- Per-packet latency (without thread contention noise)
- Hitless switchover correctness (without race conditions across threads)
- Fuel metering overhead (without scheduling jitter)

If you run multi-threaded from day one, and you see a latency spike, you can't tell if it's from your Wasm execution, from thread contention, or from CPU cache misses across cores. Single-threaded eliminates all of those variables.

### Phase 2 scaling strategy
The path to production scale is well-understood and uses **RSS (Receive Side Scaling) + multi-queue AF_XDP**:

```
Phase 1 (Research):
  NIC → 1 RX Queue → 1 AF_XDP socket → 1 Rust thread → 1 Wasm instance

Phase 2 (Production):
  NIC → N RX Queues (RSS distributes by flow hash)
    ├─ Queue 0 → AF_XDP socket 0 → Rust thread 0 → Wasm instance 0
    ├─ Queue 1 → AF_XDP socket 1 → Rust thread 1 → Wasm instance 1
    ├─ Queue 2 → AF_XDP socket 2 → Rust thread 2 → Wasm instance 2
    └─ ...
```

Each thread is pinned to a CPU core and handles its own queue independently. There is **no shared state between threads** (each has its own AF_XDP socket and Wasm instance), so there is no locking and near-linear scaling.

This is the exact same scaling model that DPDK uses (one core per queue), but without requiring exclusive NIC ownership.

> **Say this:** *"Phase 1 is single-threaded by design — we need to isolate variables for accurate latency measurement. The scaling path is RSS multi-queue AF_XDP with per-thread Wasm instances. Each thread is independent with no shared state, so scaling is near-linear with core count. This is the standard high-performance networking pattern."*

---

## Q18. If the Rust daemon crashes, who restarts it and what happens in the meantime?

This is a fault recovery question, and your architecture handles it with **three layers of defence**:

### Layer 1: eBPF Health Flag (Immediate, automatic)
The Rust daemon periodically writes a "heartbeat" into a `BPF_MAP_TYPE_ARRAY` (a simple kernel map). On every exception-path packet, the eBPF XDP program checks this flag:
- Flag is set → daemon is alive → redirect to AF_XDP as normal.
- Flag is unset → daemon is dead → return `XDP_PASS` instead of `XDP_REDIRECT`.

This means **the instant the daemon crashes**, exception-path packets automatically fall through to the normal UPF processing. No packets are lost — they just don't get custom DPI processing temporarily. The 99% fast-path is completely unaffected (it was always `XDP_PASS`).

### Layer 2: BPF Map Persistence (State survives the crash)
BPF maps are **pinned to `bpffs`** (the BPF filesystem). When the daemon crashes, the maps — including the TEID exception map, the blocklist, and all per-TEID counters — survive on disk. When the daemon restarts, it reattaches to the existing pinned maps and picks up exactly where it left off. No state is lost.

### Layer 3: Sidecar Restarts the Daemon
The per-pod Sidecar Agent maintains a **gRPC heartbeat** with the Rust daemon. When the heartbeat fails:
1. Sidecar detects the death.
2. Sidecar restarts the Rust daemon process.
3. The new daemon instance re-attaches to the pinned BPF maps.
4. The daemon re-loads the last known Wasm plugin from the shared EmptyDir volume.
5. The daemon sets the health flag in the BPF array map.
6. eBPF sees the flag, resumes redirecting exception-path packets.

### Timeline
```
T=0ms:     Rust daemon crashes
T=0ms:     eBPF health flag expires → exception packets fall through as XDP_PASS
           (automatic, no intervention needed)
T=~100ms:  Sidecar detects heartbeat failure
T=~200ms:  Sidecar restarts daemon
T=~300ms:  Daemon re-attaches to pinned BPF maps, reloads Wasm plugin
T=~350ms:  Daemon sets health flag → exception-path processing resumes
```

Total downtime for exception-path processing: **~350ms**. Fast-path is never affected. No sessions are dropped.

> **Say this:** *"The eBPF program checks a health flag on every exception-path packet. The instant the daemon crashes, packets automatically fall through to normal UPF processing — no drops. BPF maps are pinned to bpffs and survive the crash. The Sidecar detects the failure, restarts the daemon, and it reattaches to the existing maps. Total recovery time is under 500 milliseconds, and the fast path is never affected."*

---

## Q19. Where do you see this in 3 years?

### Near-term (1 year): Production-grade single-operator deployment
- Multi-queue RSS scaling for 25/100G NICs.
- PFCP integration for dynamic TEID discovery (no more static maps).
- Multi-slice concurrent tenant isolation validated at scale.
- Plugin marketplace: telecom operators can browse and deploy pre-built Wasm plugins (DPI signatures, billing modules, security filters) from an OCI registry.

### Mid-term (2 years): Industry standard for programmable UPFs
- Standardisation of the telecom Wasm ABI (PASS/DROP/TAG/METER) as an open specification.
- Adoption by open-source 5G cores (Open5GS, free5GC, OAI) as an official extension mechanism.
- Integration with O-RAN: the same Wasm plugin model could extend to RAN Intelligent Controllers (RICs), creating a unified programmability layer from RAN to core.

### Long-term (3 years): Programmable telecom infrastructure
- The UPF becomes a **platform**, not a product. Operators compete on the quality of their plugin ecosystem, not on proprietary firmware.
- Enterprise tenants self-manage their network slice logic through Kubernetes CRDs — true Network-as-a-Service.
- Extension to other network functions beyond UPF (AMF, SMF policy enforcement).

> **Say this:** *"In 3 years, I see the UPF transforming from a rigid vendor product into a programmable platform. Operators would deploy plugins from a marketplace the same way you install apps on a phone. The same Wasm plugin model could extend across the entire telecom stack — from RAN to core — creating a unified programmability layer for 5G and beyond."*

---

## Q20. Could this approach work beyond 5G — in edge computing or SD-WAN?

**Absolutely yes.** The core architecture — eBPF fast-path filtering + Wasm sandboxed processing + Kubernetes-driven lifecycle — is **not inherently tied to 5G or GTP-U**. The 5G UPF is just the first application.

### Edge Computing
Edge nodes process traffic from IoT devices, cameras, and sensors. They need:
- Custom filtering rules per device type (your eBPF TEID lookup → device ID lookup).
- Lightweight, sandboxed processing (your Wasm DPI → edge inference pre-processing).
- Dynamic updates without downtime (your hitless upgrade → live model swaps).

An edge gateway with eBPF + Wasm could dynamically process IoT telemetry at wire speed without shipping data to the cloud.

### SD-WAN
SD-WAN appliances route traffic between branch offices. They need:
- Application-aware routing (identify app by DPI → route to optimal WAN link).
- Per-tenant policies (enterprise A wants Teams traffic on MPLS, enterprise B wants it on broadband).
- Live policy updates without rebooting the appliance.

Your architecture maps directly: eBPF classifies traffic, Wasm applies tenant-specific routing policies, K8s Operator pushes policy updates.

### CDN / Load Balancers
Content Delivery Networks need to inspect HTTP traffic and apply custom caching/routing rules. Wasm plugins could implement custom cache key logic or A/B testing rules at the edge, updated live via CRDs.

### The Generalised Pattern
```
Any system that needs:
  1. High-speed packet filtering     → eBPF/XDP
  2. Custom, tenant-specific logic   → Wasm sandbox
  3. Live updates without downtime   → Atomic BPF map swap + drain
  4. Orchestration at scale          → Kubernetes CRDs

...can use this architecture.
```

> **Say this:** *"The 5G UPF is our first target, but the pattern is universal. Any network function that needs high-speed filtering with dynamically injectable custom logic — edge gateways, SD-WAN appliances, CDN nodes — could adopt the same eBPF + Wasm + Kubernetes architecture. We chose 5G because the vendor lock-in pain is most acute there, but the abstraction generalises."*

---

## Deep Dive: Encryption, SNI, and What You Can Do With Encrypted Traffic

This section goes beyond the Q13 answer and gives you a thorough understanding of how encryption works at the packet level and exactly what your architecture can still accomplish.

---

### How TLS Encryption Actually Works (What's Visible vs. What's Hidden)

When a user on the 5G network opens `https://youtube.com`, the traffic goes through **two layers of encapsulation** before reaching your UPF:

```
┌─────────────────────────────────────────────────────────────┐
│ GTP-U Header                                                │
│  ├─ TEID: 12345              ← ALWAYS visible (Layer 4)     │
│  ├─ GTP-U flags, sequence#                                  │
│  │                                                           │
│  └─ Inner IP Packet                                          │
│      ├─ IP Header                                            │
│      │   ├─ Src IP: 10.0.0.5     ← visible (not encrypted)  │
│      │   └─ Dst IP: 142.250.190.46  ← visible               │
│      ├─ TCP Header                                           │
│      │   ├─ Src Port: 49821     ← visible                    │
│      │   └─ Dst Port: 443       ← visible (tells us HTTPS)  │
│      │                                                       │
│      └─ TLS Record                                           │
│          ├─ TLS Handshake (first few packets)                │
│          │   └─ ClientHello                                  │
│          │       └─ SNI: "youtube.com"  ← VISIBLE (plaintext)│
│          │                                                   │
│          └─ Application Data (after handshake)               │
│              └─ 0x17 03 03 ... [encrypted blob]  ← OPAQUE   │
│                  You see random bytes. No patterns.          │
│                  Cannot read, search, or modify.             │
└─────────────────────────────────────────────────────────────┘
```

**Key insight:** Encryption protects the *application data* (the HTTP request body, video stream bytes, etc.), but the **network headers** and the **TLS handshake metadata** are still in the clear.

---

### What is SNI (Server Name Indication) — In Depth

**The Problem SNI Solves:**
A single server (e.g., IP `142.250.190.46`) can host hundreds of different websites (youtube.com, google.com, gmail.com). When a client connects via HTTPS, the server needs to know *which* website the client wants so it can present the correct TLS certificate. But the HTTP `Host:` header is inside the encrypted payload — the server can't read it yet because encryption hasn't been set up.

**The Solution:**
During the TLS handshake, the very first message the client sends is called the **ClientHello**. Inside this message, there is a field called the **Server Name Indication (SNI)** that contains the target hostname **in plaintext**.

```
ClientHello message (sent BEFORE encryption is established):
  ├─ TLS Version: 1.3
  ├─ Cipher Suites: [list of supported encryption methods]
  ├─ Random: [32 bytes of random data]
  └─ Extensions:
      └─ server_name: "youtube.com"    ← THIS IS PLAINTEXT
```

**Why is SNI plaintext?**
Because it's a chicken-and-egg problem. You need the server name to set up encryption, but encryption hasn't started yet. So the server name must be sent unencrypted.

**What your Wasm plugin can do with SNI:**
Your plugin sees the raw TLS ClientHello inside the GTP-U payload. It can parse out the SNI field and:
- **Classify traffic:** "This session is accessing youtube.com" → TAG it for video QoS.
- **Block domains:** "This session is accessing gambling-site.com" → DROP.
- **Billing:** "This session is accessing a zero-rated partner" → METER with special billing tag.
- **Per-app analytics:** Count how many sessions go to each service, per TEID/user.

**Important caveat — ECH (Encrypted Client Hello):**
TLS 1.3 introduced an extension called **Encrypted Client Hello (ECH)**, which encrypts the SNI field too. If ECH is widely deployed, SNI-based classification breaks. However:
- ECH adoption is still very early (as of 2026, most traffic still exposes SNI).
- ECH requires DNS-over-HTTPS (DoH) to distribute the encryption key, so networks that control DNS resolution can still see the target domain.
- Enterprise private 5G networks can mandate TLS configurations on managed devices, preventing ECH.

---

### Concrete Use Cases That Work Even With Fully Encrypted Payloads

Even if every single byte of L7 data is encrypted, your architecture is far from useless. Here are real, production-relevant use cases:

#### 1. Per-TEID Traffic Metering (Billing)
**What:** Count bytes per user session. Every packet has a TEID in the GTP-U header, which is never encrypted.
**Wasm Verdict:** `METER` — increment a per-TEID byte counter in the BPF array map.
**Business Value:** Custom billing per session, per slice, or per enterprise tenant. The telecom can charge differently for IoT device traffic vs. smartphone traffic based on the TEID-to-slice mapping.

#### 2. SNI-Based Application Classification
**What:** Read the SNI from the TLS ClientHello to identify which application/service a session is connecting to.
**Wasm Verdict:** `TAG` — write a 1-byte classification tag into the GTP-U extension header.
**Business Value:** Zero-rating ("Free Netflix data"), per-app QoS enforcement, regulatory compliance (blocking restricted domains), parental controls.

#### 3. Volumetric Anomaly Detection (DDoS / Abuse)
**What:** Track per-TEID packet rates and byte volumes. If a single TEID suddenly starts generating 100× its normal traffic volume, it's likely compromised or abusive.
**Wasm Verdict:** `DROP` (adds TEID to eBPF blocklist) or `TAG` (flag for further investigation).
**Business Value:** Automated abuse mitigation without touching the content. Works entirely on metadata. A compromised IoT device flooding the network gets auto-blocked.

#### 4. Session-Level Access Control
**What:** Enforce which TEIDs are allowed to send traffic at all. An enterprise tenant may revoke access for a specific device instantly.
**Wasm Verdict:** `DROP`.
**Business Value:** Real-time device quarantine. If a factory detects a compromised sensor, they push a CRD update, the Wasm plugin starts DROPping that TEID's traffic, and the eBPF blocklist prevents future packets from even reaching user-space.

#### 5. Traffic Pattern Fingerprinting (Encrypted Traffic Analysis)
**What:** Even encrypted flows have distinctive patterns. Video streaming produces large, regularly spaced packets. VoIP produces small, constant-rate packets. Web browsing produces bursty, variable-size packets. Your Wasm plugin can classify traffic by analysing:
- Packet size distribution
- Inter-arrival timing
- Flow duration
- Burst patterns
**Wasm Verdict:** `TAG` with a classification label.
**Business Value:** Application-aware QoS without decryption. Prioritise VoIP packets over bulk downloads, even when both are encrypted.

#### 6. 5-Tuple Based Micro-Segmentation
**What:** Even with encrypted payloads, the inner IP header and TCP/UDP header are visible (unless IPsec tunnel mode is used, which is rare in mobile networks). You can enforce rules like:
- "TEID 12345 is only allowed to communicate with IP range 10.0.0.0/24"
- "Block all traffic from this TEID to port 22 (SSH)"
**Wasm Verdict:** `DROP` or `PASS`.
**Business Value:** Network micro-segmentation for enterprise slices. A factory's IoT devices should only talk to the factory's cloud, not to the public internet.

#### 7. GTP-U Header-Level Operations (Always Work)
**What:** Your plugin can manipulate the GTP-U extension headers themselves without touching the payload. The `TAG` verdict writes a QoS marking byte into the GTP-U extension header.
**Wasm Verdict:** `TAG`.
**Business Value:** Downstream network elements (routers, firewalls) can read these tags and apply differentiated treatment — all without ever inspecting the encrypted content.

---

### Summary: What Works vs. What Doesn't

| Capability | Cleartext Payload | Encrypted Payload |
|:---|:---:|:---:|
| TEID-based DROP/BLOCK | ✅ | ✅ |
| Per-TEID byte counting (METER) | ✅ | ✅ |
| GTP-U extension header tagging (TAG) | ✅ | ✅ |
| SNI-based app classification | ✅ | ✅ (until ECH) |
| 5-tuple based filtering | ✅ | ✅ |
| Traffic pattern fingerprinting | ✅ | ✅ |
| Volumetric anomaly / DDoS detection | ✅ | ✅ |
| Deep L7 content inspection (regex, signatures) | ✅ | ❌ |
| Inline payload modification (PASS_MOD) | ✅ | ❌ |
| Protocol-specific parsing (HTTP headers, etc.) | ✅ | ❌ |

> **Say this:** *"Encryption blocks L7 content inspection, but 7 out of 10 of our core use cases work perfectly with encrypted traffic. TEID operations, SNI classification, traffic fingerprinting, and volumetric anomaly detection all operate on metadata that is never encrypted. Our architecture is not a DPI-only tool — it's a programmable packet processing platform."*

---

## Quick Reference: One-Liner Cheat Sheet

| Q# | If asked... | Say this (one sentence) |
|:---|:---|:---|
| 5 | Why Wasm over native? | *"Wasm gives us a strict sandbox — a tenant's code cannot see or touch anything outside its own memory, unlike a native shared library."* |
| 10 | Drain window? | *"Belt, suspenders, and a safety net — fuel metering stops infinite loops, the watchdog stops hangs, and the drain window ensures in-flight packets finish during upgrades."* |
| 11 | BPF map lookup overhead? | *"O(1) hash lookup on a 4-byte key — about 50 to 100 nanoseconds. eUPF already does this in production at multi-gigabit rates."* |
| 12 | Why not DPDK? | *"DPDK requires exclusive NIC ownership. We'd have to rewrite the UPF — which is the vendor lock-in we're solving."* |
| 13 | Encrypted traffic? | *"No DPI system can inspect encrypted payloads. We still provide TEID-level control, SNI classification, and target unencrypted IoT/enterprise protocols."* |
| 14 | vs. Proxy-Wasm? | *"Proxy-Wasm is an L7 HTTP proxy. We operate at L2/L3 on raw GTP-U packets. Envoy cannot parse TEIDs. They're complementary."* |
| 16 | veth overhead? | *"2–5μs, but only on the 1% exception path. It's a pragmatic Phase 1 trade-off for working with unmodified UPFs."* |
| 17 | Single-threaded scaling? | *"Deliberate for Phase 1 measurement isolation. Phase 2 uses RSS multi-queue with per-thread Wasm instances — near-linear scaling."* |
| 18 | Daemon crash? | *"eBPF auto-falls-through to XDP_PASS. BPF maps survive on bpffs. Sidecar restarts the daemon in ~350ms. No sessions drop."* |
| 19 | 3-year vision? | *"The UPF becomes a programmable platform with a plugin marketplace, not a rigid vendor product."* |
| 20 | Beyond 5G? | *"The pattern — eBPF filtering + Wasm logic + K8s orchestration — generalises to edge, SD-WAN, and any programmable network function."* |
