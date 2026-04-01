package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Site struct {
	ID        int
	Name      string
	CanaryURL string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TestRun struct {
	ID         int
	Status     string
	TotalSites int
	Passed     int
	Failed     int
	Errored    int
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
}

type TestResult struct {
	ID          int
	RunID       int
	SiteID      int
	Status      string
	ExitCode    *int
	TestsTotal  int
	TestsPassed int
	TestsFailed int
	DurationMs  *int
	Stdout      *string
	Stderr      *string
	ErrorMsg    *string
	StartedAt   *time.Time
	FinishedAt  *time.Time

	// Joined fields
	SiteName  string
	CanaryURL string
}

func (r *TestRun) ProgressPercent() int {
	if r.TotalSites == 0 {
		return 0
	}
	done := r.Passed + r.Failed + r.Errored
	return (done * 100) / r.TotalSites
}

func (r *TestRun) Completed() int {
	return r.Passed + r.Failed + r.Errored
}

type Store struct {
	DB *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{DB: db}
}

// --- Sites ---

func (s *Store) ListSites() ([]Site, error) {
	rows, err := s.DB.Query("SELECT id, name, canary_url, active, created_at, updated_at FROM sites ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.Name, &site.CanaryURL, &site.Active, &site.CreatedAt, &site.UpdatedAt); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (s *Store) ListActiveSites() ([]Site, error) {
	rows, err := s.DB.Query("SELECT id, name, canary_url, active, created_at, updated_at FROM sites WHERE active = true ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []Site
	for rows.Next() {
		var site Site
		if err := rows.Scan(&site.ID, &site.Name, &site.CanaryURL, &site.Active, &site.CreatedAt, &site.UpdatedAt); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (s *Store) GetSite(id int) (*Site, error) {
	var site Site
	err := s.DB.QueryRow("SELECT id, name, canary_url, active, created_at, updated_at FROM sites WHERE id = $1", id).
		Scan(&site.ID, &site.Name, &site.CanaryURL, &site.Active, &site.CreatedAt, &site.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (s *Store) ToggleSite(id int) (*Site, error) {
	var site Site
	err := s.DB.QueryRow(
		"UPDATE sites SET active = NOT active, updated_at = now() WHERE id = $1 RETURNING id, name, canary_url, active, created_at, updated_at",
		id,
	).Scan(&site.ID, &site.Name, &site.CanaryURL, &site.Active, &site.CreatedAt, &site.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (s *Store) UpsertSite(name, canaryURL string, active bool) error {
	_, err := s.DB.Exec(
		`INSERT INTO sites (name, canary_url, active) VALUES ($1, $2, $3)
		 ON CONFLICT (name) DO UPDATE SET canary_url = $2, updated_at = now()`,
		name, canaryURL, active,
	)
	return err
}

// SyncSites upserts all provided sites and removes any DB sites not in the list.
// Associated test_results are deleted via CASCADE.
func (s *Store) SyncSites(entries []SiteEntry) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}

	for _, e := range entries {
		if _, err := tx.Exec(
			`INSERT INTO sites (name, canary_url, active) VALUES ($1, $2, $3)
			 ON CONFLICT (name) DO UPDATE SET canary_url = $2, updated_at = now()`,
			e.Name, e.CanaryURL, e.Active,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("upserting site %s: %w", e.Name, err)
		}
	}

	// Build set of names to keep
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}

	// Delete sites not in the list (test_results removed via CASCADE)
	if _, err := tx.Exec(
		`DELETE FROM sites WHERE name != ALL($1)`,
		pq.Array(names),
	); err != nil {
		tx.Rollback()
		return fmt.Errorf("removing stale sites: %w", err)
	}

	return tx.Commit()
}

type SiteEntry struct {
	Name      string
	CanaryURL string
	Active    bool
}

// --- Test Runs ---

func (s *Store) CreateRun(totalSites int) (*TestRun, error) {
	var run TestRun
	err := s.DB.QueryRow(
		`INSERT INTO test_runs (status, total_sites, started_at) VALUES ('running', $1, now())
		 RETURNING id, status, total_sites, passed, failed, errored, started_at, finished_at, created_at`,
		totalSites,
	).Scan(&run.ID, &run.Status, &run.TotalSites, &run.Passed, &run.Failed, &run.Errored, &run.StartedAt, &run.FinishedAt, &run.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) GetRun(id int) (*TestRun, error) {
	var run TestRun
	err := s.DB.QueryRow(
		"SELECT id, status, total_sites, passed, failed, errored, started_at, finished_at, created_at FROM test_runs WHERE id = $1",
		id,
	).Scan(&run.ID, &run.Status, &run.TotalSites, &run.Passed, &run.Failed, &run.Errored, &run.StartedAt, &run.FinishedAt, &run.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) ListRuns() ([]TestRun, error) {
	rows, err := s.DB.Query(
		"SELECT id, status, total_sites, passed, failed, errored, started_at, finished_at, created_at FROM test_runs ORDER BY created_at DESC LIMIT 50",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []TestRun
	for rows.Next() {
		var run TestRun
		if err := rows.Scan(&run.ID, &run.Status, &run.TotalSites, &run.Passed, &run.Failed, &run.Errored, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) LatestRun() (*TestRun, error) {
	var run TestRun
	err := s.DB.QueryRow(
		"SELECT id, status, total_sites, passed, failed, errored, started_at, finished_at, created_at FROM test_runs ORDER BY created_at DESC LIMIT 1",
	).Scan(&run.ID, &run.Status, &run.TotalSites, &run.Passed, &run.Failed, &run.Errored, &run.StartedAt, &run.FinishedAt, &run.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *Store) CompleteRun(id int, status string) error {
	_, err := s.DB.Exec(
		"UPDATE test_runs SET status = $1, finished_at = now() WHERE id = $2",
		status, id,
	)
	return err
}

func (s *Store) CancelRun(id int) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE test_runs SET status = 'cancelled', finished_at = now() WHERE id = $1 AND status = 'running'", id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec("UPDATE test_results SET status = 'skipped', finished_at = now() WHERE run_id = $1 AND status IN ('pending', 'running')", id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// --- Test Results ---

func (s *Store) CreateResult(runID, siteID int) error {
	_, err := s.DB.Exec(
		"INSERT INTO test_results (run_id, site_id, status) VALUES ($1, $2, 'pending')",
		runID, siteID,
	)
	return err
}

func (s *Store) ClaimPendingResult() (*TestResult, error) {
	var r TestResult
	err := s.DB.QueryRow(
		`UPDATE test_results SET status = 'running', started_at = now()
		 WHERE id = (
			SELECT tr.id FROM test_results tr
			JOIN test_runs r ON r.id = tr.run_id
			WHERE tr.status = 'pending' AND r.status = 'running'
			ORDER BY tr.id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, run_id, site_id, status`,
	).Scan(&r.ID, &r.RunID, &r.SiteID, &r.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Fetch site info
	err = s.DB.QueryRow("SELECT name, canary_url FROM sites WHERE id = $1", r.SiteID).
		Scan(&r.SiteName, &r.CanaryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching site for result: %w", err)
	}

	return &r, nil
}

func (s *Store) SubmitResult(resultID int, status string, exitCode, testsTotal, testsPassed, testsFailed, durationMs int, stdout, stderr, errorMsg string) error {
	_, err := s.DB.Exec(
		`UPDATE test_results
		 SET status = $1, exit_code = $2, tests_total = $3, tests_passed = $4, tests_failed = $5,
		     duration_ms = $6, stdout = $7, stderr = $8, error_msg = $9, finished_at = now()
		 WHERE id = $10`,
		status, exitCode, testsTotal, testsPassed, testsFailed, durationMs, stdout, stderr, errorMsg, resultID,
	)
	if err != nil {
		return err
	}

	// Update run counters
	var runID int
	err = s.DB.QueryRow("SELECT run_id FROM test_results WHERE id = $1", resultID).Scan(&runID)
	if err != nil {
		return err
	}

	col := "errored"
	switch status {
	case "passed":
		col = "passed"
	case "failed":
		col = "failed"
	}
	_, err = s.DB.Exec(fmt.Sprintf("UPDATE test_runs SET %s = %s + 1 WHERE id = $1", col, col), runID)
	if err != nil {
		return err
	}

	// Check if run is complete
	var pending int
	err = s.DB.QueryRow(
		"SELECT COUNT(*) FROM test_results WHERE run_id = $1 AND status IN ('pending', 'running')",
		runID,
	).Scan(&pending)
	if err != nil {
		return err
	}

	if pending == 0 {
		_, err = s.DB.Exec("UPDATE test_runs SET status = 'completed', finished_at = now() WHERE id = $1", runID)
	}

	return err
}

func (s *Store) GetResult(id int) (*TestResult, error) {
	var r TestResult
	err := s.DB.QueryRow(
		`SELECT tr.id, tr.run_id, tr.site_id, tr.status, tr.exit_code, tr.tests_total, tr.tests_passed,
		        tr.tests_failed, tr.duration_ms, tr.stdout, tr.stderr, tr.error_msg, tr.started_at, tr.finished_at,
		        s.name, s.canary_url
		 FROM test_results tr JOIN sites s ON s.id = tr.site_id
		 WHERE tr.id = $1`,
		id,
	).Scan(&r.ID, &r.RunID, &r.SiteID, &r.Status, &r.ExitCode, &r.TestsTotal, &r.TestsPassed,
		&r.TestsFailed, &r.DurationMs, &r.Stdout, &r.Stderr, &r.ErrorMsg, &r.StartedAt, &r.FinishedAt,
		&r.SiteName, &r.CanaryURL)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListResultsForRun(runID int) ([]TestResult, error) {
	rows, err := s.DB.Query(
		`SELECT tr.id, tr.run_id, tr.site_id, tr.status, tr.exit_code, tr.tests_total, tr.tests_passed,
		        tr.tests_failed, tr.duration_ms, tr.stdout, tr.stderr, tr.error_msg, tr.started_at, tr.finished_at,
		        s.name, s.canary_url
		 FROM test_results tr JOIN sites s ON s.id = tr.site_id
		 WHERE tr.run_id = $1
		 ORDER BY s.name`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TestResult
	for rows.Next() {
		var r TestResult
		if err := rows.Scan(&r.ID, &r.RunID, &r.SiteID, &r.Status, &r.ExitCode, &r.TestsTotal, &r.TestsPassed,
			&r.TestsFailed, &r.DurationMs, &r.Stdout, &r.Stderr, &r.ErrorMsg, &r.StartedAt, &r.FinishedAt,
			&r.SiteName, &r.CanaryURL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
