import { WebSocket } from "isomorphic-ws"
import { Receiver } from "../client/src/receiver/receiver"

import { openSync, readSync } from "fs"
import { basename } from "path"

import yargs from 'yargs'
import { hideBin } from 'yargs/helpers'

const argv = yargs(hideBin(process.argv)).options({
	url: { type: 'string', demandOption: true },
	files: { type: 'string', array: true, demandOption: true },
	timeout: { type: 'number', default: -1 }
}).parseSync()

console.log(`URL: ${argv.url} Files: ${argv.files}`)

const compareLength = 32

function main() {
    const vectors: {
        [key: string]: {
            expected: Uint8Array,
            got: Promise<Uint8Array>,
        }
    } = {}

    for (const f of argv.files) {
        const buffer = Buffer.alloc(compareLength)
        const fd = openSync(f, 'r')
        readSync(fd, buffer)

        vectors[basename(f)] = {
            expected: new Uint8Array(buffer),
            got: new Promise(r => {})
        }
    }

    new Receiver(
        new WebSocket(argv.url), {
            onRequest: (requests) => {
                console.log("Got requests", requests)
                return new Promise(r => r(true))
            },
            onReceive: (f, stream) => {
                vectors[f.Name].got = new Promise(r => {
                    stream.onFinish((buffer) => {
                        r(buffer.slice(0, compareLength))
                    })
                })
            },
            onTransfersComplete: async () => {
                for (const v in vectors) {
                    let same = true
                    const got = await vectors[v].got
                    for (let i = 0; i < compareLength; i++) {
                        if (vectors[v].expected[i] !== got[i]) {
                            same = false
                        }
                    }
                    if (!same) {
                        console.log(
                            "File", v,
                            "got", got,
                            "expected", vectors[v].expected
                        )
                    } else {
                        console.log("PASS", v)
                    }
                }
            }
        }
    )
}

main()