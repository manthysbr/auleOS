package duckdb

import (
	"context"
	"testing"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Jobs(t *testing.T) {
	// Use in-memory DB for testing
	repo, err := NewRepository("?cache=shared&mode=memory") 
    // Usually :memory: or strict memory mode works. 
    // DuckDB Go driver supports "".
	if err != nil {
		// Fallback to temp file if memory string is tricky with driver versions
		repo, err = NewRepository(t.TempDir() + "/test.db")
	}
	require.NoError(t, err)

	ctx := context.Background()

	// 1. Save Job
	jobID := domain.JobID("job-1")
	job := domain.Job{
		ID:        jobID,
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Spec: domain.WorkerSpec{
			Image: "alpine",
		},
		Metadata: map[string]string{"foo": "bar"},
	}

	err = repo.SaveJob(ctx, job)
	require.NoError(t, err)

	// 2. Get Job
	fetched, err := repo.GetJob(ctx, jobID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, fetched.ID)
	assert.Equal(t, job.Status, fetched.Status)
	assert.Equal(t, "alpine", fetched.Spec.Image)
    // Check metadata
    assert.Equal(t, "bar", fetched.Metadata["foo"])

	// 3. Update Job
	job.Status = domain.JobStatusRunning
	err = repo.SaveJob(ctx, job)
	require.NoError(t, err)

	fetched2, err := repo.GetJob(ctx, jobID)
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusRunning, fetched2.Status)

	// 4. List Jobs
	jobs, err := repo.ListJobs(ctx)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, jobID, jobs[0].ID)
}

func TestRepository_Workers(t *testing.T) {
    // New DB for isolation
	repo, err := NewRepository(t.TempDir() + "/workers.db")
	require.NoError(t, err)
    ctx := context.Background()

    // 1. Save Worker
    id := domain.WorkerID("w-1")
    worker := domain.Worker{
        ID: id,
        Status: domain.HealthStatusUnknown,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        Spec: domain.WorkerSpec{Image: "nginx"},
    }
    
    err = repo.SaveWorker(ctx, worker)
    require.NoError(t, err)

    // 2. Get Worker
    got, err := repo.GetWorker(ctx, id)
    require.NoError(t, err)
    assert.Equal(t, id, got.ID)

    // 3. Update Status
    err = repo.UpdateWorkerStatus(ctx, id, domain.HealthStatusHealthy)
    require.NoError(t, err)

    got2, err := repo.GetWorker(ctx, id)
    require.NoError(t, err)
    assert.Equal(t, domain.HealthStatusHealthy, got2.Status)
}
