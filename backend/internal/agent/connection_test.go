package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type probeRuntime struct {
	status Status
	result PromptResult
	err    error
	prompt string
}

func (p *probeRuntime) Status() Status { return p.status }
func (p *probeRuntime) Prompt(_ context.Context, prompt string) (PromptResult, error) {
	p.prompt = prompt
	return p.result, p.err
}

func TestProbeConnectionMatrix(t *testing.T) {
	tests := []struct {
		name    string
		runtime *probeRuntime
		want    error
	}{
		{"unavailable", &probeRuntime{}, ErrRuntimeUnavailable},
		{"unconfigured", &probeRuntime{status: Status{Available: true}}, ErrRuntimeUnconfigured},
		{"prompt failure", &probeRuntime{status: Status{Available: true, Configured: true}, err: errors.New("provider failed")}, errors.New("provider failed")},
		{"marker mismatch", &probeRuntime{status: Status{Available: true, Configured: true}, result: PromptResult{Content: "unexpected"}}, ErrProbeMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ProbeConnection(context.Background(), test.runtime)
			if err == nil || err.Error() != test.want.Error() {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
		})
	}
}

func TestProbeConnectionAcceptsMarkerAndBoundsResponse(t *testing.T) {
	runtime := &probeRuntime{status: Status{Available: true, Configured: true}, result: PromptResult{Content: ConnectionProbeMarker + strings.Repeat("界", 300)}}
	result, err := ProbeConnection(context.Background(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runtime.prompt, ConnectionProbeMarker) {
		t.Fatal("probe marker missing from prompt")
	}
	if len([]rune(result.Response)) != 201 || !strings.HasSuffix(result.Response, "…") {
		t.Fatalf("unbounded response: %d runes", len([]rune(result.Response)))
	}
}
