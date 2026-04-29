-- +goose Up
CREATE TABLE IF NOT EXISTS background_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    agent_provider TEXT NOT NULL DEFAULT 'claude-code',
    agent_config TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_run_at TIMESTAMPTZ,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_background_jobs_plan_id ON background_jobs(plan_id);
CREATE INDEX IF NOT EXISTS idx_background_jobs_status ON background_jobs(status);
CREATE INDEX IF NOT EXISTS idx_background_jobs_created_at ON background_jobs(created_at DESC);

CREATE TABLE IF NOT EXISTS job_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    execution_number BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    triggered_by TEXT NOT NULL,
    UNIQUE(job_id, execution_number),
    FOREIGN KEY (job_id) REFERENCES background_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_job_executions_job_id ON job_executions(job_id);
CREATE INDEX IF NOT EXISTS idx_job_executions_started_at ON job_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_executions_status ON job_executions(status);

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL UNIQUE,
    scheduled_at TIMESTAMPTZ NOT NULL,
    cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (job_id) REFERENCES background_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_scheduled_at ON scheduled_jobs(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_cancelled ON scheduled_jobs(cancelled);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_job_id ON scheduled_jobs(job_id);

-- +goose Down
DROP TABLE IF EXISTS scheduled_jobs;
DROP TABLE IF EXISTS job_executions;
DROP TABLE IF EXISTS background_jobs;
