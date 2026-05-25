# 6. Unresolved Problems and Limitations

While the Wasm/GNR-D architecture elegantly solves container overhead, hitless migration, and security isolation, a critical assessment reveals three ongoing challenges that our design does not fully resolve.

## 6.1. E2AP and ASN.1 Decoding Overhead
The E2 Application Protocol (E2AP) and E2 Service Models (E2SM) utilize ASN.1 (Abstract Syntax Notation One) for serialization. Decoding ASN.1 payloads is computationally intensive. In our architecture, the E2 Terminator strips the SCTP headers, but the raw ASN.1 payload is pushed into the Wasm sandbox via DSA. 
*   **The Problem:** Running ASN.1 decoding libraries inside Wasm introduces execution overhead compared to native C++ decoding. While Wasm AOT compilation mitigates this, the serialization overhead remains a significant bottleneck for achieving sub-millisecond control loops.

## 6.2. Global State Sharing Across Isolated Workers
Our architecture relies on the DLB to route messages to specific Wasm instances, ensuring atomic, per-UE ordering. Wasm provides strict linear memory isolation, meaning Wasm Worker 1 cannot see the memory of Wasm Worker 2.
*   **The Problem:** Some xApp algorithms require an aggregated, global view of the cell (e.g., total PRB utilization across all UEs) to make decisions. Because the Wasm instances are fully isolated, they cannot easily share a global state variable. 
*   **Current Mitigation:** We must rely on the Wasm host environment to maintain a fast, localized Key-Value store that xApps can query via host functions, which introduces a slight memory copy overhead that the DLB/DSA fast-path was specifically designed to avoid.

## 6.3. Hardware Dependency and Lock-In
The most significant limitation of our proposed architecture is its rigid dependency on Intel Granite Rapids-D (GNR-D) specific hardware blocks.
*   **The Problem:** The hitless E2 subscription transfer relies entirely on the Intel Dynamic Load Balancer (DLB), the memory isolation relies on Intel DSA for zero-copy, and the latency budget relies on Intel AMX. 
*   **The Impact:** This design cannot be easily ported to standard ARM-based servers, NVIDIA BlueField DPUs, or cloud-hosted COTS hardware without falling back to software-based dispatching, which re-introduces the latency bottlenecks we sought to eliminate. This contradicts the O-RAN ethos of hardware-agnostic, white-box deployments.
