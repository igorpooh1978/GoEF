---
name: goef-tracking
description: Work on GoEF identity resolution, change tracking, graph fixup, optimistic concurrency, Unit of Work, SaveChanges, or rollback state transitions.
---

# GoEF Tracking

1. Read [Change Tracker](../../../docs/architecture/CHANGE_TRACKER.md) and [transaction contract](../../../docs/architecture/TRANSACTION_ENGINE.md). Locate matching EF Core behavior and regressions.
2. Write a state-transition table covering identity map, snapshots, graph fixup, generated values, concurrency tokens and save ordering. Specify tracking and no-tracking modes and lifetime of a context.
3. Distinguish changes sent to the database, state accepted after success and states following savepoint rollback, external transaction decisions or uncertain commit. Check zero rows for concurrency where required. Use the typed error contract.
4. Exercise duplicate tracked keys, relationship cycles, partial write failure, key/token restoration, cancellation and whole-operation replay. Record both in-memory and database outcomes with exact issue/scenario links.

For a batch ending in an optimistic concurrency conflict, verify earlier database writes roll back as required and the tracker state still lets the caller apply the documented resolution.
