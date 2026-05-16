# Project Context: Intent-Driven Orchestration of Hybrid eBPF/WebAssembly Data Planes for 5G UPFs

## 1. The Core Architecture
*   **The Problem**: Standard 5G UPFs (User Plane Functions) have rigid data paths and suffer from vendor lock-in. Adding custom Deep Packet Inspection (DPI) requires vendor updates and pod reboots, causing dropped user sessions.
*   **The Datapath Solution**: A hybrid data plane.
    *   **eBPF (XDP)** acts as an ultra-fast in-kernel traffic cop, handling the standard 95% of GTP-U packets.
    *   Packets requiring deep inspection are dropped into an **eBPF Ring Buffer**.
    *   An in-pod **WebAssembly (Wasm)** runtime pulls the packet, executes custom DPI, and determines routing actions.
*   **The Orchestration Solution**: A custom, Go-based **Kubernetes Operator**.
    *   Listens for high-level network intents via Custom Resource Definitions (CRDs).
    *   Dynamically fetches `.wasm` plugins, injects them into the UPF's Wasm runtime, and updates eBPF kernel maps to route specific traffic to the new plugin seamlessly.

## 2. Research Novelty (The "White Space")
This research targets the intersection of Kubernetes, eBPF, and WebAssembly, bypassing saturated topics like O-RAN. The primary publishable novelties are:
*   **Hitless Module Injection**: Dynamically swapping or upgrading Wasm modules inside the UPF *without* pod restarts or dropping active 5G sessions.
*   **State Preservation**: Orchestrating the migration or preservation of eBPF map state (TEIDs/routing rules) when Kubernetes reschedules the UPF pod to a new node.
*   **Orchestration Latency Benchmarking**: Measuring the precise time from a K8s intent declaration (`kubectl apply`) to actual kernel-level packet interception.

## 3. Tier 1 Reading List
Focus on these texts to master the datapath mechanics:
*   *Wasm-bpf: Streamlining eBPF deployment...* (Zheng et al., 2024)
*   *SPRIGHT: High-performance eBPF-based event-driven, shared-memory processing* (Qi et al., 2022/2024)
*   *Safe Kernel Extensibility and Instrumentation With Webassembly* (Abdelmonem, 2025)
*   *Eunomia-bpf Open Source Project Documentation*

## 4. Critical Feedback & Risk Mitigation
*   **Avoid the "Paper Architect" Trap**: Theoretical architectures must be grounded in actionable code. **Action**: Build a "Tracer Bullet Prototype" (a minimal Go script attaching a dummy eBPF program to a WasmEdge runtime) on a local VM to validate the raw datapath before writing the Operator.
*   **Prevent Scope Creep**: Trying to build the eBPF datapath, Wasm runtime, and K8s operator simultaneously will lead to burnout. **Action**: Artificially constrain the scope by using an existing open-source datapath (like Eunomia-bpf) and focus novel engineering efforts strictly on the Go Operator orchestrator and hitless injection mechanism.
*   **Own the Synthesis**: AI can find and summarize papers, but human reading of Tier 1 sources is essential to build the technical vocabulary and mathematical understanding necessary to defend the research during peer review.