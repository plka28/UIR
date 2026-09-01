package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"waste-routing/internal/models"
)

const calculationRunColumns = `
    run_id,
    description,
    status,
    parameters,
    created_at,
    started_at,
    finished_at,
    objective_value,
    solver_exit_flag,
    solve_time_seconds,
    solver_metadata,
    error_message`

const createCalculationRunSQL = `
INSERT INTO waste_routing.calculation_runs (
    description,
    status,
    parameters,
    created_at,
    started_at,
    finished_at,
    objective_value,
    solver_exit_flag,
    solve_time_seconds,
    solver_metadata,
    error_message
) VALUES (
    $1,
    $2,
    $3::jsonb,
    COALESCE($4::timestamptz, now()),
    $5,
    $6,
    $7,
    $8,
    $9,
    $10::jsonb,
    $11
)
RETURNING` + calculationRunColumns

const selectCalculationRunSQL = `
SELECT` + calculationRunColumns + `
FROM waste_routing.calculation_runs`

const updateCalculationRunSQL = `
UPDATE waste_routing.calculation_runs
SET
    description = $2,
    status = $3,
    parameters = $4::jsonb,
    started_at = $5,
    finished_at = $6,
    objective_value = $7,
    solver_exit_flag = $8,
    solve_time_seconds = $9,
    solver_metadata = $10::jsonb,
    error_message = $11
WHERE run_id = $1`

// CreateCalculationRun inserts one optimization run and returns the values
// generated or normalized by PostgreSQL.
func (repository *Repository) CreateCalculationRun(
	ctx context.Context,
	run models.CalculationRun,
) (models.CalculationRun, error) {
	arguments, err := calculationRunArguments(run, true)
	if err != nil {
		return models.CalculationRun{}, fmt.Errorf("create calculation run: %w", err)
	}

	created, err := scanCalculationRun(repository.pool.QueryRow(ctx, createCalculationRunSQL, arguments...))
	if err != nil {
		return models.CalculationRun{}, fmt.Errorf("create calculation run: %w", err)
	}
	return created, nil
}

// GetCalculationRun returns an optimization run by its generated ID.
func (repository *Repository) GetCalculationRun(ctx context.Context, runID int64) (models.CalculationRun, error) {
	run, err := scanCalculationRun(repository.pool.QueryRow(
		ctx,
		selectCalculationRunSQL+` WHERE run_id = $1`,
		runID,
	))
	if err != nil {
		return models.CalculationRun{}, mapRowError(fmt.Sprintf("get calculation run %d", runID), err)
	}
	return run, nil
}

// UpdateCalculationRun updates mutable lifecycle and solver fields.
func (repository *Repository) UpdateCalculationRun(ctx context.Context, run models.CalculationRun) error {
	if run.RunID <= 0 {
		return fmt.Errorf("update calculation run: run ID must be positive")
	}
	arguments, err := calculationRunArguments(run, false)
	if err != nil {
		return fmt.Errorf("update calculation run %d: %w", run.RunID, err)
	}

	result, err := repository.pool.Exec(ctx, updateCalculationRunSQL, arguments...)
	if err != nil {
		return fmt.Errorf("update calculation run %d: %w", run.RunID, err)
	}
	if err := requireRowsAffected(fmt.Sprintf("update calculation run %d", run.RunID), result.RowsAffected()); err != nil {
		return err
	}
	return nil
}

func calculationRunArguments(run models.CalculationRun, create bool) ([]any, error) {
	parameters, err := normalizedJSONObject(run.Parameters)
	if err != nil {
		return nil, fmt.Errorf("parameters: %w", err)
	}
	solverMetadata, err := normalizedJSONObject(run.SolverMetadata)
	if err != nil {
		return nil, fmt.Errorf("solver metadata: %w", err)
	}

	status := run.Status
	if status == "" {
		status = models.CalculationRunStatusPending
	}

	if create {
		var createdAt any
		if !run.CreatedAt.IsZero() {
			createdAt = run.CreatedAt
		}
		return []any{
			run.Description,
			status,
			parameters,
			createdAt,
			run.StartedAt,
			run.FinishedAt,
			run.ObjectiveValue,
			run.SolverExitFlag,
			run.SolveTimeSeconds,
			solverMetadata,
			run.ErrorMessage,
		}, nil
	}

	return []any{
		run.RunID,
		run.Description,
		status,
		parameters,
		run.StartedAt,
		run.FinishedAt,
		run.ObjectiveValue,
		run.SolverExitFlag,
		run.SolveTimeSeconds,
		solverMetadata,
		run.ErrorMessage,
	}, nil
}

func normalizedJSONObject(value json.RawMessage) (json.RawMessage, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		return json.RawMessage(`{}`), nil
	}

	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, ok := decoded.(map[string]any); !ok {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	return append(json.RawMessage(nil), value...), nil
}

func scanCalculationRun(scanner rowScanner) (models.CalculationRun, error) {
	var run models.CalculationRun
	var description pgtype.Text
	var status string
	var parameters []byte
	var startedAt pgtype.Timestamptz
	var finishedAt pgtype.Timestamptz
	var objectiveValue pgtype.Float8
	var solverExitFlag pgtype.Int4
	var solveTimeSeconds pgtype.Float8
	var solverMetadata []byte
	var errorMessage pgtype.Text

	if err := scanner.Scan(
		&run.RunID,
		&description,
		&status,
		&parameters,
		&run.CreatedAt,
		&startedAt,
		&finishedAt,
		&objectiveValue,
		&solverExitFlag,
		&solveTimeSeconds,
		&solverMetadata,
		&errorMessage,
	); err != nil {
		return models.CalculationRun{}, err
	}

	run.Status = models.CalculationRunStatus(status)
	run.Parameters = append(json.RawMessage(nil), parameters...)
	run.SolverMetadata = append(json.RawMessage(nil), solverMetadata...)
	if description.Valid {
		value := description.String
		run.Description = &value
	}
	if startedAt.Valid {
		value := startedAt.Time
		run.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		run.FinishedAt = &value
	}
	run.ObjectiveValue = nullableFloatPointer(objectiveValue)
	if solverExitFlag.Valid {
		value := int(solverExitFlag.Int32)
		run.SolverExitFlag = &value
	}
	run.SolveTimeSeconds = nullableFloatPointer(solveTimeSeconds)
	if errorMessage.Valid {
		value := errorMessage.String
		run.ErrorMessage = &value
	}
	return run, nil
}
