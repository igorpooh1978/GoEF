# Exact decimal values in Go

[Issue #3](https://github.com/igorpooh1978/GoEF/issues/3) · [Model type scenarios](../../../specs/features/model/model-type-001.feature) · [Implementation](../../../model/decimal.go) · [Tests and example](../../../model/decimal_test.go)

## Contract

`model.ParseDecimal` accepts plain ASCII decimal text with an optional sign and fractional part. Each side of a decimal point requires digits. Canonical text removes redundant leading integer zeros, a leading plus, and the sign of zero. It keeps all fractional digits, including trailing zeros. For example, `+0012.300` becomes `12.300` with precision 5 and scale 3. The zero value is `0` with precision 1 and scale 0. Go equality includes scale. Each text round trip preserves both value and scale.

The parser bounds precision to 1000 digits and input to 1003 bytes, including a possible sign, point, and leading zero. Values below one can have all 1000 digits after the point. Decoding failure leaves the receiver unchanged and reports an error without echoing the supplied text. Text encoding returns caller-owned bytes, and decoding retains no input buffer. The Go value and text codec complete the provider-independent slice. The existing SQL precision and scale scenario still requires provider-specific binding, codecs, and real-engine evidence.

## Upstream comparison

The pinned EF Core mapping accepts distinct precision and scale facets for a decimal property. The pinned .NET runtime also uses a finite base-10 coefficient and scale, with its own 96-bit coefficient and scale range. GoEF's text contract preserves scale as part of Go value identity and uses the stated parser bound. Its text interfaces follow the pinned Go standard library's encode and decode ownership contract. All implementation and tests were written for GoEF. [Pinned EF Core decimal mapping](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/src/EFCore.Relational/Storage/DecimalTypeMapping.cs) · [Pinned .NET decimal implementation](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Private.CoreLib/src/System/Decimal.cs) · [Pinned .NET boundary tests](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/src/libraries/System.Runtime/tests/System.Runtime.Tests/System/DecimalTests.cs) · [Go 1.27.4 encoding interfaces](https://github.com/golang/go/blob/go1.27.4/src/encoding/encoding.go)

## Workstation checks

Windows/amd64, Go 1.27.4. The executable tests cover syntax and precision boundaries, owned text round trips, values beyond binary floating-point precision, zero, scale-sensitive equality, and receiver state on rejection. Executable source revision `57a54d897f500754fa71dcce55464a954c964a89` follows baseline `85799b679601610a1fb72971b74c38a40d9f7224`.

- RED (exit 1) on the baseline with the new tests. `go test -mod=readonly -count=1 -run '^TestDecimal' ./model` failed to compile because the public decimal API was absent.
- GREEN (exit 0). `go build -mod=readonly ./...`, `go vet -mod=readonly ./...`, and `go test -mod=readonly -count=1 -shuffle=on ./...` passed.
- Coverage (exit 0). `go test -mod=readonly -count=1 -shuffle=on -coverprofile=<profile> ./model` covered 345 of 349 model statements (98.9%) and 53 of 53 decimal implementation statements (100%).
- Fuzz (exit 0). `go test -mod=readonly -run '^$' -fuzz '^FuzzDecimalTextRoundTrip$' -fuzztime 4s -parallel 2 ./model` processed 8,734 inputs.
- Race (blocked, exit 2). `go test -mod=readonly -race -count=1 -shuffle=on ./model` requires CGO. This Windows invocation used `CGO_ENABLED=0`.
