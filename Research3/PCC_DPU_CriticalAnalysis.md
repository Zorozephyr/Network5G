# PCC-on-DPU: Deep Critical Analysis
**Architecture Under Review:** DPU runs a PCC-class process. BGP-LS and PCEP are removed **only** on the DPU↔SDN link. DPU computes local best paths per area continuously, maintains a QFI→SID-stack table, and publishes summarized results upward to the SDN controller. Each DPU is attached to a DU, which sits inside a specific router area. BGP-LS and PCEP remain everywhere else in the network.

---

## 1. What the Architect Actually Proposed (Precise Reconstruction)

```
Traditional SDN model (removed for DPU↔SDN segment):
  SDN Controller ─[BGP-LS]→ collects topology
  SDN Controller ─[PCEP]──→ pushes computed LSPs down to PCC nodes

New model for DPU↔SDN link only:
  DPU (PCC-class process)
    │  receives topology (mechanism TBD — see Issue #1)
    │  runs local path computation per area (Dijkstra / SR)
    │  maintains: QFI → SID-stack table (per slice, per egress)
    │  publishes: computed slice-path table upward to SDN
    ▼
  SDN Controller (becomes aggregator, not sole path computer)
    │  receives per-area tables from all DPUs
    │  resolves cross-area conflicts
    │  applies global TE policy
    └─[PCEP/BGP-LS remain active]─→ rest of the network
```

**Key constraint the architect set:** BGP-LS and PCEP stay active between all other nodes (routers, PCE, controllers) — only the DPU-to-SDN segment drops them. The DPU replaces that specific protocol stack with a custom publish mechanism.

---

## 2. The Eight Critical Issues

### Issue #1 — TOPOLOGY FEED: ✅ CLARIFIED → Now Opens Four New Issues

**Clarification received:** The LSDB is fed directly from the area router to the DPU's PCC process. The flow is:

```
  Area Routers (OSPF/IS-IS) → [BGP-LS] → DPU PCC (LSDB consumer)
                                                    │
                               Dijkstra/CSPF runs locally
                                                    │
                               QFI → SID-stack table
                                                    │
                               [custom publish] → SDN Controller
```

BGP-LS is **not eliminated** — it is **redirected**. The protocol remains active between the router and the DPU. What is removed is the BGP-LS session that previously went from the router (or a route reflector) upstream to the SDN controller, for the DPU's area. The SDN now receives computed outputs, not raw topology.

**This resolution creates four new issues:**

#### New Issue 1a — SDN TOPOLOGY BLINDNESS
The SDN controller no longer has LSDB visibility for any area managed by a DPU. It only sees computed slice-path tables. Consequences:
- **The SDN cannot independently verify** if a DPU's path computation is correct (no ground-truth topology to compare against)
- **The SDN cannot detect stale LSDB** on a DPU (it has no way to know the DPU's topology is 10 seconds out of date)
- **Cross-area conflict detection is degraded**: when two DPUs publish conflicting paths, the SDN knows WHAT conflicts but not WHY (it cannot inspect either area's topology to adjudicate)

This is a fundamental trust model shift: the SDN was previously a topology-aware global optimizer. It is now a **blind aggregator** that must trust DPU outputs.

#### New Issue 1b — BGP DAEMON ON DPU ARM CORES
To consume BGP-LS from the area router, the DPU must run a BGP-LS speaker process on its ARM cores. This is non-trivial:
- A full BGP daemon (e.g., FRRouting's `bgpd`) is a substantial process (~50–100 MB RSS, significant CPU on session maintenance and UPDATE processing)
- It competes directly with the existing v3 workloads: TCN inference, SHAL server, telemetry aggregation
- BGP-LS UPDATE processing during topology churn (e.g., link flap storm) can spike CPU unpredictably — exactly when the DPU most needs stable inference

**Alternative:** Use gRPC streaming telemetry (OpenConfig, gNMI) to export the LSDB from the router instead of BGP-LS. Lighter on the DPU, but requires the router to support gNMI LSDB export (not universally available).

#### New Issue 1c — DPU AS SINGLE POINT OF FAILURE FOR AREA TOPOLOGY
In the old model, the SDN had a direct BGP-LS session to the area and could compute paths independently if a DPU failed. In this model:
- If the DPU's PCC process crashes → SDN stops receiving path tables for that area
- The SDN has **no fallback**: it has no LSDB for that area, so it cannot recompute locally
- Recovery requires: DPU PCC restart + LSDB re-sync + table recompute + re-publication — potentially taking 10–60 seconds

**Question for architect:** Is there a fallback path where the area router re-establishes a direct BGP-LS session to the SDN if the DPU goes silent?

#### New Issue 1d — MULTIPLE DPUs PER AREA AMBIGUITY
If multiple DUs (and thus multiple DPUs) are attached to the same router area:
- Do all DPUs receive the same LSDB from the area router? (Yes, if BGP-LS is broadcast/multicast from the router)
- Do they independently compute and publish identical slice-path tables? (Redundancy — but then the SDN gets N copies of the same table)
- Or does one DPU act as the "area PCC master" and the others are standby? (Requires a DPU election mechanism)

**The architect must specify the multi-DPU-per-area topology** before the redundancy and failover model can be designed.

---

### Issue #2 — AREA BOUNDARY: What Exactly Is "Local Area"?

The architect says "each DPU computes for its local area." This needs precise scoping:

- Is the "area" an OSPF area? An IS-IS level? A BGP AS?
- Does the DPU compute paths **within** the area only, or also to egress points (ABRs, ASBRs)?
- If the DU's router is a border router (ABR), does the DPU's LSDB include inter-area summary LSAs or only intra-area topology?

**The architectural impact:** If the DPU only holds intra-area topology, the paths it computes can only reach egress ABRs. The SDN must then stitch: `[DPU area 1 egress] + [inter-area segment] + [DPU area 2 egress]`. This is the H-PCE model, but the inter-area stitching logic is completely unspecified.

**Concrete failure mode:** DPU in Area 1 computes: `URLLC → SID-stack [101, 102, 103-egress]`. DPU in Area 2 computes: `URLLC → SID-stack [201, 202, 203-egress]`. The SDN must produce the end-to-end SID stack: `[101, 102, 103, 201, 202, 203]`. Who computes the binding and in what order? This is non-trivial for SR-MPLS label stacking depth (typically limited to 10 labels on most hardware).

---

### Issue #3 — TABLE CONSISTENCY: Staleness and the Distributed Coherence Problem

Each DPU computes "continuously." The SDN aggregates tables from all DPUs. This creates a **distributed consistency problem** with no specified resolution protocol.

**Scenario:**
- T=0: DPU-A computes URLLC path via link L1 (cost=10). DPU-B computes URLLC path via same link L1 (cost=10).
- T=1: Link L1 fails. DPU-A detects via local OSPF in ~50ms. DPU-B detects via OSPF in ~80ms.
- T=1+50ms: DPU-A recomputes and publishes new table. SDN now has: DPU-A (new, L1-aware) + DPU-B (stale, still uses L1).
- SDN stitches a cross-area path using DPU-A's new path in Area 1 and DPU-B's stale path in Area 2 → **the stitched path traverses the failed link in Area 2's segment.**

**Questions for architect:**
1. Is the SDN allowed to use a partial table (some DPUs updated, others stale)?
2. What is the maximum age a DPU table is considered valid?
3. Who detects cross-area path conflicts: the SDN or individual DPUs?

---

### Issue #4 — THE PUBLISH INTERFACE: The Most Underspecified Layer

The architect removed PCEP upward. What replaces it?

PCEP is not just a transport — it carries:
- Delegation state (who "owns" a path: PCC or PCE)
- Path binding SIDs
- Metric constraints per path
- Operational state (active/standby)
- Error reporting

When the DPU publishes its slice-path table to the SDN, the SDN must know:
- **Which version** of the table (sequence number / timestamp)?
- **What confidence** does the DPU assign (is the LSDB fresh or stale)?
- **What constraints** were applied (optimization objective per QFI)?
- **What happens** if the SDN rejects a path? Does the DPU get an acknowledgement?

**The architect needs to define a wire format.** Options:
- Custom gRPC/Protobuf API (fast to implement, not standards-compliant)
- PCEP PCRpt messages (RFC 8231 §5.8) — PCC reporting computed path to PCE — technically PCEP could be reused in report-only mode upward
- BGP FlowSpec or BGP-LS extensions for computed path advertisement

**This is not a minor detail.** The publish interface IS the integration point with existing O-RAN NOCs and OSS/BSS stacks.

---

### Issue #5 — FAILURE HANDLING: The Race Between Local Recompute and SDN Override

When a local link fails inside the DPU's area:

1. DPU detects via local OSPF/IS-IS Hello timeout (~Dead interval: 40s default, 1-3s with aggressive BFD)
2. DPU recomputes local slice-path table
3. DPU publishes new table to SDN
4. SDN aggregates + produces new global paths
5. SDN pushes updated steering policy back down

**Problem:** During steps 1–5, traffic is being steered using the stale path. For URLLC, this is a SLA violation.

**The missing design piece:** Does the DPU apply its recomputed path immediately upon failure detection (local fast-reroute), BEFORE the SDN confirms? Or does it wait for SDN acknowledgement?

- If **local immediate apply**: The DPU becomes an autonomous forwarder. Operationally powerful, but the DPU can make steering decisions the SDN disagrees with. You need a conflict resolution protocol for when the SDN sends a different path 100ms later.
- If **wait for SDN**: You are back to centralized latency. The fast local recompute is wasted if you still wait for the controller round-trip.

**This is the core architectural tension the architect has not resolved.** It is also the most interesting research question in the entire proposal.

---

### Issue #6 — SR SID STACK ASSEMBLY: Who Builds the End-to-End Stack?

In Segment Routing, the end-to-end path is encoded as an ordered list of SIDs (the SID stack). For a cross-area path, the SID stack must include segments from multiple areas.

**The DPU only knows its local area SIDs.** It cannot build the full SID stack for an end-to-end path.

**Therefore:** The SDN must assemble the final SID stack by concatenating area-local SID stacks from multiple DPUs. But this requires:
1. A common SID namespace or translation layer across areas
2. Knowledge of inter-area binding SIDs (Adjacency SIDs, Prefix SIDs for ABRs)
3. Resolution of label stack depth limits (typically 10 SIDs max in hardware forwarding)

**Architectural question:** Does the DPU publish raw SID stacks, or abstract "egress intent" (e.g., "reach ABR-X with constraint: latency < 2ms")? The latter is cleaner architecturally but requires the SDN to translate intent into SID stacks — putting more compute back on the controller.

---

### Issue #7 — SCALE OF THE SLICE-PATH TABLE: Validating the O(10²) Claim

| Factor | Low estimate | Realistic production |
|---|---|---|
| Active QFI classes | 3 (URLLC/eMBB/mMTC) | 8–16 (3GPP TS 23.501 Table 5.7.4-1 defines 32 standardized 5QIs) |
| Active slices per QFI | 1 | 5–20 (enterprise, MVNO, IoT domains) |
| Egress paths per slice | 2–5 (primary + backup) | 10–50 (ECMP + diverse protection paths) |

**Revised table size:** 20 slices × 20 egress paths = 400 entries. Still trivially small for DPU memory (< 1 MB). The memory claim holds.

**But recompute frequency matters:** If "continuously" means event-triggered (on OSPF LSA arrival), Dijkstra on a 20-router, 50-link area is sub-millisecond per run — fine. If it means a tight polling loop, the architect must specify the interval and justify ARM core budget. The word "continuously" must be replaced with a precise trigger mechanism.

---

### Issue #8 — INTEGRATION WITH EXISTING v3 DPU ARCHITECTURE

The PCC proposal adds a new compute layer that interacts with your v3 DPUArchDesign2.md in unspecified ways:

| Conflict point | v3 Architecture | PCC proposal |
|---|---|---|
| ARM core budget | 16 A78 cores: telemetry, TCN inference, SHAL server, model lifecycle | PCC process + Dijkstra + table maintenance + publish daemon now also need cores |
| Slice identity | `hw_slice.qfi` + `hw_slice.slice_id` (local DPU construct) | QFI → SID-stack table (network-wide path construct) |
| Control authority | DPU autonomously adjusts bandwidth via eSwitch meters | SDN sets paths via DPU-published tables |
| Northbound interface | SHAL (gRPC/Protobuf) to management plane | New publish protocol to SDN controller |

**Specific unresolved interaction:** Does the SID-stack table from the PCC process feed INTO the `hw_slice` struct's forwarding decision? When the bandwidth AI says URLLC needs more capacity, does it use the locally-computed SID stack to steer traffic to an alternative path? Or does it only adjust the local meter and leave path selection entirely to the SDN? **These are two completely different control loops that need explicit coordination.**

---

## 3. Questions for the Architect (22 Questions, Organized by Layer)

### Layer 1: Topology Source (Resolve First — Everything Depends on This)

1. What is the exact source of topology data for the DPU's local path computation — the area router via BGP-LS, a direct OSPF adjacency, or a controller-pushed snapshot?
2. Is the DPU running a routing protocol instance (OSPF/IS-IS) or is it purely a passive LSDB consumer?
3. What is the LSDB scope: intra-area only, or including inter-area summaries?
4. What is the update latency of topology data reaching the DPU, and what is the maximum tolerated staleness for URLLC path computation?

### Layer 2: Area Boundary and Scope

5. How is "local area" defined: OSPF area, IS-IS level, BGP AS, or a custom administrative boundary?
6. Does the DPU compute paths only to intra-area destinations, or also to egress ABRs/ASBRs?
7. If the DU's router is itself a border router, does the DPU see both sides of the area boundary?

### Layer 3: Path Computation Logic

8. What path computation algorithm is intended: Dijkstra for IGP shortest path, CSPF (Constrained Shortest Path First) for multi-constraint paths, or a custom algorithm per QFI?
9. What are the per-QFI optimization objectives — latency only for URLLC, bandwidth for eMBB, hop count for mMTC, or a weighted combination?
10. Is ECMP (Equal-Cost Multi-Path) expected in the slice-path table, or only a single best path per QFI?
11. Does the DPU compute backup paths (protection LSPs) in addition to primary paths?
12. What does "continuously" mean precisely — event-triggered on OSPF LSA arrival, or a fixed polling interval?

### Layer 4: Publish Interface (Most Critical Unspecified Element)

13. What protocol carries the DPU's slice-path table to the SDN — a custom API, PCEP PCRpt messages (RFC 8231), or something else?
14. What does the publish message contain: full table replacement, incremental diffs, or event-triggered updates?
15. Does the SDN send acknowledgements? Can the SDN reject a DPU-computed path?
16. How does the DPU know if the SDN has consumed its latest table (sequence numbering, heartbeat, expiry timer)?

### Layer 5: Failure Handling and Fast Reroute

17. When a local link fails: does the DPU apply its recomputed path immediately (local autonomy) or wait for SDN confirmation?
18. If the DPU acts immediately, what is the conflict resolution protocol when the SDN sends a different path 100ms later?
19. What is the recovery time objective (RTO) for URLLC paths after a link failure?

### Layer 6: Cross-Area Path Stitching

20. Who assembles the end-to-end SID stack for cross-area paths: the SDN, or does each DPU publish "egress intent" that the SDN translates?
21. How are inter-area binding SIDs managed — is there a global SID namespace, or per-area namespaces that need translation?

### Layer 7: Integration with v3 DPU Architecture

22. Does the PCC-computed SID stack influence the local eSwitch forwarding decision (flow rules), or is it only published upward while the DPU continues using existing local path steering?

---

## 4. Six Concrete AI Use Cases

### AI Use Case 1 — Predictive Path Pre-computation (SDN Controller)

**Mechanism:** AI model inside the SDN ingests per-DPU slice-path table history + BGP-LS TE metric streams (link utilization as TE attributes from the router layer, which still runs BGP-LS). Predicts P(link_L_in_area_A becomes bottleneck in next T seconds).

**Action:** Pre-position alternate SID-stack recommendations to the relevant DPUs before congestion materializes. The DPU can adopt the suggestion or override with its local computation.

**Model class:** Time-series regression (LSTM or TCN) per link. Binary classification: congestion/not-congestion within horizon T. Small feature space: link utilization time series is a scalar per link.

**Training data:** BGP-LS TE attribute history (link utilization, delay, loss) + ground truth congestion labels from operator NOC.

**Novel claim:** Closes the loop between reactive DPU local computation and proactive SDN global optimization. Neither purely reactive (current SDN) nor purely proactive (this fills the gap).

---

### AI Use Case 2 — Per-QFI Path Scoring Beyond Dijkstra (Inside DPU)

**Mechanism:** The DPU's PCC generates K candidate paths (K-shortest paths, K=5–10) using classical CSPF. An AI model then scores each candidate path per QFI class:

$$\text{score}(p, \text{QFI}) = f(\text{latency}_p, \sigma^2_{\text{jitter}, p}, \text{failure\_rate}_p, \text{time\_of\_day})$$

The top-scored path per QFI is inserted into the slice-path table.

**Input features:** Per-link historical telemetry from the DPU's observation of local LSDB TE attributes + time-of-day encoding + QFI class embedding.

**Model class:** Lightweight MLP (< 1000 parameters, fits in ARM L1). Can be quantized to INT8 alongside the existing TCN.

**Novel claim:** Combines hard constraint satisfaction (CSPF eliminates infeasible paths) with soft learned scoring (MLP ranks feasible paths by historical reliability). Pure CSPF ignores historical reliability; pure ML ignores hard constraints. The composition is the contribution.

---

### AI Use Case 3 — Link Failure Prediction for Preemptive Path Migration (DPU)

**Mechanism:** Each DPU monitors early warning signals of impending link degradation: increasing OSPF Hello retransmission counts, rising BFD detection intervals, incrementing interface error counters. AI model trained on these precursor time series predicts link failure 5–30 seconds before it occurs.

**Action:** DPU pre-computes the alternate path and pre-installs it as a standby SID stack. When the failure occurs, switchover is instantaneous — no recompute or SDN round-trip needed.

**Why critical for URLLC:** The 1–3 second BFD detection + recompute + SDN round-trip window violates URLLC SLA. Prediction-based pre-provisioning eliminates the recovery window entirely.

**Model class:** LSTM on OSPF/BFD counter time series with binary failure label. Even a non-ML heuristic (exponentially-smoothed rate-of-change threshold) is publishable if validated on real failure traces from a carrier network.

---

### AI Use Case 4 — Cross-Area Path Conflict Resolution (SDN Controller)

**Mechanism:** When multiple DPUs submit slice-path tables that, when stitched, create cross-area conflicts (two areas routing high-demand slices through the same inter-area link), an AI model in the SDN resolves conflicts by selecting which area's path to prefer.

**Input:** All per-DPU slice-path tables + inter-area link utilization (from BGP-LS TE attributes, still active between routers and SDN) → modeled as a graph.

**Output:** Conflict resolution decision: Area A keeps its path, Area B recomputes with constraint C (e.g., "avoid inter-area link X").

**Model class:** Graph Neural Network (GNN) where nodes are areas, edges are inter-area links. GNN maps the multi-area path selection problem to a learned assignment. Published precedent: Geyer & Carle, "Learning and Generating Distributed Routing Protocols Using Graph-Based Deep Learning," Big-DAMA 2018.

**Why AI instead of rules:** In a multi-area network with N DPUs each publishing K paths for M slices, the conflict graph is NP-hard to resolve optimally (maps to multi-commodity flow). AI provides fast approximate solutions within the controller's policy latency budget.

---

### AI Use Case 5 — Adaptive Recompute Interval Tuning (Inside DPU)

**Mechanism:** The DPU does not need to recompute paths at a fixed interval. When the network is stable, recomputing every 10 seconds is sufficient. During high churn (many LSA changes), recomputing every 100ms is necessary. An AI controller dynamically adjusts the recompute interval based on observed LSDB change rate.

**Input state:** Rate of OSPF LSA arrivals in last N seconds, number of paths that changed in last recompute, current slice SLA violation count.

**Output:** Next recompute interval (ms).

**Why non-trivial:** Too-frequent recomputation wastes ARM cores competing with TCN inference. Too-infrequent causes stale tables. The optimal interval is a function of current network dynamics — a classic adaptive control problem.

**Model class:** Tabular Q-learning or a linear function approximator. Does not need a neural network. Explicitly demonstrable on DPU hardware with minimal compute overhead. This is the **easiest AI insertion point to implement and validate.**

---

### AI Use Case 6 — Distributed Slice-Path Table Anomaly Detection (SDN, Federated)

**Mechanism:** The SDN runs an anomaly detector over all published DPU tables to identify:
- **Dead routes:** paths that no DPU has chosen in the last N intervals (routing black holes)
- **Hot routes:** paths that all DPUs are concentrating on (congestion precursor not yet visible in TE metrics)
- **Silent DPUs:** DPUs whose tables have not updated within expected intervals (PCC process failure or connectivity loss)

**Input:** Slice-path table update stream from all DPUs (timestamps, path selections, SID stacks, metric values).

**Output:** Anomaly flag with type classification and affected area/DPU.

**Model class:** Multivariate time-series anomaly detection. LSTM autoencoder or Isolation Forest over the table-update feature vectors. Online training — no labeled failure data required.

**Why publishable:** This is the **observability layer** for the distributed PCC architecture. Without it, the system is a black box to the operator. Anomaly detection over distributed path computation outputs is a novel contribution in the context of disaggregated SDN. No existing work addresses this specific observability gap.

---

## 5. Summary Assessment Table

| Dimension | Status | Risk Level |
|---|---|---|
| PCC on DPU ARM cores (compute feasibility) | **Feasible** — PCEP client is control-plane, A78 handles it | Low |
| Topology feed to DPU | **Unresolved** — exact source not specified | **Critical** |
| Area boundary definition | **Unresolved** — OSPF area? IS-IS level? custom? | High |
| Slice-path table size | **Fine** — O(400 entries), trivial for DPU memory | Low |
| Continuous recompute overhead | **Fine** — Dijkstra on small area graph is sub-ms, but "continuously" needs precision | Medium |
| Publish interface to SDN | **Unresolved** — no wire format, no ack model, no version control | **Critical** |
| Failure handling / fast-reroute | **Unresolved** — local autonomy vs. SDN confirmation tension is unresolved | **Critical** |
| Cross-area SID stack assembly | **Unresolved** — who stitches multi-area paths? | High |
| Integration with v3 DPU (TCN/SHAL/ARM cores) | **Unresolved** — ARM core budget conflict, dual control loops not coordinated | High |
| AI insertion points identified | **6 concrete, tractable use cases** | Low |

> [!IMPORTANT]
> The three **Critical** items (topology feed, publish interface, failure handling) must be resolved before the architecture can be considered internally consistent. All other issues are resolvable in design iteration. These three are foundational — everything else depends on them.

---

*Analysis prepared June 2026. Based on architect proposal transcript and v3 DPUArchDesign2.md. BGP-LS/PCEP removal scope: DPU↔SDN link only. BGP-LS and PCEP remain active between all routers, PCE, and SDN controller in the rest of the network.*
