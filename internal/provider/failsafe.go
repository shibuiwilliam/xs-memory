package provider

import (
	"context"
	"fmt"
)

// FailsafeLLM is a provider double that fails the test if invoked.
// Used to verify host-delegated mode never calls a server-side model (addendum H1).
type FailsafeLLM struct {
	// OnCall is invoked if Complete is called. In tests, set this to t.Fatal.
	OnCall func(msg string)
}

func NewFailsafeLLM(onCall func(string)) *FailsafeLLM {
	return &FailsafeLLM{OnCall: onCall}
}

func (f *FailsafeLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	msg := "FailsafeLLM: server-side provider was called — this violates the host-delegation contract (addendum H1)"
	if f.OnCall != nil {
		f.OnCall(msg)
	}
	return CompletionResponse{}, fmt.Errorf("%s", msg)
}
