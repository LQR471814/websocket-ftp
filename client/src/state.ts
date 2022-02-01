export const BUFFER_SIZE = 1024

export enum Event {
	start,
	beginFileUpload,
	exitFileUpload,
	uploadComplete,
}

export enum State {
	INITIAL,
	WAITING_TO_START,
	WAITING_FOR_COMPLETION,
}

export enum Action {
	SendFileRequest,
	UploadFile,
	IncrementFileIndex,
	Quit,
}

type StateMatrix = {
	[key in Event]: {
		[key in State]?: {
			actions: Action[]
			newState: State
		}
	}
}

export const EventStateMatrix: StateMatrix = {
	[Event.start]: { [State.INITIAL]: {
		actions: [Action.SendFileRequest],
		newState: State.WAITING_TO_START,
	} },
	[Event.beginFileUpload]: { [State.WAITING_TO_START]: {
		actions: [Action.UploadFile],
		newState: State.WAITING_FOR_COMPLETION,
	} },
	[Event.uploadComplete]: { [State.WAITING_FOR_COMPLETION]: {
		actions: [Action.IncrementFileIndex],
		newState: State.WAITING_TO_START,
	} },
	[Event.exitFileUpload]: {
		[State.WAITING_TO_START]: {
			actions: [Action.Quit],
			newState: State.INITIAL,
		},
		[State.WAITING_FOR_COMPLETION]: {
			actions: [Action.Quit],
			newState: State.INITIAL,
		}
	},
}
