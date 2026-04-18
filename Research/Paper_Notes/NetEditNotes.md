# Summary: NetEdit - Orchestration Platform for eBPF Network Functions at Scale

## 1. What is the paper about?
NetEdit is a large-scale orchestration framework built by Meta. It focuses on the problem of dynamically managing, deploying, and verifying thousands of eBPF programs across millions of diverse servers without causing network disruption.

## 2. What is the new thing that it is proposing (in detail)?
NetEdit completely decouples the eBPF data plane from its userspace control plane to ensure that orchestrator upgrades do not drop network connections.
Key mechanisms include:
- **BPFAdapter & SharedMaps:** An abstraction to handle lazy loading across varied eBPF kernel hooks securely.
- **Explicit Garbage Collection:** Leveraging the eBPF filesystem pinning feature so programs survive userspace orchestrator crashes.
- **bpf-iter:** Instead of waiting for new connection events, they retroactively apply loaded policies against actively running connections.
- **PolicyEngine & Initializer:** A centralized controller determining configuration parameters based on cluster placement and routing contexts.

## 3. Is it something we are implementing in our paper?
**Yes.** We are implementing the cluster-level control concept. Our Custom Go-based Kubernetes Operator mirrors their "PolicyEngine", watching WasmPlugin CRDs and pushing state to UPF pods. Our "Operator Crash (Control Plane Failure)" strategy uses the exact same eBPF VFS pinning trick to survive control-plane outages. 

## 4. What ideas are better in their paper that maybe we can use and what is not better etc?
**Better/Applicable:**
- The concept of using `bpf-iter` to retroactively update active flows is brilliant. While our atomic map swap applies to the *next* packet, applying rules retroactively to buffered packets could be a useful optimization for our drain cycles.
**Not Better/Not Applicable:**
- NetEdit is focused primarily on low-level TCP connection tuning, congestion control, and telemetry spanning pure eBPF functions. It does not address payload-level application logic (String matching, DPI), which is why we must introduce WebAssembly.

## 5. Additional Literature Analysis Questions
- **How does NetEdit guarantee state consistency when an entire server reboots unexpectedly, and can this be applied to our UPF sidecars?**
- **How do they manage conflicting tuning intents applied to the same hookpoint compared to our targeted TEID-based architecture?**
