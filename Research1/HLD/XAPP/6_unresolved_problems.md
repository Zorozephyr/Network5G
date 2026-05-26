# 6. Unresolved Problems and Limitations

While the Wasm/GNR-D architecture addresses container overhead, hitless migration, security isolation, and conflict enforcement, a critical self-assessment reveals five ongoing challenges that our design does not fully resolve. Academic integrity requires these be documented and discussed transparently.

## 6.1. E2AP and ASN.1 Decoding Overhead

The E2 Application Protocol (E2AP) and E2 Service Models (E2SM) utilize ASN.1 (Abstract Syntax Notation One) for serialization. Decoding ASN.1 payloads is computationally intensive. In our architecture, the E2 Terminator strips the SCTP headers, but the raw ASN.1 payload is pushed into the Wasm sandbox via DSA. 

*   **The Problem:** Running ASN.1 decoding libraries inside Wasm introduces execution overhead compared to native C++ decoding. While Wasm AOT compilation mitigates this, the serialization overhead remains a significant bottleneck for achieving sub-millisecond control loops.
*   **Context:** BubbleRAN/FlexRIC solves this problem by abandoning standard ASN.1 encoding entirely, using a custom efficient binary format. This achieves 650µs control loops but sacrifices E2 standards compliance. WA-RAN identifies ASN.1 overhead as an open problem but does not propose a solution.
*   **Our Trade-off:** We retain standard E2AP/ASN.1 encoding for compliance but accept the decoding overhead. Potential mitigations include: (a) pre-decoding ASN.1 in the E2T (native C++) before pushing structured data into Wasm, trading compliance purity for performance; (b) developing a Wasm-optimized ASN.1 decoder with SIMD extensions; (c) using Intel DSA for parallel decode of multiple E2 Indications.
*   **Impact Assessment:** This overhead primarily affects the sub-1ms aspiration. For the standard 10ms–1s Near-RT RIC budget, ASN.1 decoding (typically 100–500µs native, ~1–3ms in Wasm) remains within budget. It becomes critical only if targeting BubbleRAN-class sub-ms performance.

## 6.2. Global State Sharing Across Isolated Workers

Our architecture relies on the DLB to route messages to specific Wasm instances, ensuring atomic, per-UE ordering. Wasm provides strict linear memory isolation, meaning Wasm Worker 1 cannot see the memory of Wasm Worker 2.

*   **The Problem:** Some xApp algorithms require an aggregated, global view of the cell (e.g., total PRB utilization across all UEs) to make decisions. Because the Wasm instances are fully isolated, they cannot easily share a global state variable. 
*   **Current Mitigation:** We rely on the Wasm host environment to maintain a fast, localized Key-Value store that xApps can query via host functions (`read_cell_state()`, `write_vendor_state()`), which introduces a slight memory copy overhead that the DLB/DSA fast-path was specifically designed to avoid.
*   **The Security Risk (Data Poisoning):** As detailed in Section 8.8, introducing shared state creates a new attack vector. A rogue xApp could write falsified global metrics (e.g., `total_prb_utilization = 100%`) to trick other xApps into suboptimal decisions. The mitigation (RBAC on the KV API with vendor-namespaced writes) reduces but does not eliminate this risk — read access to global aggregates still allows information leakage.
*   **Comparison with SDL:** The O-RAN standard's Shared Data Layer (SDL) addresses a similar problem but operates at much higher latency (network round-trip to Redis). Our in-process host KV store trades distributed availability for microsecond access — appropriate for the Near-RT control loop but unsuitable for cross-node state sharing.

## 6.3. Hardware Dependency and Lock-In

The most significant limitation of our proposed architecture is its rigid dependency on Intel Granite Rapids-D (GNR-D) specific hardware blocks.

*   **The Problem:** The hitless E2 subscription transfer relies entirely on the Intel Dynamic Load Balancer (DLB), the memory isolation relies on Intel DSA for zero-copy, and the latency budget relies on Intel AMX.
*   **The Impact:** This design cannot be easily ported to standard ARM-based servers, NVIDIA BlueField DPUs, or cloud-hosted COTS hardware without falling back to software-based dispatching, which re-introduces the latency bottlenecks we sought to eliminate. This contradicts the O-RAN ethos of hardware-agnostic, white-box deployments.
*   **Comparison with BubbleRAN:** BubbleRAN achieves sub-ms loops on COTS x86 hardware with no special dependencies — demonstrating that aggressive software optimization alone can meet latency targets. Our architecture offers stronger security isolation and hitless upgrades, but at the cost of a hardware constraint that BubbleRAN avoids entirely.
*   **Mitigation Path:** The architecture should be designed with a Hardware Abstraction Layer (HAL) that allows software fallback for each accelerator:
    * **DLB → Software:** Fall back to DPDK `rte_distributor` or user-space consistent hashing. Functional but with CPU overhead.
    * **DSA → Software:** Fall back to standard `memcpy()`. Functional but consumes CPU cycles.
    * **AMX → Software:** Fall back to standard Wasm SIMD or CPU-based inference. Functional but 10–50ms per inference instead of ~5µs.
    * With software fallback, the architecture degrades gracefully to WA-RAN-class performance (functional, with higher latency) rather than failing entirely.

## 6.4. Wasm Ecosystem Maturity for RAN Workloads

*   **The Problem:** The Wasm ecosystem for systems-level, latency-critical workloads is immature compared to native C/C++ toolchains. Existing xApp SDKs (OSC, BubbleRAN) are designed for native runtimes with Python/C++ bindings. There is no established Wasm xApp SDK, no Wasm-compatible RMR library, and no Wasm-native E2SM codec.
*   **The Impact:** Any xApp must be recompiled to Wasm, which may require significant porting effort — especially for xApps with complex native dependencies (e.g., TensorFlow, PyTorch, ONNX runtimes for ML inference). WA-RAN (HotNets '24) identifies "developing RAN-specific Wasm toolchains with appropriate compilers and sanitizers" as an open research area.
*   **Adoption Barrier:** eZTrust/OZTrust's eBPF approach is transparently injectable into existing containers without modifying xApp source code. Our approach requires fundamental recompilation — a much higher adoption barrier for operators with existing xApp investments.

## 6.5. Conflict Detection is Out of Scope

*   **The Problem:** Our architecture provides tamper-proof **enforcement** of conflict management decisions (Section 2.4). However, we do not contribute any conflict **detection** or **resolution** algorithm. The Host-Enforced Conflict Manager must be populated with actual arbitration logic — PACIFISTA-style statistical profiling, COMIX-style NDT simulation, Adamczyk CMF priority rules, or GraphSAGE-based dynamic detection.
*   **The Impact:** Our enforcement layer is an empty shell without an integrated detection/resolution system. A production deployment requires combining our platform with one of the existing conflict management frameworks.
*   **The Opportunity:** This is intentionally complementary. Our enforcement mechanism is **algorithm-agnostic** — any conflict detection/resolution system can be embedded in the `send_e2_control()` host function without modification to the xApp or the enforcement architecture. This makes our platform a universal enforcement substrate for the entire conflict management research community.
