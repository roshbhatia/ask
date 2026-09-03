package schema

#Action: {
	description: string & !=""
	argv?: [...string]
	env?: [=~"^[A-Za-z_][A-Za-z0-9_]*$"]: string
}

#Provider: {
	version: "provider/v1"
	name: string & =~"^[a-z][a-z0-9._-]*$"
	description: string & !=""
	command: [string & !="", ...string]
	actions: [=~"^[a-z][a-z0-9._-]*$"]: #Action
	requires?: {
		commands?: [...string & !=""]
		environment?: [...string & =~"^[A-Za-z_][A-Za-z0-9_]*$"]
		paths?: [...string & !=""]
	}
	defaults?: {
		timeout?: string & =~"^[0-9]+(ns|us|µs|ms|s|m|h)$"
		priority?: int
	}
}
