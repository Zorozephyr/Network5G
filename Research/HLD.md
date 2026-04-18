High-Level Design (HLD): Hitless eBPF & WebAssembly Data Plane Orchestration for 5G Core Networks
1. Executive Summary & Objective
Current 5G User Plane Functions (UPFs) suffer from rigid data planes. Introducing custom Deep Packet Inspection (DPI), proprietary telemetry, or zero-day threat mitigation requires restarting containerized UPFs, resulting in dropped sessions and SLA violations.

This architecture proposes a vendor-agnostic framework that leverages eBPF (Extended Berkeley Packet Filter) for line-rate packet pre-filtering and WebAssembly (Wasm) for secure, Turing-complete payload execution in user space. A custom Go-based Kubernetes Operator manages the dynamic lifecycle, enabling hitless injection and atomic swapping of network logic without disrupting the active 5G fast-path.

2. High-Level Architecture (The Three Pillars)
The system is decoupled into three strict domains to isolate state, speed, and orchestration.

2.1 The Data Plane (eBPF XDP Fast-Path)
Packet Interception: eBPF programs attach at the XDP (eXpress Data Path) layer to intercept all inbound GTP-U traffic before the Linux networking stack allocates memory (sk_buff).

TEID Pre-Filtering: eBPF performs an O(1) lookup in a pinned Hash Map (BPF_MAP_TYPE_HASH) using the GTP-U Tunnel Endpoint Identifier (TEID).

Fast-Path (99%): If no match is found, XDP immediately passes the packet to the standard UPF.

Exception-Path (1%): If matched, the packet is flagged for Wasm processing.

The Metadata Handoff (Bypassing the Encapsulation Tax): To prevent Wasm from wasting CPU cycles parsing nested telecom headers (Ethernet > IP > UDP > GTP-U), the eBPF program calculates the exact byte offset of the L7 payload. It passes a metadata wrapper [TEID, Payload_Offset] alongside the packet to user space.

2.2 The Execution Environment (WebAssembly Runtime)
The Rust Daemon: A high-performance Rust process runs alongside the Wasm runtime (e.g., WasmEdge) inside the UPF pod. It handles low-level system interactions (memory allocation, IPC, eBPF map updates) on behalf of the sandboxed Wasm module.

Turing-Complete DPI: Wasm executes the custom logic (e.g., string matching for malware, proprietary protocol parsing) safely. Fuel/Gas metering is strictly enforced to prevent memory leaks or infinite loops from crashing the pod.

2.3 The Control Plane (Intent-Driven Kubernetes Orchestration)
To separate cluster-wide orchestration from node-level latency, the control plane uses a Master/Sidecar pattern.

Central Go Controller: A centralized operator watches for WasmPlugin Custom Resource Definitions (CRDs). It identifies the target UPF pods and patches their metadata annotations with the desired state (e.g., plugin-version: v2).

UPF Pod Sidecar: A lightweight Go container running inside the UPF pod monitors local annotations. Upon detecting a change, it pulls the .wasm binary from an OCI registry to a shared EmptyDir volume and triggers the Rust Daemon via local gRPC over a Unix Domain Socket to load the new module.

3. Architectural Deployment Models (The 3 Approaches)
Depending on the specific enterprise use case, the handoff between eBPF and Wasm can be configured in three distinct ways.

Approach 1: Out-of-Band Analysis (Asynchronous)
Mechanism: eBPF XDP uses a standard BPF_MAP_TYPE_RINGBUF. It sends a copy of the packet to Wasm and immediately forwards the original packet to the UPF (XDP_PASS). Wasm analyzes the copy asynchronously. If malicious, Wasm instructs the Rust Daemon to update an eBPF Blocklist map to drop future packets from that source.

Primary Use Case: Zero-Day Threat Mitigation and Security Intrusion Detection.

Pros: Zero latency overhead on the 5G fast-path. Safest deployment model. Easiest to implement.

Cons: "Fail Open" by nature. Because analysis is asynchronous, the very first malicious packet will successfully traverse the network before the blocklist is updated.

Approach 2: Bump-in-the-Wire (Inline Service Function Chaining)
Mechanism: eBPF XDP uses XDP_REDIRECT to push packets into a zero-copy AF_XDP socket. Wasm reads the shared memory and modifies the payload in place. The Rust Daemon then injects the modified packet directly into the UPF container's network namespace via a veth (Virtual Ethernet) pair.

Primary Use Case: Proprietary protocol translation, custom header stripping, and inline enterprise billing telemetry.

Pros: Synchronous modification. 100% vendor-agnostic (wraps around the UPF without touching its internals).

Cons: High implementation complexity. Crossing the virtual interface (veth) boundary introduces a slight CPU context-switch penalty compared to pure kernel processing.

Approach 3: User-Space UPF Integration (Direct IPC)
Mechanism: Similar to Approach 2, eBPF pushes exception traffic via AF_XDP to user space. However, instead of using a veth pair, the Rust Daemon passes the memory pointer directly to a cloud-native UPF process (e.g., Open5GS) via Inter-Process Communication (IPC) or a shared memory queue.

Primary Use Case: Extreme high-throughput, low-latency enterprise environments.

Pros: The absolute fastest method for inline processing, completely avoiding virtual network stack overhead.

Cons: Severe vendor lock-in. It requires tight, custom coupling with a specific vendor's UPF internals, destroying the universal "plugin" concept.

4. Hitless Lifecycle Management (The Upgrade Flow)
The core novelty of the architecture is the ability to upgrade a Wasm plugin from V1 to V2 without dropping a single packet.

Warm-Up: The Sidecar downloads v2.wasm. The Rust Daemon initializes it alongside v1.wasm and assigns it a new memory boundary.

Atomic Swap: The Rust Daemon executes a single bpf_map_update_elem() system call. Protected by kernel RCU locks, the pointer for the enterprise TEID is flipped from V1 to V2 in a single CPU cycle.

The Cutover: The very next packet arriving at the NIC is routed to V2.

Drain Cycle: V1 remains alive for 100ms to process any packets already in its buffer. Once empty, V1 is gracefully terminated.

5. Fault Tolerance & Graceful Degradation
Telecom carrier-grade requirements dictate that the system must degrade gracefully without interrupting active user sessions.

Wasm Sandbox Crash (Data Plane Failure): If a user's Wasm module panics or hangs, the eBPF buffer will rapidly fill. The eBPF fast-path is hardcoded with a capacity check. If the buffer hits >95% capacity, eBPF defaults to XDP_PASS (Fail Open), bypassing Wasm and routing the traffic normally. Telemetry is lost, but the 5G connection survives.

Operator Crash (Control Plane Failure): Because eBPF maps are pinned to the Linux Virtual File System (bpf fs), they survive user-space pod crashes. If the K8s Operator dies mid-flight, the active datapath continues routing traffic perfectly based on the last known state until K8s schedules a replacement Operator.

6. Prototype Scope (Phase 1 Target)
To prove baseline viability for the SBPI paper in a vendor-agnostic environment, the initial prototype will implement Approach 1 (Out-of-Band Analysis). This isolates the research to validating the K8s orchestration mechanics, the atomic map swaps, and the baseline latency of the eBPF-to-Wasm metadata handoff, serving as the foundation for future inline modifications.