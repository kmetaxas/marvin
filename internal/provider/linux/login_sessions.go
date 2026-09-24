package linux

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// loginSessionsTask implements the linux.login.sessions capability.
type loginSessionsTask struct{ provider *Provider }

const loginSessionsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.login.sessions Parameters",
  "description": "Parameters for linux.login.sessions."
}`

func (t *loginSessionsTask) Name() string       { return "linux.login.sessions" }
func (t *loginSessionsTask) JSONSchema() string { return loginSessionsSchema }

func (t *loginSessionsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("login.sessions starting", "capability", t.Name())

	entries, err := readLoginSessions()
	if err != nil {
		slog.Info("login.sessions failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("login.sessions succeeded", "capability", t.Name(), "count", len(entries))
	return common.SuccessResult(map[string]any{"sessions": entries}), nil
}

var _ task.Task = (*loginSessionsTask)(nil)

// utmp types
const (
	UT_LINESIZE = 32
	UT_NAMESIZE = 32
	UT_HOSTSIZE = 256
)

type UtmpEntry struct {
	Type int16
	_    int16 // pid alignment padding
	Pid  int32
	Line [UT_LINESIZE]byte
	ID   [4]byte
	User [UT_NAMESIZE]byte
	Host [UT_HOSTSIZE]byte
	Exit struct {
		Termination int16
		Exit        int16
	}
	Session int32
	Tv      struct {
		Sec  int32
		Usec int32
	}
	AddrV6 [16]byte
	_      [20]byte // reserved
}

type LoginSession struct {
	User     string `json:"user,omitempty"`
	TTY      string `json:"tty,omitempty"`
	Host     string `json:"host,omitempty"`
	Time     string `json:"time,omitempty"`
	PID      int32  `json:"pid,omitempty"`
	TypeName string `json:"type"`
}

func readLoginSessions() ([]LoginSession, error) {
	data, err := os.ReadFile("/var/run/utmp")
	if err != nil {
		// Fallback to /run/utmp
		data, err = os.ReadFile("/run/utmp")
		if err != nil {
			return nil, fmt.Errorf("read utmp: %w", err)
		}
	}

	var sessions []LoginSession
	recordSize := binary.Size(UtmpEntry{})
	for i := 0; i+recordSize <= len(data); i += recordSize {
		var ut UtmpEntry
		buf := data[i : i+recordSize]
		if err := binary.Read(newSliceReader(buf), binary.LittleEndian, &ut); err != nil {
			continue
		}
		// Types: 7 = USER_PROCESS (login session)
		if ut.Type != 7 {
			continue
		}
		sessions = append(sessions, LoginSession{
			User:     stringFromBytes(ut.User[:]),
			TTY:      stringFromBytes(ut.Line[:]),
			Host:     stringFromBytes(ut.Host[:]),
			Time:     time.Unix(int64(ut.Tv.Sec), 0).UTC().Format(time.RFC3339),
			PID:      ut.Pid,
			TypeName: "user_process",
		})
	}
	return sessions, nil
}

func stringFromBytes(b []byte) string {
	i := 0
	for i < len(b) && b[i] != 0 {
		i++
	}
	return string(b[:i])
}

type sliceReader struct {
	data []byte
	pos  int
}

func newSliceReader(data []byte) *sliceReader {
	return &sliceReader{data: data}
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
