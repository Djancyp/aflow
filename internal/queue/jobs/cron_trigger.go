package jobs

// CronTriggerArgs is the River job payload for a scheduled cron trigger.
type CronTriggerArgs struct {
	WorkflowID  string `json:"workflow_id"`
	WorkspaceID string `json:"workspace_id"`
	Schedule    string `json:"schedule"`
}

func (CronTriggerArgs) Kind() string { return "cron.trigger" }
