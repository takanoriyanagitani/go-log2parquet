#!/bin/sh

outname="./sample.parquet"

geninput(){
	jq -c -n '[
		{timestamp:"2025-10-02T00:45:32.012345Z", severity:"info", body:"apt update done", status:200, ip:"127.0.0.1", method:"POST"},
		{timestamp:"2025-10-02T00:45:32.012345Z", severity:"warn", body:"apt update failure", status:500, ip:"127.0.0.1", method:"POST"}
	]' |
		jq -c '.[]'
}

conv_native(){
	echo converting the json logs to a parquet...
	cat |
		./generic-json-logs2parquet > "${outname}"
}

conv_wasi(){
	echo converting the json logs to a parquet using wazero...
	cat |
		wazero run ./generic-json-logs2parquet.wasm > "${outname}"
}

conv(){
	which wazero | fgrep -q wazero && conv_wasi
	which wazero | fgrep -q wazero || conv_native
}

geninput | conv

parquet-read --json ./sample.parquet |
	jq -c |
	dasel --read=json --write=yaml |
	bat --language=yaml
