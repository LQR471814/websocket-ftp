import WebSocket, { MessageEvent } from "isomorphic-ws"
import { EventStateMatrix, Event, Action, State, BUFFER_SIZE } from "./state"

type Signals =
| { Type: "start" }
| { Type: "exit" }
| { Type: "complete" }

export type File = {
	Name: string
	Size: number
	Type: string
	data: Uint8Array | ArrayBuffer
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

	cancel() {
		this.actionReducer(Action.Quit)
	}

	eventReducer(event: Event) {
		const cell = EventStateMatrix[event][this.state]
		if (this.verbose) {
			console.log(`Event ${event} - State ${this.state}`)
		}

		if (!cell) {
			throw new Error(`Undefined cell ${event} ${this.state}`)
		}

		this.state = cell.newState

		for (const action of cell.actions) {
			this.actionReducer(action)
		}
	}

	actionReducer(action: Action) {
		switch (action) {
			case Action.SendFileRequest:
				this._sendRequests()
				break
			case Action.UploadFile:
				const f = this.files[this.currentFile]
				const statusUpdateOffset = f.Size / 16

				let bytesRemaining = f.Size
				let byteStart = 0
				let updatePosition = statusUpdateOffset

				while (bytesRemaining > 0) {
					this.conn.send(
						f.data.slice(
							byteStart,
							byteStart + BUFFER_SIZE
						)
					)

					bytesRemaining -= BUFFER_SIZE
					byteStart += BUFFER_SIZE

					// Only update progress if current sent bytes
					//  is larger than update frequency
					if (this.hooks?.onprogress && byteStart > updatePosition) {
						this.hooks.onprogress(
							byteStart < f.Size
								? byteStart
								: f.Size,
							f.Size
						)
						updatePosition += statusUpdateOffset
					}
				}

				break
			case Action.IncrementFileIndex:
				this.currentFile++
				if (!this.files[this.currentFile]) {
					this.eventReducer(Event.exitFileUpload)
					this.hooks?.onsuccess?.()
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
