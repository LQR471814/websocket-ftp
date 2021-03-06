export enum State {
    INITIAL,
    LISTENING_REQUESTS,
    WAITING_CONFIRMATION,
    RECEIVING,
}

export enum Event {
    PEER_CONNECT,
    RECEIVE_REQUESTS,
    USER_ACCEPT,
    USER_DENY,
    RECEIVE_CONTENTS,
    RECEIVE_DONE,
}
