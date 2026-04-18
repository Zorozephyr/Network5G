# Summary: Orchestrating Network Security Borders in the Computing Continuum (Liqo / Tornesello, 2026)

## 1. What is the paper about?
This thesis/paper explores the complex network orchestration required when scaling Kubernetes from single centralized clusters to multi-tenant, federated clusters (the Computing Continuum) using the Liqo framework. It focuses on ensuring strict network isolation perimeters between mutual untrusted tenants.

## 2. What is the new thing that it is proposing (in detail)?
The paper implements an advanced dynamic controller/Network Manager within Liqo. This controller dynamically translates high-level multi-tenant security intents into low-level Netfilter and emerging eBPF data-plane primitives. 
Critically, it defines "Data Plane State Preservation" techniques: ensuring that when a security rule changes or an orchestrator updates, the data plane does not enter an inconsistent, "fail-open," or degraded state during the translation phase.

## 3. Is it something we are implementing in our paper?
**Yes.** The state preservation challenge translates identically to our "Hitless Lifecycle Management." Our architecture's core novelty is executing an "Atomic Swap" using kernel RCU locks. We transition from V1 to V2 of a Wasm plugin by flipping an eBPF map pointer in a single CPU cycle to prevent any packet drops or inconsistent processing states, just as they preserve boundary states.

## 4. What ideas are better in their paper that maybe we can use and what is not better etc?
**Better/Applicable:**
- Their rigorous analysis of intermediate state instabilities during rule propagation solidifies the necessity of an atomic update mechanism. Their evaluation can serve as theoretical backing for our "Drain Cycle" and map swap designs.
**Not Better/Not Applicable:**
- Their scope addresses complex multi-cluster federation (Liqo) and broad firewall definitions. We operate at the micro-level: upgrading individual Turing-complete Deep Packet Inspection functions on a single UPF node seamlessly.

## 5. Additional Literature Analysis Questions
- **How do they handle residual connections or state migration between older and newer rulesets compared to our explicit 100ms graceful "Drain Cycle"?**
- **Does their controller architecture support graceful degradation (e.g., reverting safely backwards) if the new policy fails validation on deployment?**
