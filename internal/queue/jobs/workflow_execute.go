package jobs

// WorkflowExecuteArgs is the River job payload for workflow execution.
type WorkflowExecuteArgs struct {
	ExecutionID string `json:"execution_id"`
}

func (WorkflowExecuteArgs) Kind() string { return "workflow.execute" }
