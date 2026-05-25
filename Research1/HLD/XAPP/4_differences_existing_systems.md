# 4. Differences from Existing Systems

Our proposed architecture fundamentally alters how xApps are executed and managed compared to the reference implementations provided by the O-RAN Software Community (OSC RIC) and major vendors.

## 4.1. The E2 Subscription Pipeline (Docker vs. Wasm/DLB)

| Feature | OSC RIC (Standard K8s/Docker) | Our Architecture (Wasm + GNR-D) |
| :--- | :--- | :--- |
| **Execution Environment** | Docker Container (K8s Pods) | Wasm AOT Linear Memory Sandbox |
| **Startup Time** | 2 - 30 seconds | ~50 microseconds |
| **High-Speed Routing** | Software-based RMR (Routing Manager) | Hardware-accelerated Intel DLB |
| **Memory Isolation** | Shared Kernel, cgroups (Weak) | Mathematical Sandbox (Strong) |
| **E2 Sub Upgrade** | Drop subscription, reconnect (Disruptive) | DLB pointer swap (Hitless/Lossless) |
| **ML Inference** | Standard CPU execution | Hardware offload via Intel AMX |
| **Conflict Enforcement** | Intent-based (Relying on xApp compliance) | Hardened runtime capability boundary |

## 4.2. Clarifying the Scope: Near-RT RIC vs. dApps

> [!WARNING]
> **Avoiding the dApp Collision:** The O-RAN Next Generation Research Group (nGRG) is actively standardizing the use of WebAssembly for **dApps (Distributed Applications)**. It is critical to differentiate our work from this emerging concept.

*   **dApps (The nGRG Focus):** dApps operate over the upcoming **E3 interface**, co-located directly at the **O-DU (Distributed Unit)**. They operate at the MAC/PHY layer on sub-1ms timescales, manipulating raw I/Q samples.
*   **xApps (Our Focus):** Our architecture remains strictly anchored to the **Near-RT RIC and the E2 interface** (10ms to 1s timescale). We are solving the container orchestration, state migration, and security issues of standard xApps interacting with the O-CU/O-DU via E2AP/SCTP.

While both utilize Wasm, our contribution is the **systems architecture of the Near-RT RIC platform itself** (utilizing DLB and DSA for hitless state migration and message routing), rather than the sub-millisecond edge logic of O-DU dApps.
