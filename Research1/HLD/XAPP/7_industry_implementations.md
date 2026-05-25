# 7. Current Industry Implementations (2025–2026 Context)

To position our architecture effectively, it is vital to understand how major telecommunications vendors are currently implementing the Near-RT RIC and dealing with the constraints of the O-RAN architecture.

## 7.1. The Shift to Non-RT RIC and rApps
The most notable trend in 2025/2026 is a commercial pivot away from the Near-RT RIC (xApps) in favor of the Non-RT RIC (rApps). 
*   Because the 10ms–1s E2 control loop is so technically demanding and prone to the container/latency issues outlined in Document 1, operators have struggled to deploy complex Near-RT xApps reliably.
*   Instead, vendors are pushing automation into **rApps**, which operate on >1-second loops over the A1 interface, managing slower tasks like long-term energy saving and policy updates.
*   **Our Position:** Our architecture directly addresses the technical debt that caused this pivot, providing a viable path to resurrecting high-speed, sub-second xApp commercial viability.

## 7.2. Vendor Landscape

### Ericsson (EIAP)
*   **Architecture:** Ericsson's Intelligent Automation Platform (EIAP) focuses heavily on the Service Management and Orchestration (SMO) layer and the Non-RT RIC. 
*   **Implementation:** Ericsson approaches the Near-RT RIC with caution, preferring to handle critical, low-latency radio resource management inside their proprietary O-DU/O-CU software rather than exposing it to third-party xApps, due to security and performance concerns.

### Nokia (MantaRay & Juniper Acquisition)
*   **Architecture:** In late 2025, Nokia absorbed Juniper Networks’ highly regarded Near-RT RIC assets to bolster its MantaRay SMO platform.
*   **Implementation:** The Juniper RIC was built on a cloud-native Kubernetes microservices architecture. It heavily utilizes standard containerization and relies on robust conflict management modules to handle multi-vendor xApps. However, it still suffers from the fundamental container cold-start and state-transfer limitations inherent to K8s.

### VMware (Broadcom)
*   **Status:** VMware’s Distributed RIC was once a leading vendor-neutral implementation. Following corporate restructuring, their presence in the standalone RIC market has diminished.
*   **Legacy:** Their architecture relied heavily on a distributed, shared Redis database to manage E2 state and K8s for xApp orchestration.

## 7.3. Summary
The major vendors are actively grappling with the limitations of K8s-based xApps. By providing a hardware-accelerated Wasm platform, our architecture offers a solution to the exact latency and security vulnerabilities that currently restrict widespread Near-RT RIC adoption in tier-1 carrier networks.
