# How Mutated Test Cases Command the Raft Environment in `RunIteration`

This document explains how the fuzzer applies mutated test cases to control the Raft environment, focusing on the `RunIteration` function.

---

## 1. Loading Mutated Choices

If a mutated test case (`mimic`) is provided, its scheduling choices—such as node steps, booleans, integers, crash/restart points, and client requests—are loaded into the `traceCtx` context. This context acts as a deterministic script for the test run.

```go
if mimic != nil {
    tCtx.mimicTrace = mimic
    for i := 0; i < mimic.Size(); i++ {
        ch, _ := mimic.Get(i)
        switch ch.Type {
        case Node:
            tCtx.nodeChoices.Push(ch.Copy())
        case RandomBoolean:
            tCtx.booleanChoices.Push(ch.BooleanChoice)
        case RandomInteger:
            tCtx.integerChoices.Push(ch.IntegerChoice)
        case StartNode:
            tCtx.startPoints[ch.Step] = ch.Node
        case StopNode:
            tCtx.crashPoints[ch.Step] = ch.Node
        case ClientRequest:
            tCtx.clientRequests[ch.Step] = ch.Request
        }
    }
}
```

---

## 2. Step-by-Step Execution

For each step in the test, the fuzzer consults the loaded choices to decide:
- Which nodes to crash or restart
- Which messages to deliver between which nodes
- When to inject client requests
- How many messages to deliver

These decisions are made by popping the next choice from the corresponding queue in `traceCtx`.

---

## 3. Commanding the Raft Environment

The fuzzer then calls methods on the Raft environment to enact these choices:

- `Stop` and `Start` to crash/restart nodes
- `Step` to deliver messages or client requests
- `Tick` to advance Raft time and collect outgoing messages

```go
for j := 0; j < f.config.Steps; j++ {
    // Crash or restart nodes as dictated by the trace
    if toCrash, ok := tCtx.CanCrash(j); ok {
        f.raftEnvironment.Stop(fCtx, toCrash)
        crashed[toCrash] = true
    }
    if toStart, ok := tCtx.CanStart(j); ok {
        if isCrashed := crashed[toStart]; isCrashed {
            f.raftEnvironment.Start(fCtx, toStart)
            delete(crashed, toStart)
        }
    }

    // Deliver messages as dictated by the trace
    from, to, maxMessages := tCtx.GetNextNodeChoice()
    if _, ok := crashed[to]; !ok {
        messages := f.Schedule(from, to, maxMessages)
        for _, m := range messages {
            recordReceive(m, tCtx.eventTrace)
            f.raftEnvironment.Step(fCtx, m)
        }
    }

    // Inject client requests as dictated by the trace
    if reqNum, ok := tCtx.IsClientRequest(j); ok {
        req := pb.Message{
            Type: pb.MsgProp,
            From: uint64(0),
            Entries: []pb.Entry{
                {Data: []byte(strconv.Itoa(reqNum))},
            },
        }
        f.raftEnvironment.Step(fCtx, req)
    }

    // Advance Raft time and collect outgoing messages
    for _, n := range f.raftEnvironment.Tick(fCtx) {
        recordSend(n, tCtx.eventTrace)
        key := fmt.Sprintf("%d_%d", n.From, n.To)
        f.messageQueues[key].Push(n)
    }
}
```

---

## 4. Summary

- Every mutated test case is replayed step-by-step as a sequence of commands to the Raft environment.
- All Raft node actions, message deliveries, failures, and client requests are dictated by the mutated trace.
- This allows the fuzzer to deterministically explore new behaviors and edge cases in the Raft implementation.

**In short:**  
*The mutated test case is a script of scheduling choices. The fuzzer reads this script and, step by step, commands the Raft environment to crash/restart nodes, deliver messages, and inject client requests exactly as specified by the