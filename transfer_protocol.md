## Control Protocol Specification

`Text` WS messages will always be handled as control messages

`Binary` WS messages will always be handled as file chunks (these should be set to 1 mB)

Message Types described with `typescript`

```typescript
type ClientBound =
  | { Type: "start" }
  | { Type: "exit" }
  | { Type: "complete" }

type ServerBound =
  | { Type: "files"
      Files: {
        Name: string
        Size: number
        Type: string //? Mimetype
      }[] }
```

## Client State

| Events           | 0 - Initial | 1 - Connecting | 2 - Waiting to Start | 3 - Waiting for Upload Complete |
|------------------|:-----------:|:--------------:|:--------------------:|:-------------------------------:|
| start            |    opw/1    |        -       |           -          |                -                |
| open             |      -      |      srq/2     |           -          |                -                |
| startfileupload  |      -      |        -       |         upl/3        |                -                |
| exitfileupload   |      -      |        -       |         qui/0        |              qui/0              |
| uploadcomplete   |      -      |        -       |           -          |              inc/2              |

```text
opw : open websocket connection
srq : send files request
upl : upload file at current index
inc : increment current file index (emits event exitfileupload if out of range and start file upload otherwise)
qui : close connection and quit
```

## Server State

| Events           | 0 - Initial | 1 - Listening for File Requests | 2 - Waiting for User Confirmation |       3 - Receiving       |
|------------------|:-----------:|:-------------------------------:|:---------------------------------:|:-------------------------:|
| peerconnect      |      1      |                -                |                 -                 |             -             |
| recvrequests     |      -      |              dsr/2              |                 -                 |             -             |
| useraccept       |      -      |                -                |             scs, bfw/3            |             -             |
| userdeny         |      -      |                -                |               scx/0               |             -             |
| recvfilecontents |      -      |                -                |                 -                 |            sgd            |
| recvdone         |      -      |                -                |                 -                 |     sbw, sfu, inc, rdh    |

```text
dsr : display file send requests to user
inc : increment current file index

scs : send start signal
scx : send exit signal
sfu : send upload finished signal

bfw : start file writer goroutine
sgd : write chunk with file writer goroutine
sbw : stop file writer goroutine (wait for routine to finish)

rdh : Handler for recvdone, translates to something like this
if !(index >= len(files)) bfw, scs
```

## Server File Writer Goroutine State

| Events         | 0 - Initial | 1 - Waiting for file data |
|----------------|-------------|---------------------------|
| oninit         | crf, ibf/1  | -                         |
| onrecvfiledata | -           | wrb/1                     |
| onfileallrecv  | -           | cll, clo/0                |

```text
crf : create file
sfc : send client file upload complete
cll : call onfinish callback with filename

ibf : initialize buffer
wrb : write to buffer
clo : flush buffer and close file
```
