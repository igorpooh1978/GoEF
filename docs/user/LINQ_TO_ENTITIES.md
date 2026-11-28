# Compose a database-backed LINQ query

GoEF can compile one PostgreSQL equality filter and an explicit projection of mapped Go fields. Compile once and supply a typed value at each execution. The compiler quotes schema, table and column names and sends the value in a separate parameter slot. [Issue #36](https://github.com/igorpooh1978/GoEF/issues/36)

## Read settings by key

Define the row according to the existing table. Pointer fields represent SQL NULL.

```go
type Setting struct {
    Key         string     `goef:"column=setting_key"`
    Revision    int64      `goef:"column=saved_revision"`
    Applied     *int64     `goef:"column=applied_revision"`
    SavedAt     time.Time  `goef:"column=saved_at"`
    AppliedAt   *time.Time `goef:"column=applied_at"`
}

mapped, err := model.Map(model.Struct[Setting]("Setting", []string{"Key"},
    model.ToTable("application_settings")))
if err != nil {
    return err
}
plan, err := queryable.Compile(
    postgres.EqualityCompiler{Schema: "public"},
    mapped.Entities()[0],
    expressions.EqualField[Setting, string]("Key"),
    "Revision", "Applied", "SavedAt", "AppliedAt",
)
if err != nil {
    return err
}
setting, found, err := plan.Fetch(ctx, backend, key)
```

Import `time` and the GoEF packages `model`, `linq/expressions`, `linq/queryable` and `providers/postgres`. Use Go 1.27.4, PostgreSQL 18.7 and the adapter built with pgx v5.13.0 for the verified configuration. Construct `backend` using the [PostgreSQL adapter](../providers/POSTGRESQL.md) and pass a context with the application's query budget. The [live example](../../test/conformance/driverlab/postgres_queryable_test.go) exercises the call against PostgreSQL 18.7. Compile and execute it with `python -B test/conformance/lab/postgres_queryable.py`.

- A match returns a detached `Setting` and `found == true`. Unselected fields keep their Go zero values.
- A missing match returns the zero result, `false` and `nil`. A second match returns `goeferr.ErrMultipleResults`.
- An invalid mapping or unsupported field type returns `goeferr.ErrQueryTranslation` before acquiring a connection. A canceled read returns its cause. GoEF closes the result stream and releases the lease before returning. The caller owns the pool.

For a [generated equality binding](LINQ_GENERATED.md), pass the binding to `queryable.CompileGenerated` and execute the returned scalar plan using `Fetch(ctx, backend, binding.Value())`. The scalar comes from the SQL projection.

## Supported scope

This path accepts signed integers, booleans, strings, `float64`, byte slices and `time.Time`, along with pointers to those types in projected fields. The caller maps fields to PostgreSQL columns that the pgx `database/sql` driver can read as the corresponding transport types. UUID, JSONB, arrays and exact decimal values need separately supported mappings. The equality argument is non-null. Equality matches only non-null predicate columns. PostgreSQL governs equality and collation.

The API reads a single mapped relation with one predicate. Entity tracking, transaction-bound execution, joins, ordering, collection terminals and the other provider dialects remain outside this read path. Once materialized, the result is an ordinary Go value that can be used with LINQ to Objects. [Architecture and remaining cases](../architecture/LINQ_QUERYABLE.md)
