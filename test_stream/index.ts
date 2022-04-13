import { NodeFileStream } from '../client/src/upload'
import { statSync, createReadStream } from "fs"

import yargs from 'yargs'
import { hideBin } from 'yargs/helpers'

const argv = yargs(hideBin(process.argv)).options({
	files: { type: 'string', array: true, demandOption: true },
}).parseSync()

console.log(`Files: ${argv.files}`)

async function main() {
	for (const path of argv.files) {
		const stream = new NodeFileStream(createReadStream(path))
		const total = statSync(path).size

		let left = total
		while (left > 0) {
			const buff = await stream.read()
			if (buff) {
				left -= buff.length
			}
		}

		if (left !== 0) {
			console.error("Did not read fully, remaining", left)
		} else {
			console.log("PASS", path)
		}
	}
}

main()
