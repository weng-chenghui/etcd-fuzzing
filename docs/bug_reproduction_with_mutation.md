# How Mutations Can Reproduce Bugs in etcd-raft

This document explains how the mutation strategies in this repository can reproduce both seeded and new bugs in etcd-raft, focusing on the choices made by the mutator and their impact on the system.

---

## 1. What the Mutator Changes

The mutator generates new test cases by altering:
- **Crash/restart schedules:** Which nodes are down or up at each step.
- **Message delivery:** Which messages are delivered, delayed, or dropped between nodes.
- **Client request timing:** When and to which node client requests are sent.
- **Message batch size:** How many messages are delivered at once.

---

## 2. How This Can Trigger Bugs

### Seeded Bug (Quorum Condition)
- If the quorum check is weakened (e.g., from n/2+1 to n/3+1), a test case where only a minority of nodes are up (due to crashes; can be controlled by the mutation of test cases) and messages are delivered only among them can allow a leader to be elected incorrectly.
- Mutations that crash just enough nodes and deliver just the right messages can reproduce this bug.

### New Bug (Missing Snapshot Crash)
- Mutations may create a scenario where:
  - A node is crashed for a long period (crash schedule).
  - The rest of the cluster continues, possibly compacting logs and creating a snapshot.
  - The crashed node is restarted (restart schedule) and tries to catch up.
  - If the snapshot it needs is missing (due to message delivery choices or timing), the node may crash.
- The fuzzer, by mutating crash/restart schedules and message deliveries, can create this rare interleaving.

---

## 3. Why ModelFuzz Finds the New Bug

- **Coverage-Guided Mutation:** ModelFuzz uses feedback from the TLC model checker to guide mutations toward unexplored or interesting states, increasing the chance of hitting subtle bugs.
- **Systematic Exploration:** By mutating not just randomly but also based on coverage, ModelFuzz can generate rare schedules (e.g., long node downtime, specific message drops) that are unlikely in pure random search.

---

## 4. Example Mutation Sequence

The mutator creates a test where:
1. Node 2 is crashed at step 5 and restarted at step 40.
2. During this time, nodes 1 and 3 continue, compact logs, and create a snapshot.
3. Upon restart, Node 2 requests a snapshot that is no longer available.
4. The system crashes, revealing the bug.

---

## 5. Mutation Types and Possible Bug Triggers

| Mutation Type         | Possible Bug Triggered                |
|-----------------------|---------------------------------------|
| Crash/Restart Nodes   | Node misses snapshot, triggers crash  |
| Message Delivery      | Delayed/lost snapshot messages        |
| Client Request Timing | Requests at critical recovery moments |
| Max Messages          | Batching affects state propagation    |

---

**In summary:**  
By systematically mutating crash/restart schedules, message deliveries, and client request timings, the fuzzer can create rare and complex scenarios that expose subtle bugs—such as a node crashing when a required snapshot is missing. Coverage-guided mutation (as in ModelFuzz) increases the likelihood of finding such bugs compared to random search.