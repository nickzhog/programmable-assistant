package fsm

type State struct {
	Step      string            `json:"step"`
	Data      map[string]string `json:"data"`
	HandlerNS string            `json:"handler_ns"`
}
