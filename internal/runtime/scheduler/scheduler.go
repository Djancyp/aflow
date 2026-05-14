// Package scheduler implements trigger scheduling for workflow activation.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/djan/aflow/internal/queue/jobs"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// Scheduler handles trigger activation on workflow publish.
// Implements service.TriggerScheduler.
type Scheduler struct {
	riverClient *river.Client[pgx.Tx]
}

func New(riverClient *river.Client[pgx.Tx]) *Scheduler {
	return &Scheduler{riverClient: riverClient}
}

// ScheduleCron inserts a River cron.trigger job for the next scheduled run.
// Uses River's unique-by-args option to prevent duplicate cron chains per workflow.
func (s *Scheduler) ScheduleCron(ctx context.Context, workspaceID, workflowID, schedule string) error {
	next, err := nextRunTime(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}

	_, err = s.riverClient.Insert(ctx, jobs.CronTriggerArgs{
		WorkflowID:  workflowID,
		WorkspaceID: workspaceID,
		Schedule:    schedule,
	}, &river.InsertOpts{
		ScheduledAt: next,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateScheduled,
				rivertype.JobStateAvailable,
				rivertype.JobStateRunning,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("insert cron job: %w", err)
	}
	return nil
}

func nextRunTime(schedule string) (time.Time, error) {
	s, err := cronParser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return s.Next(time.Now()), nil
}
