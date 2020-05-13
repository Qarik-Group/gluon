package v1alpha1

import (
	batchv1 "k8s.io/api/batch/v1"
)

const (
	StatePending   = "pending"
	StateResolving = "resolving"
	StateResolved  = "resolved"
	StateFailed    = "failed"
)

func DetermineReadiness(job *batchv1.Job) (bool, string) {
	if job == nil || job.Status.Succeeded+job.Status.Failed == 0 {
		return false, StatePending
	}

	if job.Status.Active != 0 {
		return false, StateResolving
	}

	if job.Status.Succeeded > 0 && job.Status.Failed == 0 {
		return true, StateResolved
	}

	return true, StateFailed
}
