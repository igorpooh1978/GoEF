module github.com/igorpooh1978/GoEF/providers/mysql

go 1.27.4

require (
	github.com/go-sql-driver/mysql v1.10.0
	github.com/igorpooh1978/GoEF v0.1.0-alpha.1.0.20261128074836-493db45c51d9
)

require filippo.io/edwards25519 v1.2.0 // indirect

replace github.com/igorpooh1978/GoEF => ../..
