# Summary: Safe Kernel Extensibility and Instrumentation with WebAssembly (CMU-CS-25-123)

## 1. What is the paper about?
This paper explores using WebAssembly (Wasm) as a secure middle ground for kernel extensibility. It aims to bridge the gap between Loadable Kernel Modules (which have high performance but are unsafe and can crash the kernel) and eBPF (which is safe but heavily restricted by its verifier, preventing Turing-complete operations like loops).

## 2. What is the new thing that it is proposing (in detail)?
The authors propose embedding a WebAssembly runtime (`wasm3`) directly inside a Linux kernel module. 
They separate the Control Path (user tooling to lifecycle the Wasm binaries via an `ioctl` device) from the Data Path (dynamically hooking into system calls using `kprobes`). 
Crucially, they implement a strict security model:
- **Zero Host Imports:** Wasm cannot access any kernel APIs.
- **Data Marshalling:** Pointers are strictly forbidden. When a syscall occurs, the kernel copies string arguments into a pre-allocated Wasm linear memory buffer before executing the Wasm function.
- **Fail-Closed Policy:** Handlers cannot sleep, and panics simply log the error and resume without crashing the kernel.

## 3. Is it something we are implementing in our paper?
**No.** While we also use WebAssembly for Turing-complete extensibility to bypass eBPF verifier limits, we run our Wasm runtime (WasmEdge/Rust Daemon) strictly in *user-space*. The CMU paper embeds Wasm directly *in the kernel*.

## 4. What ideas are better in their paper that maybe we can use and what is not better etc?
**Better/Applicable:**
- Their strict "Zero Host Imports" and sandbox isolation provide incredibly strong security guarantees, effectively making it impossible for a plugin to cause a kernel panic due to invalid memory access.
**Not Better/Not Applicable:**
- Data marshaling (copying) is inherently slow. In high-throughput 5G core networks (UPF), copying payload data on every packet is unacceptable. Our architectural choice (user-space shared memory / Ring Buffers) trades some of this embedded kernel isolation for the zero-copy line-rate speed required in telecom.

## 5. Additional Literature Analysis Questions
- **How do they handle infinite-loop prevention without crashing the kernel thread compared to our gas/fuel metering inside user restrictions?**
- **Would embedding a lightweight Wasm runtime in the kernel for *control plane* configuration updates be faster than our Sidecar/Rust daemon IPC overhead?**
