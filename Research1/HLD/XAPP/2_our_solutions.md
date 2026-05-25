# 2. Our Solutions: A Hardware-Accelerated Wasm Architecture

To address the fundamental limitations of container-based xApps, we propose transitioning the Near-RT RIC execution environment from Kubernetes/Docker to a **WebAssembly (Wasm) architecture accelerated by the Intel GNR-D SoC**. 

Our design systematically resolves the four critical issues identified in Document 1.

## 2.1. Wasm AOT for Microsecond Cold Starts
* **The Solution:** Instead of spinning up 500MB Docker containers, xApps are compiled to Wasm bytecode and further optimized via Ahead-of-Time (AOT) compilation into native machine code.
* **The Result:** Wasm instances instantiate in **~50 microseconds**. When the RIC scales horizontally, new xApp workers are available to process E2 indications almost instantly. This entirely eliminates the multi-second "blind spot" during scaling events, ensuring strict adherence to the 10ms control loop budget.

## 2.2. Hardware-Backed Hitless Subscription Transfer
* **The Solution:** We decouple the E2 subscription state from the application lifecycle. Instead of using K8s network namespaces and RMR for routing, incoming E2 Indication messages (post-SCTP termination) are mapped to **Intel Dynamic Load Balancer (DLB)** hardware queues. 
* **The Result:** When upgrading an xApp (e.g., from v1 to v2), the E2 subscription is **never deleted**. The gNB continues sending reports uninterrupted. The RIC platform simply executes a hitless pointer swap: it redirects the DLB queue from the memory address of the v1 Wasm instance to the v2 Wasm instance. This achieves true lossless migration with zero gap in RAN control.

## 2.3. Hardened Sandboxing & Fuel Metering (Immunity to E2T Crashes)
* **The Solution:** Wasm provides strict linear memory sandboxing. An xApp physically cannot access the memory of the host system, the E2T, or other xApps. Furthermore, we implement **Fuel Metering**—a mechanism that halts execution if an xApp consumes too many CPU cycles (e.g., an infinite loop).
* **The Result:** This neutralizes the vectors for **CVE-2023-41628**. A malicious xApp cannot flood the network or crash the E2T, because it lacks the OS-level network sockets to bypass its designated output channel. If an xApp crashes, it only kills its isolated Wasm instance, not the RIC platform.

## 2.4. Enforced Execution Boundaries for Conflict Resolution
* **The Solution:** In a containerized environment, Conflict Managers operate on the honor system (assuming xApps send intents via RMR). In our architecture, the Wasm sandbox enforces a hard capability-based security model. xApps cannot emit E2 Control messages directly; they must call a specifically provisioned host function (e.g., `send_e2_control()`).
* **The Result:** We physically embed the Conflict Manager logic into the host function boundary. It is structurally impossible for an xApp to bypass conflict resolution. This provides the "hardened runtime isolation" that researchers have identified as necessary to permanently solve multi-vendor parameter flipping.
