import { Transfer, File } from '../client/src/client/client'
import { NodeFileStream } from '../client/src/stream'
import { statSync, createReadStream } from 'fs'
import { basename } from 'path'

import yargs from 'yargs'
import { hideBin } from 'yargs/helpers'

const argv = yargs(hideBin(process.argv)).options({
	url: { type: 'string', demandOption: true },
	files: { type: 'string', array: true, demandOption: true },
	timeout: { type: 'number', default: -1 }
}).parseSync()

console.log(`URL: ${argv.url} Files: ${argv.files}`)

async function loadFiles() {
	const files: File[] = []
	for (const path of argv.files) {
		const size = statSync(path).size
		files.push({
			Name: basename(path),
			Type: "application/octet-stream",
			Size: size,
			data: new NodeFileStream(createReadStream(path)),
		})
	}
	return files
}

async function main() {
	const t = new Transfer(
		argv.url,
		await loadFiles(),
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
}

main()