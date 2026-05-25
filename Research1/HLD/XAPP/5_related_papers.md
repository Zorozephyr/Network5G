# 5. Related Literature and Differentiation

Our proposed Wasm/GNR-D architecture intersects with several recent advancements in O-RAN research. It is critical to delineate how our systems-level approach differs from existing solutions.

## 5.1. Wasm in O-RAN: WA-RAN (HotNets '24)
The paper *"Towards Seamless 5G Open-RAN Integration with WebAssembly"* (HotNets 2024) introduced the foundational concept of using Wasm plugins to run 5G RAN stack components. 
*   **How we are different:** WA-RAN proposed the general concept of Wasm for platform agnosticism and basic sandboxing. However, it was a software-only proof of concept. It did not address the high-throughput challenges of the E2 interface (relying on generic software dispatch), lacked ML hardware acceleration, and did not design a hitless E2 subscription transfer mechanism. Our work elevates the WA-RAN concept into a complete, line-rate system architecture leveraging Intel DLB/AMX/DSA.

## 5.2. Conflict Resolution: ORCA and COMIX (2024–2026)
Recent literature focuses heavily on solving xApp conflicts. Frameworks like **ORCA** use Large Language Models (LLMs) to reason about conflicts, while **COMIX** uses Network Digital Twins to simulate parameter flipping before execution.
*   **How we are different (and complementary):** These papers focus entirely on the *intent* layer—the algorithms used to decide *if* a conflict exists. They assume a perfectly compliant xApp that sends messages via RMR. Our work provides the **enforcement mechanism**. By embedding the Conflict Manager at the Wasm host-function boundary, we provide the hardened runtime isolation that guarantees xApps cannot exploit Linux networking to bypass intelligent frameworks like ORCA.

## 5.3. xApp Migration and State Transfer: MANATEE and CORMO-RAN
Several works address the need to migrate xApps across the Near-RT RIC cluster.
*   **MANATEE:** Uses Service Mesh technology for DevOps orchestration, canary releases, and A/B testing of xApps.
*   **CORMO-RAN:** Proposes a framework for stateful migration of containers to consolidate workloads for energy savings.
*   **How we are different:** Both MANATEE and CORMO-RAN operate at the Kubernetes container level. Their migration processes take seconds to minutes and require the xApp to push its state to a Shared Data Layer (SDL), tear down the pod, boot a new pod, pull the state from the SDL, and request a new E2 subscription.
*   **Our Advantage:** Our architecture achieves **microsecond-level hitless migration** for xApp updates. Because the E2 subscription state is anchored at the DLB/E2T layer, we do not require the xApp to pause E2 reporting or sync through an external SDL. We simply load the new Wasm binary and swap the hardware queue pointer, resulting in zero E2 subscription gaps.
