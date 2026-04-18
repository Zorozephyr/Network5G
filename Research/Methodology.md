# Methodology

## Phase 1: Baseline (Weeks 1-2)
*   **Literature**: Execute three-pass reading of Tier 1 papers (SPRIGHT, Wasm-bpf) focusing on limitations.
*   **Datapath**: Adopt existing frameworks (Eunomia-bpf or SPRIGHT shared-memory) instead of building custom eBPF parsers.
*   **Metrics**: Define key evaluation metrics (e.g., orchestration latency, packet loss, CPU overhead).

## Phase 2: Prototype (Weeks 3-4)
*   **Sandbox**: Provision an isolated Linux VM (Ubuntu 22/24, kernel >= 5.15).
*   **eBPF Component**: Write minimal eBPF C program to intercept packets and forward payloads to a BPF Ring Buffer.
*   **Wasm Component**: Write a lightweight Wasm program (Rust/Go via WasmEdge) to read and print from the Ring Buffer.
*   **Integration**: Build a Go script (`cilium/ebpf`) to load the eBPF program and bridge the ring buffer to Wasm.

## Phase 3: Engineering (Weeks 5-8)
*   **Operator**: Scaffold a Go-based Kubernetes Operator using Kubebuilder.
*   **CRD Definition**: Create `WasmPlugin` CRD (parameters: Wasm URL, target UPF pod selector, slice ID).
*   **Hitless Injection**: Implement reconciliation loop to:
    *   Detect new intents and fetch Wasm binaries.
    *   Inject Wasm into the target UPF pod (via shared volume/socket).
    *   Update eBPF Kernel Maps to route traffic seamlessly without pod restarts.

## Phase 4: Benchmarking (Weeks 9-10)
*   **Testbed**: Deploy K8s cluster (preferably bare-metal for final numbers).
*   **Traffic Generation**: Use TRex/Pktgen for high-throughput simulated GTP-U traffic (1M pps).
*   **Experimentation**: 
    *   Inject `WasmPlugin` during steady-state traffic.
    *   Measure orchestration latency (`kubectl apply` to first processed packet).
    *   Validate 0% packet drop rate during eBPF map updates.

## Phase 5: Synthesis & Writing (Weeks 11-12)
*   **Visuals**: Generate system architecture diagrams and clean data graphs (e.g., CDF latency plots).
*   **Narrative Structure**:
    *   *Intro*: Agility needs vs. UPF rigidity.
    *   *Background*: eBPF speed + Wasm flexibility vs. orchestration challenges.
    *   *Design*: Go Operator architecture.
    *   *Evaluation*: Hitless update proofs via benchmarks.
*   **Review**: Conduct peer critique of methodology prior to submission.
