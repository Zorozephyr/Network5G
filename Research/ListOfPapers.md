1. The Ring-Buffer Masterclass (Start Here)
Before you can hand packets to WebAssembly, you have to understand how to get packets out of the eBPF kernel space quickly without burning CPU cycles.

Paper: SPRIGHT: High-performance eBPF-based event-driven, shared-memory processing for serverless computing (Qi et al., 2022/2024)

Link: IEEE | ACM

Why you are reading this first: This is the gold standard for eBPF shared memory. It explains how to creatively use eBPF socket messages to bypass heavy protocol processing.

What to extract: Pay close attention to their architecture diagrams showing how memory is shared between the eBPF datapath and user-space functions. This is the exact blueprint for your "exception handoff" ring buffer.

2. The Datapath Prototype (The "Wasm-in-Kernel" Mechanic)
Now that you know how shared memory works, you need to see how academics are currently triggering WebAssembly from inside the kernel.

Paper: Safe Kernel Extensibility and Instrumentation With Webassembly (Abdelmonem, CMU, 2025)

Link: CMU Reports Archive

Why you are reading this second: This paper separates the control path from the data path. It physically embeds a Wasm runtime into a kernel module and triggers it via kernel hooks.

What to extract: Read the "System Design" section heavily. You are looking for the limitations. How hard was it for them to pass data into Wasm? You will use their pain points to justify why your ring-buffer approach is better for 5G packets.

3. The Packaging Mechanism (Bringing eBPF and Wasm Together)
You know how to share memory, and you know how Wasm connects to the kernel. Now you need to learn how to package them together so Kubernetes can actually deploy them.

Paper: Wasm-bpf: Streamlining eBPF deployment in cloud environments with WebAssembly (Zheng et al., 2024)

Link: arXiv

Why you are reading this third: This paper solves the cloud-native deployment nightmare. They package the eBPF bytecode and the user-space control code into a single .wasm module.

What to extract: Understand their build and deployment pipeline. Your Go operator is going to need to fetch modules packaged exactly like this.

4. The Engineering Reality Check (Open Source Docs)
Take a break from academic formatting and look at how the open-source community actually codes this.

Resource: Eunomia-bpf Documentation

Link: Eunomia.dev

Why you are reading this fourth: Academic papers hide the ugly code. Eunomia is the leading open-source project bridging Wasm and eBPF right now.

What to extract: Go through their tutorials. Look at the actual C code for the eBPF programs and the Rust/Go code for the Wasm side. If you are going to build a "Tracer Bullet Prototype," you will likely fork code from here.

5. The Operator Playbook (Scaling to Kubernetes)
Now you transition from node-level kernel hacking to cluster-level orchestration.

Paper: NetEdit: An orchestration platform for eBPF network functions at scale (Benson et al., Meta, 2024)

Link: ACM

Why you are reading this fifth: Meta built an orchestrator to manage eBPF across a massive fleet of servers. This will teach you how big tech companies translate high-level network intents into low-level eBPF map updates.

What to extract: Look at how they design their "Intent-based Configuration." This will directly inspire the Custom Resource Definitions (CRDs) you write for your K8s Operator.

6. The Hitless Holy Grail (State Preservation)
Finally, read the closest thing to your core novelty to ensure you understand the state preservation problem.

Paper: Orchestrating Network Security Borders in the Computing Continuum with Liqo (Tornesello, Politecnico di Torino, 2026)

Link: Politecnico di Torino Thesis (Note: If the PDF is gated, search for the title on Google Scholar for the open-access preprint).

Why you are reading this last: This paper implements a dynamic Kubernetes controller that reconfigures the eBPF data plane.

What to extract: Skip straight to the section on "Data Plane State Preservation." Read how they ensure the system never enters an inconsistent state during updates. This is the exact problem you are solving with your "hitless module injection."

7.WebAssembly Orchestration in the Context of Serverless Computing (Springer / Kjorveziroski & Filiposka, 2023)