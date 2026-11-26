module github.com/igorpooh1978/GoEF/providers/mysql

go 1.27.4

require (
	github.com/go-sql-driver/mysql v1.10.0
	github.com/igorpooh1978/GoEF v0.0.0-20261126162303-a2b0babac18d
)

require filippo.io/edwards25519 v1.2.0 // indirect

replace github.com/igorpooh1978/GoEF => ../..
