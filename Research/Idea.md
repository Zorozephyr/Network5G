1. How It Works Currently (The Industry Baseline)
Right now, in the broader cloud-native ecosystem (outside of telecom), WebAssembly and eBPF are like two separate departments in a company that occasionally talk to each other.

Current eBPF:

Where it lives: Deep inside the Linux Kernel (e.g., attached to the XDP hook on the physical or virtual NIC).

What it does: It intercepts raw packets before the OS even knows they exist. It is written in restricted C, compiled to bytecode, and does simple, lightning-fast operations: "Read IP header, drop packet" or "Read port, forward packet."

The Limitation: The kernel verifier prevents eBPF from running complex loops, making external database calls, or doing heavy payload inspection. It has to be simple to be safe.

Current WebAssembly (Wasm):

Where it lives: In user-space (like a standard container, but much lighter).

What it does: It runs custom, complex logic (written in Go, Rust, C++) inside a highly secure sandbox. Service meshes like Envoy use Wasm heavily.

The Limitation: Because it lives in user-space, a network packet has to travel all the way up the heavy Linux network stack to reach it, which ruins ultra-low latency.

How they currently interact (The "Control Plane" Model):
Currently, tools use Wasm to manage eBPF. A lightweight Wasm container spins up in user-space, reads a configuration, and pushes the eBPF bytecode down into the kernel. The Wasm module acts as the manager, and eBPF is the dumb, fast worker.

2. What We Are Proposing (The Telecom Data Plane Handoff)
We are proposing to flip that relationship upside down to solve a massive bottleneck in the 5G UPF.

Instead of Wasm managing eBPF from above, we are using eBPF to act as an ultra-fast traffic cop that feeds specific packets into a Wasm engine for custom processing.

Here is the exact step-by-step datapath architecture for your paper:

The Fast Path (eBPF): A massive stream of 5G GTP-U packets hits the worker node. The eBPF XDP program intercepts them. For 95% of standard internet traffic, eBPF instantly strips the GTP-U tunnel header, looks up the TEID in a kernel map, and forwards the packet out. Blistering fast, millions of packets per second.

The Exception Handoff: Suddenly, a packet arrives from a specific enterprise network slice that requires proprietary Deep Packet Inspection (DPI). eBPF is too restricted to do complex DPI. So, the eBPF program writes that specific packet payload into a highly optimized eBPF Ring Buffer (a shared memory space between the kernel and user-space).

The Custom Logic (Wasm): A Wasm runtime (embedded within your UPF pod) is listening to that ring buffer. It instantly pulls the packet, runs the complex, proprietary DPI logic (which was compiled into Wasm from Go or Rust), and determines the routing action.

3. The Exact Use Case (Why Telcos Will Care)
If you submit this paper, the "Introduction" and "Motivation" sections need to hit a nerve. Here is the exact enterprise use case you are solving: Vendor Independence and Live Upgrades.

Imagine a telecom operator is running a live 5G Core using a UPF provided by a major vendor.

The Problem: A new security vulnerability drops, or an enterprise client requests a custom billing metric based on a new IoT protocol. To add this logic to the UPF today, the telco has to wait months for the vendor to release a new C-based binary. When they finally deploy it, they have to restart the UPF pods, dropping thousands of active phone calls.

Your Solution: With your architecture, the telco's network engineers can write the custom DPI or billing logic in Go, compile it into a tiny .wasm file, and declare an intent via Kubernetes.

The Go Operator's Role: Your custom K8s Operator sees this intent. It reaches out, grabs the .wasm file, and injects it into the Wasm runtime of the already running UPF pod. It then updates the eBPF map to say, "Hey, start routing packets from slice X to the Wasm buffer."

The Result: You have just added custom, proprietary, heavy-compute logic to a live 5G UPF data plane in milliseconds, without rebooting the pod, without dropping a single active packet, and without asking the UPF vendor for permission.

That is the "hitless module injection" magic.


Things To Worry About:
1.Isolation and security concerns
2.Polling(More CPU cycles)/Event Driven -> Interupt Waking up program
3.Limitations of eBPF
4. malicious inputs are out of scope for this prototype...
