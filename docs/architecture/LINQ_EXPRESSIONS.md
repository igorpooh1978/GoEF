# Typed LINQ expressions

**Proposed contract · LINQ-EXPRESSION-001 · [issue #4](https://github.com/igorpooh1978/GoEF/issues/4).** A dynamic search form can add a lower bound to a symbolic amount field. The bound value occupies a parameter slot. The expression records the field owner, operand types, nullability and operation. The SQL compiler receives typed data and can reject a mismatched field or an unregistered call before a command is sent.

| Node or action | Contract to prove |
| --- | --- |
| Field, parameter, constant | Identify the model member, alias, value type, nullable flag and parameter slot. Validate the field belongs to this query scope. Keep value bindings separate from the plan shape. |
| Boolean logic, comparisons, arithmetic | Type-check both operands. Record lifting and checked conversions explicitly. Cover two missing operands, one missing operand, false and unknown. The provider lowers the chosen null semantics. |
| Conversion and conditional | Check source/target types, branch compatibility and precision. Reject lossy casts unless the API explicitly requests the conversion. |
| Registered call | Bind a known signature to an operator or capability. Missing registration carries a typed diagnostic and source span. Registration never authorizes an arbitrary Go closure. |
| Composition, projection, visitor | Work on immutable nodes. Preserve aliases and source spans during rewriting. Visit each parameter slot consistently so a cached shape can bind different arguments. |

A marked design-time declaration provides Go syntax to a source generator. The generator resolves packages, members and captured values, emits a typed entrypoint and records its input fingerprint. The binder evaluates parameters for each invocation under a documented timing rule. W013 needs a changed-capture fixture, a stale-output build check and exact source locations for unsupported syntax. Ordinary Go compilation does not rewrite the call site. The [generated binding guide](../user/LINQ_GENERATED.md) exercises a first frontend slice with a direct field, an explicit scalar argument, and a scalar projection under [#24](https://github.com/igorpooh1978/GoEF/issues/24).

The runtime DSL and generated entrypoint lower into the same logical query representation. The symbolic subset has no requirement to compile IL, proxy dynamic binders, reproduce arbitrary reflection or execute application code to discover a plan. The [operator registry](../parity/linq-operators.json) retains expression members outside this subset so their applicability can be decided explicitly. See [expression specifications](../../specs/features/linq/expressions.feature) and the [boundary decision](../adr/0007-linq-execution-boundaries.md).
