import WebSocket, { MessageEvent } from "isomorphic-ws"
import { EventStateMatrix, Event, Action, State, BUFFER_SIZE } from "./state"
import { FileStream, isFileStream } from "../stream"

type Signals =
| { Type: "start" }
| { Type: "exit" }
| { Type: "complete" }

type Bytes = Uint8Array | ArrayBuffer

export type File = {
	Name: string
	Size: number
	Type: string

	data: FileStream | Bytes
}

export type Hooks = {
	onstart?: () => void
	onprogress?: (received: number, total: number) => void
	onsuccess?: () => void
	onclose?: () => void
}

export class Transfer {
	hooks: Hooks
	files: File[]
	conn: WebSocket

	state: State
	currentFile: number

	verbose: boolean

	constructor(
		url: string,
		files: File[],
		hooks: Hooks = {},
		verbose = false,
	) {
		this.files = files
		this.hooks = hooks
		this.verbose = verbose

		this.state = State.INITIAL
		this.currentFile = 0

		this.conn = new WebSocket(url)

		this.conn.onopen = () => this.eventReducer(Event.start)
		this.conn.onmessage = (ev) => this._connHandler(ev)
	}

	private _connHandler(message: MessageEvent) {
		const msg = JSON.parse(message.data.toString()) as Signals
		switch (msg.Type) {
			case "start":
				this.hooks?.onstart?.()
				this.eventReducer(Event.beginFileUpload)
				break
			case "exit":
				this.eventReducer(Event.exitFileUpload)
				break
			case "complete":
				this.eventReducer(Event.uploadComplete)
				break
		}
	}

	private _sendRequests = () =>
		this.conn.send(
			JSON.stringify(
				{
					Type: "files",
					Files: this.files
				},
				(k, v) => (k === "data")
					? undefined
					: v
			)
		)

	private _log(message: string, ...args: any[]) {
		if (this.verbose) {
			console.info(`[Client] ${message}`, ...args)
		}
	}

	async cancel() {
		this.actionReducer(Action.Quit)
	}

	async eventReducer(event: Event) {
		const cell = EventStateMatrix[event][this.state]
		this._log(`event ${event} - state ${this.state}`)

		if (!cell) {
			throw new Error(`Undefined cell ${event} ${this.state}`)
		}

		this.state = cell.newState
		for (const action of cell.actions) {
			this.actionReducer(action)
		}
	}

	async actionReducer(action: Action) {
		switch (action) {
			case Action.SendFileRequest:
				this._sendRequests()
				break
			case Action.UploadFile:
				const f = this.files[this.currentFile]
				const statusUpdateOffset = f.Size / 16

				let lastUpdatePosition = 0
				let uploaded = 0

				const isStream = isFileStream(f.data)
				while (uploaded < f.Size) {
					let buff: Bytes | undefined

					if (isStream) {
						const b = await (f.data as FileStream).read()
						buff = b
					} else {
						buff = (f.data as Bytes).slice(uploaded, uploaded+BUFFER_SIZE)
					}

					if (buff) {
						this.conn.send(buff)
						uploaded += buff.byteLength
						this._log(`sent ${uploaded} / ${f.Size}`)
					}

					if (uploaded - lastUpdatePosition > statusUpdateOffset) {
						if (this.hooks?.onprogress) {
							this.hooks.onprogress(
								uploaded > f.Size ?
									f.Size :
									uploaded,
								f.Size
							)
						}
						lastUpdatePosition = uploaded
					}
				}

				break
			case Action.IncrementFileIndex:
				this.currentFile++
				if (!this.files[this.currentFile]) {
					this.hooks?.onsuccess?.()
					this.eventReducer(Event.exitFileUpload)
					return
				}
				break
			case Action.Quit:
				this.conn?.close()
				this.hooks?.onclose?.()
				break
			default:
				throw new Error(`${action} does not exist!`)
		}
	}
}
