import WebSocket, { OPEN } from "isomorphic-ws"
import { Event, State } from "./state"
import { WritableStream } from "../stream"

export type File = {
    Name: string
    Size: number
    Type: string
}

type Signals = {
    Type: "files",
    Files: File[]
}

type EventObjects =
    | { event: Event.PEER_CONNECT }
    | { event: Event.RECEIVE_REQUESTS, files: File[] }
    | { event: Event.RECEIVE_CONTENTS, contents: Promise<Uint8Array>, length: number }
    | { event: Event.RECEIVE_DONE }

function listenEvents(
    conn: WebSocket,
    callback: (e: EventObjects) => void
) {
    if (conn.readyState === OPEN) {
        callback({ event: Event.PEER_CONNECT })
    } else {
        conn.onopen = () => {
            callback({ event: Event.PEER_CONNECT })
        }
    }

    conn.onmessage = (message) => {
        const data = message.data as any
        switch(data.constructor.name) {
            case "String":
                const control = JSON.parse(data as string) as Signals
                switch (control.Type) {
                    case "files":
                        callback({
                            event: Event.RECEIVE_REQUESTS,
                            files: control.Files,
                        })
                }
                break
            case "Blob":
                const blob = data as Blob
                callback({
                    event: Event.RECEIVE_CONTENTS,
                    contents: new Promise(r => {
                        blob.arrayBuffer().then((value) => {
                            r(new Uint8Array(value))
                        })
                    }),
                    length: blob.size,
                })
                break
            default:
                const array = new Uint8Array(data as ArrayBuffer | Buffer)
                callback({
                    event: Event.RECEIVE_CONTENTS,
                    contents: new Promise(r => r(array)),
                    length: array.byteLength
                })
        }
    }
}

export type Hooks = {
    onRequest: (files: File[]) => Promise<boolean>
    onReceive?: (file: File, stream: WritableStream) => void
    onTransfersComplete?: () => void
}

export class Receiver {
    state: State = State.INITIAL
    requests?: File[]
    outputs: WritableStream[] = []
    currentFile?: number = 0
    hooks: Hooks
    conn: WebSocket

    verbose: boolean

    constructor(conn: WebSocket, hooks: Hooks, verbose: boolean = false) {
        this.conn = conn
        this.hooks = hooks
        this.verbose = verbose
        listenEvents(conn, (e) => {
            switch (e.event) {
                case Event.PEER_CONNECT:
                    if (this.state !== State.INITIAL) {
                        throw new Error(
                            `Invalid state ${this.state} to receive PEER_CONNECT`
                        )
                    }
                    this._log("peer connected")
                    this.state = State.LISTENING_REQUESTS
                    break
                case Event.RECEIVE_REQUESTS:
                    if (this.state !== State.LISTENING_REQUESTS) {
                        throw new Error(
                            `Invalid state ${this.state} to receive RECEIVE_REQUESTS`
                        )
                    }
                    this.state = State.WAITING_CONFIRMATION
                    this.requests = e.files
                    this._log("got requests", this.requests)
                    this.hooks.onRequest(this.requests).then(
                        (choice) => {
                            this._log("got choice", choice)
                            if (this.state !== State.WAITING_CONFIRMATION) {
                                throw new Error(
                                    `Invalid state ${this.state} to receive USER_CHOICE`
                                )
                            }

                            if (choice) {
                                this.currentFile = 0
                                const file = this.requests![this.currentFile]
                                this.requests?.map((f, i) => {
                                    this.outputs[i] = new WritableStream(f.Size)
                                    this.outputs[i].onFinish(() => {
                                        this._log(
                                            "completed transfer",
                                            this.currentFile
                                        )
                                        this.currentFile!++
                                        this._send({ Type: "complete" })
                                        if (this.currentFile! < this.requests!.length) {
                                            this._log(
                                                "beginning receive for",
                                                this.outputs[this.currentFile!]
                                            )
                                            this.hooks.onReceive?.(
                                                this.requests![this.currentFile!],
                                                this.outputs[this.currentFile!]
                                            )
                                            this._send({ Type: "start" })
                                        } else {
                                            this.state = State.INITIAL
                                            this.hooks.onTransfersComplete?.()
                                            this.conn.close()
                                        }
                                    })
                                })
                                this._log(
                                    "beginning receive for",
                                    this.outputs[this.currentFile]
                                )
                                this.hooks.onReceive?.(
                                    file, this.outputs[this.currentFile]
                                )
                                this._send({ Type: "start" })
                                this.state = State.RECEIVING
                                return
                            }
                            this.state = State.INITIAL
                            this._send({ Type: "exit" })
                        }
                    )
                    break
                case Event.RECEIVE_CONTENTS:
                    if (this.state !== State.RECEIVING) {
                        throw new Error(
                            `Invalid state ${this.state} to receive RECEIVE_CONTENTS`
                        )
                    }
                    //? explicit offset specification to avoid edge case
                    //? where promise of Uint8Array resolves after another
                    //? packet comes in
                    this.outputs[this.currentFile!].writeAsync(e.contents, e.length)
                    this._log("Receive contents", e.length)
                    break
            }
        })
    }

    private _send(signal: Object) {
        this.conn.send(JSON.stringify(signal))
    }

    private _log(message: string, ...args: any[]) {
        if (this.verbose) {
            console.info(`[Receiving] ${message}`, ...args)
        }
    }
}
