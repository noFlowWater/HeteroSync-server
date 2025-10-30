package repository

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"time-sync-server/internal/models"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return repo, nil
}

func (r *SQLiteRepository) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS time_sync_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device1_id TEXT NOT NULL,
		device1_type TEXT NOT NULL,
		device1_timestamp INTEGER,
		device2_id TEXT NOT NULL,
		device2_type TEXT NOT NULL,
		device2_timestamp INTEGER,
		server_request_time INTEGER NOT NULL,
		server_response_time INTEGER,
		device1_rtt INTEGER,
		device2_rtt INTEGER,
		time_difference INTEGER,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sync_device1 ON time_sync_records(device1_id);
	CREATE INDEX IF NOT EXISTS idx_sync_device2 ON time_sync_records(device2_id);
	CREATE INDEX IF NOT EXISTS idx_sync_created ON time_sync_records(created_at);

	CREATE TABLE IF NOT EXISTS aggregated_sync_results (
		aggregation_id TEXT PRIMARY KEY,
		pairing_id TEXT NOT NULL,
		best_offset INTEGER NOT NULL,
		median_offset INTEGER NOT NULL,
		mean_offset REAL NOT NULL,
		offset_std_dev REAL NOT NULL,
		min_rtt INTEGER NOT NULL,
		max_rtt INTEGER NOT NULL,
		mean_rtt REAL NOT NULL,
		confidence REAL NOT NULL,
		jitter REAL NOT NULL,
		total_samples INTEGER NOT NULL,
		valid_samples INTEGER NOT NULL,
		outlier_count INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_agg_pairing ON aggregated_sync_results(pairing_id);
	CREATE INDEX IF NOT EXISTS idx_agg_created ON aggregated_sync_results(created_at);

	CREATE TABLE IF NOT EXISTS aggregation_measurements (
		aggregation_id TEXT NOT NULL,
		measurement_id INTEGER NOT NULL,
		FOREIGN KEY (aggregation_id) REFERENCES aggregated_sync_results(aggregation_id),
		FOREIGN KEY (measurement_id) REFERENCES time_sync_records(id)
	);

	CREATE INDEX IF NOT EXISTS idx_agg_meas_agg ON aggregation_measurements(aggregation_id);
	CREATE INDEX IF NOT EXISTS idx_agg_meas_meas ON aggregation_measurements(measurement_id);

	CREATE TABLE IF NOT EXISTS pairings (
		pairing_id TEXT PRIMARY KEY,
		device1_id TEXT NOT NULL,
		device2_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		auto_sync_interval_sec INTEGER,
		auto_sync_sample_count INTEGER,
		auto_sync_interval_ms INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_pairing_device1 ON pairings(device1_id);
	CREATE INDEX IF NOT EXISTS idx_pairing_device2 ON pairings(device2_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_pairing_devices ON pairings(device1_id, device2_id);
	`

	_, err := r.db.Exec(schema)
	return err
}

func (r *SQLiteRepository) SaveTimeSyncRecord(record *models.TimeSyncRecord) error {
	query := `
	INSERT INTO time_sync_records (
		device1_id, device1_type, device1_timestamp,
		device2_id, device2_type, device2_timestamp,
		server_request_time, server_response_time,
		device1_rtt, device2_rtt, time_difference,
		status, error_message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.Exec(query,
		record.Device1ID,
		record.Device1Type,
		record.Device1Timestamp,
		record.Device2ID,
		record.Device2Type,
		record.Device2Timestamp,
		record.ServerRequestTime,
		record.ServerResponseTime,
		record.Device1RTT,
		record.Device2RTT,
		record.TimeDifference,
		record.Status,
		record.ErrorMessage,
		record.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save time sync record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	record.ID = id
	return nil
}

// GetTimeSyncRecord retrieves a single time sync record by ID
func (r *SQLiteRepository) GetTimeSyncRecord(id int64) (*models.TimeSyncRecord, error) {
	query := `
	SELECT id, device1_id, device1_type, device1_timestamp,
	       device2_id, device2_type, device2_timestamp,
	       server_request_time, server_response_time,
	       device1_rtt, device2_rtt, time_difference,
	       status, error_message, created_at
	FROM time_sync_records
	WHERE id = ?
	`

	record := &models.TimeSyncRecord{}
	err := r.db.QueryRow(query, id).Scan(
		&record.ID,
		&record.Device1ID,
		&record.Device1Type,
		&record.Device1Timestamp,
		&record.Device2ID,
		&record.Device2Type,
		&record.Device2Timestamp,
		&record.ServerRequestTime,
		&record.ServerResponseTime,
		&record.Device1RTT,
		&record.Device2RTT,
		&record.TimeDifference,
		&record.Status,
		&record.ErrorMessage,
		&record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sync record not found: %d", id)
		}
		return nil, fmt.Errorf("failed to query sync record: %w", err)
	}

	return record, nil
}

func (r *SQLiteRepository) GetTimeSyncRecords(limit, offset int) ([]*models.TimeSyncRecord, error) {
	query := `
	SELECT id, device1_id, device1_type, device1_timestamp,
	       device2_id, device2_type, device2_timestamp,
	       server_request_time, server_response_time,
	       device1_rtt, device2_rtt, time_difference,
	       status, error_message, created_at
	FROM time_sync_records
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query time sync records: %w", err)
	}
	defer rows.Close()

	var records []*models.TimeSyncRecord
	for rows.Next() {
		record := &models.TimeSyncRecord{}
		err := rows.Scan(
			&record.ID,
			&record.Device1ID,
			&record.Device1Type,
			&record.Device1Timestamp,
			&record.Device2ID,
			&record.Device2Type,
			&record.Device2Timestamp,
			&record.ServerRequestTime,
			&record.ServerResponseTime,
			&record.Device1RTT,
			&record.Device2RTT,
			&record.TimeDifference,
			&record.Status,
			&record.ErrorMessage,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time sync record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

func (r *SQLiteRepository) GetTimeSyncRecordsByDeviceID(deviceID string, limit, offset int) ([]*models.TimeSyncRecord, error) {
	query := `
	SELECT id, device1_id, device1_type, device1_timestamp,
	       device2_id, device2_type, device2_timestamp,
	       server_request_time, server_response_time,
	       device1_rtt, device2_rtt, time_difference,
	       status, error_message, created_at
	FROM time_sync_records
	WHERE device1_id = ? OR device2_id = ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, deviceID, deviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query time sync records by device: %w", err)
	}
	defer rows.Close()

	var records []*models.TimeSyncRecord
	for rows.Next() {
		record := &models.TimeSyncRecord{}
		err := rows.Scan(
			&record.ID,
			&record.Device1ID,
			&record.Device1Type,
			&record.Device1Timestamp,
			&record.Device2ID,
			&record.Device2Type,
			&record.Device2Timestamp,
			&record.ServerRequestTime,
			&record.ServerResponseTime,
			&record.Device1RTT,
			&record.Device2RTT,
			&record.TimeDifference,
			&record.Status,
			&record.ErrorMessage,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time sync record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

func (r *SQLiteRepository) GetTimeSyncRecordsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*models.TimeSyncRecord, error) {
	query := `
	SELECT id, device1_id, device1_type, device1_timestamp,
	       device2_id, device2_type, device2_timestamp,
	       server_request_time, server_response_time,
	       device1_rtt, device2_rtt, time_difference,
	       status, error_message, created_at
	FROM time_sync_records
	WHERE created_at BETWEEN ? AND ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	startMillis := startTime.UnixMilli()
	endMillis := endTime.UnixMilli()

	rows, err := r.db.Query(query, startMillis, endMillis, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query time sync records by time range: %w", err)
	}
	defer rows.Close()

	var records []*models.TimeSyncRecord
	for rows.Next() {
		record := &models.TimeSyncRecord{}
		err := rows.Scan(
			&record.ID,
			&record.Device1ID,
			&record.Device1Type,
			&record.Device1Timestamp,
			&record.Device2ID,
			&record.Device2Type,
			&record.Device2Timestamp,
			&record.ServerRequestTime,
			&record.ServerResponseTime,
			&record.Device1RTT,
			&record.Device2RTT,
			&record.TimeDifference,
			&record.Status,
			&record.ErrorMessage,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan time sync record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// SaveAggregatedSyncResult saves an aggregated sync result with its measurements
func (r *SQLiteRepository) SaveAggregatedSyncResult(result *models.AggregatedSyncResult) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert aggregated result
	query := `
	INSERT INTO aggregated_sync_results (
		aggregation_id, pairing_id, best_offset, median_offset, mean_offset,
		offset_std_dev, min_rtt, max_rtt, mean_rtt, confidence, jitter,
		total_samples, valid_samples, outlier_count, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.Exec(query,
		result.AggregationID,
		result.PairingID,
		result.BestOffset,
		result.MedianOffset,
		result.MeanOffset,
		result.OffsetStdDev,
		result.MinRTT,
		result.MaxRTT,
		result.MeanRTT,
		result.Confidence,
		result.Jitter,
		result.TotalSamples,
		result.ValidSamples,
		result.OutlierCount,
		result.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert aggregated result: %w", err)
	}

	// Insert links to individual measurements
	linkQuery := `INSERT INTO aggregation_measurements (aggregation_id, measurement_id) VALUES (?, ?)`
	for _, measurement := range result.Measurements {
		if measurement.ID == 0 {
			continue // Skip measurements without ID
		}
		_, err = tx.Exec(linkQuery, result.AggregationID, measurement.ID)
		if err != nil {
			return fmt.Errorf("failed to link measurement: %w", err)
		}
	}

	return tx.Commit()
}

// GetAggregatedSyncResult retrieves an aggregated sync result by ID
func (r *SQLiteRepository) GetAggregatedSyncResult(aggregationID string) (*models.AggregatedSyncResult, error) {
	query := `
	SELECT aggregation_id, pairing_id, best_offset, median_offset, mean_offset,
	       offset_std_dev, min_rtt, max_rtt, mean_rtt, confidence, jitter,
	       total_samples, valid_samples, outlier_count, created_at
	FROM aggregated_sync_results
	WHERE aggregation_id = ?
	`

	result := &models.AggregatedSyncResult{}
	err := r.db.QueryRow(query, aggregationID).Scan(
		&result.AggregationID,
		&result.PairingID,
		&result.BestOffset,
		&result.MedianOffset,
		&result.MeanOffset,
		&result.OffsetStdDev,
		&result.MinRTT,
		&result.MaxRTT,
		&result.MeanRTT,
		&result.Confidence,
		&result.Jitter,
		&result.TotalSamples,
		&result.ValidSamples,
		&result.OutlierCount,
		&result.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("aggregation not found: %s", aggregationID)
		}
		return nil, fmt.Errorf("failed to query aggregated result: %w", err)
	}

	// Load associated measurements
	measurements, err := r.getAggregationMeasurements(aggregationID)
	if err != nil {
		return nil, fmt.Errorf("failed to load measurements: %w", err)
	}
	result.Measurements = measurements

	return result, nil
}

// GetAggregatedSyncResultsByPairing retrieves aggregated results for a pairing
func (r *SQLiteRepository) GetAggregatedSyncResultsByPairing(pairingID string, limit, offset int) ([]*models.AggregatedSyncResult, error) {
	query := `
	SELECT aggregation_id, pairing_id, best_offset, median_offset, mean_offset,
	       offset_std_dev, min_rtt, max_rtt, mean_rtt, confidence, jitter,
	       total_samples, valid_samples, outlier_count, created_at
	FROM aggregated_sync_results
	WHERE pairing_id = ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, pairingID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated results: %w", err)
	}
	defer rows.Close()

	var results []*models.AggregatedSyncResult
	for rows.Next() {
		result := &models.AggregatedSyncResult{}
		err := rows.Scan(
			&result.AggregationID,
			&result.PairingID,
			&result.BestOffset,
			&result.MedianOffset,
			&result.MeanOffset,
			&result.OffsetStdDev,
			&result.MinRTT,
			&result.MaxRTT,
			&result.MeanRTT,
			&result.Confidence,
			&result.Jitter,
			&result.TotalSamples,
			&result.ValidSamples,
			&result.OutlierCount,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregated result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetAllAggregatedSyncResults retrieves all aggregated results
func (r *SQLiteRepository) GetAllAggregatedSyncResults(limit, offset int) ([]*models.AggregatedSyncResult, error) {
	query := `
	SELECT aggregation_id, pairing_id, best_offset, median_offset, mean_offset,
	       offset_std_dev, min_rtt, max_rtt, mean_rtt, confidence, jitter,
	       total_samples, valid_samples, outlier_count, created_at
	FROM aggregated_sync_results
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query all aggregated results: %w", err)
	}
	defer rows.Close()

	var results []*models.AggregatedSyncResult
	for rows.Next() {
		result := &models.AggregatedSyncResult{}
		err := rows.Scan(
			&result.AggregationID,
			&result.PairingID,
			&result.BestOffset,
			&result.MedianOffset,
			&result.MeanOffset,
			&result.OffsetStdDev,
			&result.MinRTT,
			&result.MaxRTT,
			&result.MeanRTT,
			&result.Confidence,
			&result.Jitter,
			&result.TotalSamples,
			&result.ValidSamples,
			&result.OutlierCount,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregated result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetAggregatedSyncResultsByTimeRange retrieves aggregated results within a time range
func (r *SQLiteRepository) GetAggregatedSyncResultsByTimeRange(startTime, endTime time.Time, limit, offset int) ([]*models.AggregatedSyncResult, error) {
	query := `
	SELECT aggregation_id, pairing_id, best_offset, median_offset, mean_offset,
	       offset_std_dev, min_rtt, max_rtt, mean_rtt, confidence, jitter,
	       total_samples, valid_samples, outlier_count, created_at
	FROM aggregated_sync_results
	WHERE created_at BETWEEN ? AND ?
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`

	startMillis := startTime.UnixMilli()
	endMillis := endTime.UnixMilli()

	rows, err := r.db.Query(query, startMillis, endMillis, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated results by time range: %w", err)
	}
	defer rows.Close()

	var results []*models.AggregatedSyncResult
	for rows.Next() {
		result := &models.AggregatedSyncResult{}
		err := rows.Scan(
			&result.AggregationID,
			&result.PairingID,
			&result.BestOffset,
			&result.MedianOffset,
			&result.MeanOffset,
			&result.OffsetStdDev,
			&result.MinRTT,
			&result.MaxRTT,
			&result.MeanRTT,
			&result.Confidence,
			&result.Jitter,
			&result.TotalSamples,
			&result.ValidSamples,
			&result.OutlierCount,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan aggregated result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// getAggregationMeasurements loads all measurements linked to an aggregation
func (r *SQLiteRepository) getAggregationMeasurements(aggregationID string) ([]*models.TimeSyncRecord, error) {
	query := `
	SELECT t.id, t.device1_id, t.device1_type, t.device1_timestamp,
	       t.device2_id, t.device2_type, t.device2_timestamp,
	       t.server_request_time, t.server_response_time,
	       t.device1_rtt, t.device2_rtt, t.time_difference,
	       t.status, t.error_message, t.created_at
	FROM time_sync_records t
	INNER JOIN aggregation_measurements am ON t.id = am.measurement_id
	WHERE am.aggregation_id = ?
	ORDER BY t.created_at ASC
	`

	rows, err := r.db.Query(query, aggregationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query measurements: %w", err)
	}
	defer rows.Close()

	var records []*models.TimeSyncRecord
	for rows.Next() {
		record := &models.TimeSyncRecord{}
		err := rows.Scan(
			&record.ID,
			&record.Device1ID,
			&record.Device1Type,
			&record.Device1Timestamp,
			&record.Device2ID,
			&record.Device2Type,
			&record.Device2Timestamp,
			&record.ServerRequestTime,
			&record.ServerResponseTime,
			&record.Device1RTT,
			&record.Device2RTT,
			&record.TimeDifference,
			&record.Status,
			&record.ErrorMessage,
			&record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan measurement: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}

// SavePairing saves a pairing to the database
func (r *SQLiteRepository) SavePairing(pairing *models.PersistentPairing) error {
	query := `
	INSERT INTO pairings (
		pairing_id, device1_id, device2_id, created_at,
		auto_sync_interval_sec, auto_sync_sample_count, auto_sync_interval_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.Exec(query,
		pairing.PairingID,
		pairing.Device1ID,
		pairing.Device2ID,
		pairing.CreatedAt.UnixMilli(),
		pairing.AutoSyncIntervalSec,
		pairing.AutoSyncSampleCount,
		pairing.AutoSyncIntervalMs,
	)

	if err != nil {
		return fmt.Errorf("failed to save pairing: %w", err)
	}

	return nil
}

// GetPairingByID retrieves a pairing by its ID
func (r *SQLiteRepository) GetPairingByID(pairingID string) (*models.PersistentPairing, error) {
	query := `
	SELECT pairing_id, device1_id, device2_id, created_at,
	       auto_sync_interval_sec, auto_sync_sample_count, auto_sync_interval_ms
	FROM pairings
	WHERE pairing_id = ?
	`

	pairing := &models.PersistentPairing{}
	var createdAtMillis int64

	err := r.db.QueryRow(query, pairingID).Scan(
		&pairing.PairingID,
		&pairing.Device1ID,
		&pairing.Device2ID,
		&createdAtMillis,
		&pairing.AutoSyncIntervalSec,
		&pairing.AutoSyncSampleCount,
		&pairing.AutoSyncIntervalMs,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pairing not found: %s", pairingID)
		}
		return nil, fmt.Errorf("failed to query pairing: %w", err)
	}

	pairing.CreatedAt = time.UnixMilli(createdAtMillis)
	return pairing, nil
}

// GetPairingsByDeviceID retrieves all pairings that include the specified device
func (r *SQLiteRepository) GetPairingsByDeviceID(deviceID string) ([]*models.PersistentPairing, error) {
	query := `
	SELECT pairing_id, device1_id, device2_id, created_at,
	       auto_sync_interval_sec, auto_sync_sample_count, auto_sync_interval_ms
	FROM pairings
	WHERE device1_id = ? OR device2_id = ?
	ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, deviceID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pairings by device: %w", err)
	}
	defer rows.Close()

	var pairings []*models.PersistentPairing
	for rows.Next() {
		pairing := &models.PersistentPairing{}
		var createdAtMillis int64

		err := rows.Scan(
			&pairing.PairingID,
			&pairing.Device1ID,
			&pairing.Device2ID,
			&createdAtMillis,
			&pairing.AutoSyncIntervalSec,
			&pairing.AutoSyncSampleCount,
			&pairing.AutoSyncIntervalMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pairing: %w", err)
		}

		pairing.CreatedAt = time.UnixMilli(createdAtMillis)
		pairings = append(pairings, pairing)
	}

	return pairings, nil
}

// GetPairingByDevices retrieves a pairing by device IDs (bidirectional check)
func (r *SQLiteRepository) GetPairingByDevices(device1ID, device2ID string) (*models.PersistentPairing, error) {
	query := `
	SELECT pairing_id, device1_id, device2_id, created_at,
	       auto_sync_interval_sec, auto_sync_sample_count, auto_sync_interval_ms
	FROM pairings
	WHERE (device1_id = ? AND device2_id = ?) OR (device1_id = ? AND device2_id = ?)
	LIMIT 1
	`

	pairing := &models.PersistentPairing{}
	var createdAtMillis int64

	err := r.db.QueryRow(query, device1ID, device2ID, device2ID, device1ID).Scan(
		&pairing.PairingID,
		&pairing.Device1ID,
		&pairing.Device2ID,
		&createdAtMillis,
		&pairing.AutoSyncIntervalSec,
		&pairing.AutoSyncSampleCount,
		&pairing.AutoSyncIntervalMs,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pairing not found for devices: %s, %s", device1ID, device2ID)
		}
		return nil, fmt.Errorf("failed to query pairing by devices: %w", err)
	}

	pairing.CreatedAt = time.UnixMilli(createdAtMillis)
	return pairing, nil
}

// DeletePairing deletes a pairing from the database
func (r *SQLiteRepository) DeletePairing(pairingID string) error {
	query := `DELETE FROM pairings WHERE pairing_id = ?`

	result, err := r.db.Exec(query, pairingID)
	if err != nil {
		return fmt.Errorf("failed to delete pairing: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("pairing not found: %s", pairingID)
	}

	return nil
}

// GetAllPairings retrieves all pairings from the database
func (r *SQLiteRepository) GetAllPairings() ([]*models.PersistentPairing, error) {
	query := `
	SELECT pairing_id, device1_id, device2_id, created_at,
	       auto_sync_interval_sec, auto_sync_sample_count, auto_sync_interval_ms
	FROM pairings
	ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all pairings: %w", err)
	}
	defer rows.Close()

	var pairings []*models.PersistentPairing
	for rows.Next() {
		pairing := &models.PersistentPairing{}
		var createdAtMillis int64

		err := rows.Scan(
			&pairing.PairingID,
			&pairing.Device1ID,
			&pairing.Device2ID,
			&createdAtMillis,
			&pairing.AutoSyncIntervalSec,
			&pairing.AutoSyncSampleCount,
			&pairing.AutoSyncIntervalMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pairing: %w", err)
		}

		pairing.CreatedAt = time.UnixMilli(createdAtMillis)
		pairings = append(pairings, pairing)
	}

	return pairings, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}
