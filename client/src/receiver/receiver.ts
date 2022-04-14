import WebSocket, { OPEN } from "isomorphic-ws"
import { Event, State } from "./state"
import { WritableStream } from "../stream"

type File = {
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
    | { event: Event.RECEIVE_CONTENTS, contents: Uint8Array }
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
        if (typeof message.data === "string") {
            const control = JSON.parse(message.data) as Signals
            switch (control.Type) {
                case "files":
                    callback({
                        event: Event.RECEIVE_REQUESTS,
                        files: control.Files,
                    })
            }
        } else {
            callback({
                event: Event.RECEIVE_CONTENTS,
                contents: new Uint8Array(message.data as ArrayBuffer | Buffer)
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

    constructor(conn: WebSocket, hooks: Hooks) {
        this.conn = conn
        this.hooks = hooks
        listenEvents(conn, (e) => {
            switch (e.event) {
                case Event.PEER_CONNECT:
                    if (this.state !== State.INITIAL) {
                        throw new Error(
                            `Invalid state ${this.state} to receive PEER_CONNECT`
                        )
                    }
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
                    this.hooks.onRequest(this.requests).then(
                        (choice) => {
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
                                        console.log("completed transfer", this.currentFile)
                                        this.currentFile!++
                                        this.send({ Type: "complete" })
                                        if (this.currentFile! < this.requests!.length) {
                                            this.hooks.onReceive?.(
                                                this.requests![this.currentFile!],
                                                this.outputs[this.currentFile!]
                                            )
                                            this.send({ Type: "start" })
                                        } else {
                                            this.state = State.INITIAL
                                            this.hooks.onTransfersComplete?.()
                                            this.conn.close()
                                        }
                                    })
                                })
                                this.hooks.onReceive?.(
                                    file, this.outputs[this.currentFile]
                                )
                                this.send({ Type: "start" })
                                this.state = State.RECEIVING
                                return
                            }
                            this.state = State.INITIAL
                            this.send({ Type: "exit" })
                        }
                    )
                    break
                case Event.RECEIVE_CONTENTS:
                    if (this.state !== State.RECEIVING) {
                        throw new Error(
                            `Invalid state ${this.state} to receive RECEIVE_CONTENTS`
                        )
                    }
                    this.outputs[this.currentFile!].write(e.contents)
                    break
            }
        })
    }

    send(signal: Object) {
        this.conn.send(JSON.stringify(signal))
    }
}
