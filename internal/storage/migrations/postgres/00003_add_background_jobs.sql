-- +goose Up
-- Background job definitions
CREATE TABLE background_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    agent_provider TEXT NOT NULL DEFAULT 'claude-code',
    agent_config TEXT NOT NULL DEFAULT '{}',  -- JSON configuration
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, running, paused, completed, failed, cancelled
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    last_run_at TIMESTAMP,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX idx_background_jobs_plan_id ON background_jobs(plan_id);
CREATE INDEX idx_background_jobs_status ON background_jobs(status);

-- Job execution history
CREATE TABLE job_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,  -- References background_jobs.id
    execution_number BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed, cancelled
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    triggered_by TEXT NOT NULL,  -- manual, scheduled, api
    UNIQUE(job_id, execution_number),
    FOREIGN KEY (job_id) REFERENCES background_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_job_executions_job_id ON job_executions(job_id);
CREATE INDEX idx_job_executions_started_at ON job_executions(started_at DESC);

-- One-time scheduled jobs
CREATE TABLE scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL UNIQUE,  -- One schedule per job
    scheduled_at TIMESTAMP NOT NULL,  -- When to execute (one-time)
    cancelled BOOLEAN NOT NULL DEFAULT false,  -- Allow cancellation before execution
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (job_id) REFERENCES background_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_scheduled_jobs_scheduled_at ON scheduled_jobs(scheduled_at);
CREATE INDEX idx_scheduled_jobs_cancelled ON scheduled_jobs(cancelled);

-- +goose Down
DROP TABLE IF EXISTS scheduled_jobs;
DROP TABLE IF EXISTS job_executions;
DROP TABLE IF EXISTS background_jobs;
