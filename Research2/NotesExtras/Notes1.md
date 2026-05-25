# 5G & Networking Quick Notes

## 1. Core Networking Concepts
- **DSCP (Differentiated Services Code Point)**: A 6-bit field in IPv4/IPv6 used for QoS (Quality of Service) to decide packet priority. e.g., High DSCP for voice, low for file downloads. SmartNICs can rewrite this in the pipeline.
- **NAT (Network Address Translation)**: Translates Private to Public IPs and vice versa.
- **MPLS (Multiprotocol Label Switching)**:
  - **LER (Label Edge Router)**: Enters the MPLS network, looks at IP, assigns the label.
  - **LSR (Label Switch Router)**: Swaps the incoming label for a new one to the next hop.
  - **LSP (Label Switched Path)**: The predefined tunnel/path the packet takes.

## 2. 5G Session & Data Plane
- **Session Setup**: Phone talks to SMF -> TEID assigned -> SMF uses PFCP protocol to notify UPF (running on CPU).
- **Hardware Offload**: UPF software uses P4Runtime API to program match-action rules directly into the SmartNIC.
- **Descriptor Ring**: A shared data buffer in RAM allowing CPU and NIC to communicate without constant interrupts. Contains Address, Status (Empty = NIC ready, Full = CPU ready). In fast-path hardware offload, CPU is bypassed; DMA and rings are used only if falling back to the slow-path CPU.

## 3. Hardware Accelerators & NPUs
- **Hardware Accelerator**: Specialized silicon designed to perform specific functions (crypto, AI, compression) faster than a general-purpose CPU.
- **NPU (Network Processing Unit)**: A highly specialized ASIC with massively parallel architecture and specific instruction sets optimized for line-rate packet processing (parsing headers, routing lookups, ACLs).

## 4. NVIDIA BlueField & DOCA
- **BlueField DPU**: Physical silicon on a SmartNIC with 8-16 ARM cores running an independent Linux OS. Contains hardware accelerators for crypto, regex, decompression, and a wire-speed network engine. Fundamentally different from eBPF/XDP (which runs on the host CPU).
- **DOCA (Data Center Infrastructure-on-a-Chip Architecture)**: The software framework (like CUDA for GPUs) for DPUs.
  - **DOCA Flow**: Programs the eSwitch pipeline (match/action rules).
  - **DOCA Crypto**: Offloads IPsec and TLS.
  - **DOCA DMA**: Bypasses kernel, allowing DPU to read/write host RAM.
  - **DOCA GPUNetIO**: DPU memory to GPU memory (AI inference).
  - **DOCA Regex, Compress, Telemetry**: Dedicated offload libraries.

## 5. Hardware Comparisons

### Software vs. SmartNIC vs. DPU vs. SuperNIC
| Feature | DPDK | SmartNIC | DPU | SuperNIC |
| :--- | :--- | :--- | :--- | :--- |
| **Type** | Software | Hardware | Hardware | Hardware |
| **Job** | Kernel Bypass | Task Offload | Infrastructure Isolation | AI GPU-to-GPU Fabric |
| **Control**| Host OS | Host OS | Local DPU OS | Host/Fabric Controller |
| **Use Case**| NFV, Fast packet | vSwitch, telemetry | Multi-tenancy, Zero-trust | Gen AI training |

### Intel GNR-D vs. DPU
*Note: GNR-D is used at the Far Edge, while DPUs are used in Core Data Centers.*

| Feature | Intel GNR-D (Xeon 6 SoC) | DPU (NVIDIA BlueField-3) |
| :--- | :--- | :--- |
| **Role** | The main brain / host CPU | A helper PCIe card |
| **Cores** | High-performance x86 P-Cores | Low-power ARM Cores |
| **OS** | Runs the main Host OS | Runs its own secure OS (DOCA) |
| **Accel.** | Inline hardware blocks (QAT, DLB, DSA) | Dedicated NPU / Switch ASIC |
| **Network** | Integrated Ethernet ports (up to 100G) | Acts as the NIC to the 400G+ network |
| **Security**| Shares Host CPU trust domain | "Zero Trust" isolated from Host CPU |


Why Not UPF: Not able to find a usecase which needs the architecture i propose