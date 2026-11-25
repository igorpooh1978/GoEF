---
name: goef-transactions
description: Define GoEF transaction ownership, connection affinity, isolation, savepoints, bounded retry, external transactions, cancellation and ambiguous commits.
---

# GoEF Transactions

1. Read the [transaction contract](../../../docs/architecture/TRANSACTION_ENGINE.md) and [errors](../../../docs/architecture/ERROR_CONTRACT.md). Model GoEF-owned and borrowed transactions explicitly. Map capabilities for isolation, savepoints and cross-context sharing.
2. Bind all transactional commands to the same connection or transaction. Specify finalization and cleanup ownership for rows, handles, rollback and commit. Check nested/savepoint behavior and isolation on the actual engine and driver.
3. Distinguish retryable failure before commit from an unknown commit outcome. Limit retries to a replay-safe unit with a fresh transaction and consistent tracker state, and preserve cancellation. Define how the caller observes unreconciled outcomes.
4. Exercise external ownership, late read/commit failure, savepoint rollback, cancellation, disconnection, and concurrency with controlled faults. Capture operation phases and state before and after the error.

If a connection drops after a commit request, record the ambiguous outcome and follow the reconciliation contract. A transient transport classification alone cannot authorize replay.
