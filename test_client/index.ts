import { Transfer } from 'websocket-ftp'
import { readFileSync } from 'fs'
import { basename } from 'path'

import yargs from 'yargs'
import { hideBin } from 'yargs/helpers'

const argv = yargs(hideBin(process.argv)).options({
	url: { type: 'string', demandOption: true },
	files: { type: 'string', array: true, demandOption: true },
	timeout: { type: 'number', default: -1 }
}).parseSync()

console.log(`URL: ${argv.url} Files: ${argv.files}`)

const t = new Transfer(
	argv.url,
	argv.files.map((path) => {
		const b = readFileSync(path)
		return {
			Name: basename(path),
			Size: b.length,
			Type: "application/octet-stream",
			data: b,
		}
	}),
	{
		onstart: () => {
			if (argv.timeout > 0)
				setTimeout(t.cancel, argv.timeout)
		},
		onprogress: (r, t) => console.log(`Sent ${r} / ${t} bytes`),
		onsuccess: () => {
			console.log("Successfully completed transfer")
		},
		onclose: () => {
			console.log("Closed WS connection")
			process.exit()
		},
	},
	true
)
