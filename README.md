## websocket-ftp

A simple FTP protocol with client and server implemented in TypeScript and Golang.

### Example (Client)

```typescript
const buffer: Uint8Array = (new TextEncode()).encode("Testing Text")

new Transfer(
    "ws://example.server.com/filetransfer",
    [
        {
            Name: "Test File",
            Size: buffer.length,
            Type: "application/octet-stream",
            data: buffer
        },
    ],
    {
        onstart: () => {
            console.log("Transfer has started.")
        },
        onprogress: (r, t) => console.log(`Sent ${r} / ${t} bytes`),
        onsuccess: () =>
            console.log("Success!"),
        onclose: () =>
            console.log("Closed WS connection"),
    },
)
```

### Example (Server)

```golang
type Hooks struct{}

func (h Hooks) OnTransferRequest(t *Transfer) chan bool {
    log.Println("Got requests", t.Data.Files)
    c := make(chan bool, 1)
    c <- true
    return c
}

func (h Hooks) OnTransferUpdate(t *Transfer) {
    log.Println(
        "Progress",
        t.State.Received,
        "/",
        t.Data.Files[t.State.CurrentFile].Size,
    )
}

func (h Hooks) OnTransferComplete(t *Transfer, f File) {
    log.Println(f.Name, "has been received")
}

func (h Hooks) OnAllTransfersComplete(t *Transfer) {
    log.Println("Transfer", t.ID, "has completed")
}

server := NewServer(ServerConfig{
    Handlers: Hooks{},
})

server.Serve()
```
