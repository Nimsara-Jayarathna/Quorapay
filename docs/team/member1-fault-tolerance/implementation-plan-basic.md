# Member 1 Fault Tolerance - Basic Implementation Plan

## Goal
Make the system stay reliable when nodes crash, restart, or rejoin.

## What Member 1 Will Build
- A simple failure handling model.
- A clear recovery flow for crashed/restarted nodes.
- Basic resilience test scenarios with expected results.

## Step 1 - Define Failure States
- Add simple node states:
  - `HEALTHY`
  - `FAILED`
  - `RECOVERING`
  - `REJOINED`
- Define when the node moves from one state to another.
- Keep transitions easy to read and deterministic.

## Step 2 - Wire State Changes to Coordination Events
- Listen to ZooKeeper/membership events.
- When leader/follower disappears, mark as `FAILED`.
- When node comes back, mark as `RECOVERING`, then `REJOINED` after sync.

## Step 3 - Implement Recovery on Startup/Rejoin
- On restart, load local durable state.
- Reconnect to coordination service.
- If not up to date, catch up from leader.
- Only return to normal serving mode after catch-up is complete.

## Step 4 - Improve Status Visibility
- Extend `/status` output with:
  - current fault-tolerance state
  - recovery in progress (true/false)
  - last recovery time
  - last failure reason
- Make failover and recovery easy to observe.

## Step 5 - Add Basic Failure Tests
- Test 1: leader crash -> follower becomes new leader.
- Test 2: follower crash -> follower restarts and catches up.
- Test 3: repeated restart of one node -> cluster remains available.
- Test 4: temporary node loss -> ledger converges after rejoin.

## Step 6 - Validate and Document
- Run all tests using local scripts.
- Record expected vs actual behavior.
- Document known limits and next improvements.

## First Coding Tasks (Start Here)
1. Add fault-tolerance state fields in runtime status model.
2. Add transition helper methods for state changes.
3. Hook transitions to startup/shutdown/rejoin events.
4. Add minimal test for leader crash and successful re-election.

## Definition of Done
- Node state transitions are visible and correct.
- Restarted node can rejoin safely.
- Basic crash/recovery tests pass.
- Behavior is documented in Member 1 docs.
