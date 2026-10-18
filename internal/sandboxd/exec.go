//go:build linux

package sandboxd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandbox"
)

// output is a command's collected output and how it ended.
type output struct {
	stdout, stderr bytes.Buffer
	status         sandbox.ExitStatus
}

// startExec sends an exec request and returns the connection its frames come on.
func (m *vm) startExec(ctx context.Context, req sandbox.AgentRequest) (net.Conn, error) {
	req.Op = sandbox.OpExec
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(req)
	if _, err := conn.Write(append(encoded, '\n')); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// exec runs a command to its end and collects its output. The context bounds the wait:
// when it ends, the command is killed.
func (m *vm) exec(ctx context.Context, req sandbox.AgentRequest, stdin []byte) (*output, error) {
	req.Stdin = len(stdin) > 0
	conn, err := m.startExec(ctx, req)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if req.Stdin {
		sandbox.WriteFrame(conn, sandbox.FrameStdin, stdin)
		sandbox.WriteFrame(conn, sandbox.FrameCloseStdin, nil)
	}
	stop := context.AfterFunc(ctx, func() {
		sandbox.WriteFrame(conn, sandbox.FrameKill, nil)
		// The agent's answer, or nothing if the VM is gone: do not wait long for either.
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	})
	defer stop()
	result := &output{}
	for {
		kind, payload, err := sandbox.ReadFrame(conn)
		if err != nil {
			if ctx.Err() != nil {
				result.status.TimedOut = true
				return result, nil
			}
			return nil, err
		}
		switch kind {
		case sandbox.FrameStdout:
			result.stdout.Write(payload)
		case sandbox.FrameStderr:
			result.stderr.Write(payload)
		case sandbox.FrameExit:
			if err := json.Unmarshal(payload, &result.status); err != nil {
				return nil, err
			}
			if ctx.Err() != nil {
				result.status.TimedOut = true
			}
			return result, nil
		case sandbox.FrameError:
			return nil, errors.New(string(payload))
		}
	}
}

// toResult turns collected output into what the API returns.
func (o *output) toResult(cpuMillis int) sandbox.Result {
	result := sandbox.Result{
		Stdout: o.stdout.String(), Stderr: o.stderr.String(), ExitCode: o.status.Code, Signal: o.status.Signal,
		DurationMillis: o.status.DurationMillis, CPUMillis: cpuMillis, LimitsHit: []string{},
		StdoutTruncated: o.status.StdoutTruncated, StderrTruncated: o.status.StderrTruncated,
	}
	if o.status.TimedOut {
		result.LimitsHit = append(result.LimitsHit, sandbox.LimitTime)
	}
	if o.status.OutOfMemory {
		result.LimitsHit = append(result.LimitsHit, sandbox.LimitMemory)
	}
	if o.status.StdoutTruncated || o.status.StderrTruncated {
		result.LimitsHit = append(result.LimitsHit, sandbox.LimitOutput)
	}
	return result
}
