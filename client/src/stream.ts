import { ReadStream } from "fs"

interface BrowserFile {
    stream(): ReadableStream<Uint8Array>
}

export interface FileStream {
    closed: boolean
    read(): Promise<Uint8Array | undefined>
}

export function isFileStream(object: any): boolean {
    return "read" in object
}

export class NodeFileStream implements FileStream {
    closed: boolean = false
    private stream: ReadStream
    private bufferSize: number

    constructor(stream: ReadStream, bufferSize: number = 1024*1024) {
        this.bufferSize = bufferSize
        this.stream = stream
        this.stream.on("end", () => this.closed = true)
    }

    read(): Promise<Uint8Array | undefined> {
        return new Promise(r => {
            const handler = () => {
                const data = this.stream.read(this.bufferSize)
                if (data) {
                    r(data)
                    this.stream.removeListener("readable", handler)
                }
            }
            this.stream.on("readable", handler)
        })
    }
}

export class BrowserFileStream implements FileStream {
    closed: boolean
    private stream: ReadableStream<Uint8Array>

    constructor(file: BrowserFile) {
        this.closed = false
        this.stream = file.stream()
        this.stream.getReader().closed.then(() => { closed = true })
    }

    async read(): Promise<Uint8Array | undefined> {
        const result = await this.stream.getReader().read()
        if (result.done) {
            closed = true
        }
        return result.value
    }
}

export class WritableStream {
    buffer: Uint8Array
    total: number
    private written: number = 0
    private onFinishListeners: ((buffer: Uint8Array) => void)[] = []

    constructor(size: number) {
        this.total = size
        this.buffer = new Uint8Array(new ArrayBuffer(size))
    }

    write(bytes: Uint8Array): boolean {
        if (this.written + bytes.length > this.total) {
            return false
        }

        this.buffer.set(bytes, this.written)
        this.written += bytes.byteLength

        if (this.written >= this.total) {
            for (const callback of this.onFinishListeners) {
                callback(this.buffer)
            }
            return false
        }

        return true
    }

    onFinish(callback: (buffer: Uint8Array) => void) {
        this.onFinishListeners.push(callback)
    }
}
