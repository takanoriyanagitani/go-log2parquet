package main

import (
	"log"

	lp "github.com/takanoriyanagitani/go-log2parquet"
)

func main() {
	e := lp.StdinToGenericLogsToParquetToStdoutDefault()
	if nil != e {
		log.Fatal(e)
	}
}
