Current Design -> eBPF/XDP -> AF_XDP -> WASM on host CPU
A DOCA-based design would move the GTP-U fast-path into DPU silicon via DOCA Flow (hardware TEID matching at line rate, zero CPU), run the Wasm exception-path engine on the DPU's Arm cores (isolated from host workloads), and use DOCA DMA to inject modified packets back into the host's UPF without kernel stack traversal  — solving your veth reinjection overhead entirely.
However, this creates hardware vendor lock-in (NVIDIA BlueField only), which is the exact opposite of your paper's "vendor independence" thesis.

In an actual UPF, superNICS are not being used...Even DPU are very less...Its mostly still DPDK
Need to think of DPDK based solutions

           ┌─────────────────────┐
           │  Central Core UPF   │  ← Aggregates entire city/region
           │  100+ Gbps needed   │  ← THIS is where DPUs/DPDK matter
           │  Dedicated hardware  │
           └──────────┬──────────┘
                      │
        ┌─────────────┴─────────────┐
        │                           │
  ┌─────┴─────┐              ┌─────┴─────┐
  │ Regional   │              │ Regional   │
  │ Edge UPF   │              │ Edge UPF   │  ← 10-40 Gbps sufficient
  │ (MEC site) │              │ (MEC site) │  ← Software on COTS works
  └─────┬─────┘              └───────────┘
        │
  ┌─────┴──────────────┐
  │  Far-Edge UPF      │  ← Factory, hospital, campus
  │  1-5 Gbps enough   │  ← Tiny software UPF on a mini-server
  │  Private 5G / IoT  │  ← Your architecture's sweet spot
  └────────────────────┘

The killer use cases for software UPFs (where your research lives):

Private 5G / Enterprise campus — A factory with 10,000 IoT sensors doesn't need 100 Gbps. It needs 1–5 Gbps, data sovereignty (traffic never leaves premises), and the ability to add custom DPI rules without calling Ericsson. This is literally your paper's motivation.

Edge / MEC sites — Telecom operators deploying UPFs at the edge of the network (closer to base stations) for low-latency applications. These handle a fraction of the core's traffic — 10–20 Gbps is plenty.

MVNOs and smaller operators — They can't afford $2,000 DPU cards in every rack. A software UPF on a $3,000 Dell server is their entire business model.

Development/testing/staging — Every 5G vendor needs software UPFs for their CI/CD pipelines.


DPU = NVIDIA vendor lock-in (the exact problem your paper solves)
DPU = $2,000+ per card vs. $0 for eBPF/XDP
DPU = proprietary DOCA SDK vs. standard Linux kernel APIs
DPU = cannot run arbitrary tenant Wasm plugins on fixed-function silicon
DPU = cannot do hitless plugin upgrades (the logic is in hardware)

Can the ARM Nodes in DPU be controlled by Kubernetes