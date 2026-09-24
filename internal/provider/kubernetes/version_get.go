package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*versionGetTask)(nil)

type versionGetTask struct{ provider *Provider }

func (t *versionGetTask) Name() string { return "kubernetes.version.get" }

func (t *versionGetTask) JSONSchema() string { return versionGetSchema }

func (t *versionGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	ver, err := client.clientset.Discovery().ServerVersion()
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"version": versionInfo{
		Major:        ver.Major,
		Minor:        ver.Minor,
		GitVersion:   ver.GitVersion,
		GitCommit:    ver.GitCommit,
		GitTreeState: ver.GitTreeState,
		BuildDate:    ver.BuildDate,
		GoVersion:    ver.GoVersion,
		Compiler:     ver.Compiler,
		Platform:     ver.Platform,
	}}), nil
}

const versionGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Version Get Parameters",
  "description": "Retrieves Kubernetes server version. No parameters required.",
  "properties": {},
  "additionalProperties": false
}`

type versionInfo struct {
	Major        string `json:"major,omitempty"`
	Minor        string `json:"minor,omitempty"`
	GitVersion   string `json:"git_version,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
	GitTreeState string `json:"git_tree_state,omitempty"`
	BuildDate    string `json:"build_date,omitempty"`
	GoVersion    string `json:"go_version,omitempty"`
	Compiler     string `json:"compiler,omitempty"`
	Platform     string `json:"platform,omitempty"`
}
