# Summary: WebAssembly Orchestration in the Context of Serverless Computing (Kjorveziroski & Filiposka, 2023)

## 1. What is the paper about?
The paper examines how to orchestrate WebAssembly (Wasm) modules as first-class serverless workloads within Kubernetes. By benchmarking against OpenFaaS containers, it demonstrates that Wasm offers up to 2x faster instantiation times, an order of magnitude smaller artifact sizes, and per-invocation isolation, while maintaining competitive execution performance for non-processor-intensive tasks.

## 2. What is the new thing that it is proposing (in detail)?
The paper details an architecture to extend Kubernetes so it can natively schedule and run WebAssembly modules without requiring full container images. 
- **Containerd Shim:** They utilize an extended WebAssembly software shim (based on Spin/Wasmtime) that interfaces directly with `containerd`.
- **Kubernetes Operator & CRD:** They introduce a custom Kubernetes Operator with a `WasmApp` Custom Resource Definition (CRD) to abstract away boilerplate.
- **Per-Invocation Isolation:** They configure the shim so that each incoming request instantiates a new Wasm environment linearly mapped to a lightweight OCI image, spinning down immediately afterward.

## 3. Is it something we are implementing in our paper?
**Yes, but with fundamental architectural differences.** 
You are also implementing a Custom Go Kubernetes Operator with a CRD (`WasmPlugin`) that pulls Wasm payloads from an OCI registry. 
However, their design schedules **standalone Pods** managed by a `containerd` shim for stateless HTTP functions. In contrast, your 5G HLD dictates using a **Sidecar container** with a dedicated Rust Daemon/WasmEdge runtime embedded *inside* an existing UPF pod. 

## 4. What ideas are better in their paper that maybe we can use and what is not better etc?
**Better/Applicable:**
- Storing Wasm modules in OCI "scratch" images is the definitive way to distribute plugins to our Sidecar.
- The conceptualization of a Kubernetes Operator watching CRDs to declaratively deploy Wasm is exactly aligned with your Control Plane design.
**Not Better/Not Applicable:**
- Their execution model focuses on complete "per-invocation isolation" (creating a new Wasm environment for every request). That cold-start latency (even if fast for serverless) is incompatible with line-rate 5G networking. Your design requires a persistently running ("warm") Wasm sandbox reading continuously from an eBPF zero-copy Ring Buffer or AF_XDP socket.

## 5. Additional Literature Analysis Questions
- **How would the `containerd` shim architecture handle direct shared-memory integration with the kernel's eBPF map subsystem, given its reliance on standard HTTP/CGI proxies for I/O?**
- **While their model destroys state after every execution, how can we leverage their OCI artifact system to implement our "warm" hitless V1-to-V2 atomic swap safely?**
