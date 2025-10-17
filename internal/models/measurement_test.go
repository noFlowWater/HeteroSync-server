package models

import (
	"testing"
)

func TestGetAdjustedTimeDifference(t *testing.T) {
	tests := []struct {
		name           string
		measurement    SyncMeasurement
		expectedResult int64
	}{
		{
			name: "Device1 measured earlier",
			measurement: SyncMeasurement{
				LocalTime1:    1000,   // 1 second (ms)
				ResponseTime1: 100000, // 100ms RTT (μs)
				LocalTime2:    1200,   // 1.2 seconds (ms)
				ResponseTime2: 300000, // 300ms RTT (μs)
			},
			// networkDelay1 = 100000/2/1000 = 50ms
			// networkDelay2 = 300000/2/1000 = 150ms
			// measurementDelay = 150 - 50 = 100ms (positive)
			// Adjust Device2: prevLocalTime2 = 1200 - 100 = 1100ms
			// Difference: |1000 - 1100| = 100ms
			expectedResult: 100,
		},
		{
			name: "Device2 measured earlier",
			measurement: SyncMeasurement{
				LocalTime1:    1000,   // 1 second (ms)
				ResponseTime1: 300000, // 300ms RTT (μs)
				LocalTime2:    900,    // 0.9 seconds (ms)
				ResponseTime2: 100000, // 100ms RTT (μs)
			},
			// networkDelay1 = 300000/2/1000 = 150ms
			// networkDelay2 = 100000/2/1000 = 50ms
			// measurementDelay = 50 - 150 = -100ms (negative)
			// Adjust Device1: prevLocalTime1 = 1000 + (-100) = 900ms
			// Difference: |900 - 900| = 0ms
			expectedResult: 0,
		},
		{
			name: "Equal measurement times",
			measurement: SyncMeasurement{
				LocalTime1:    1000,   // 1 second (ms)
				ResponseTime1: 200000, // 200ms RTT (μs)
				LocalTime2:    1100,   // 1.1 seconds (ms)
				ResponseTime2: 200000, // 200ms RTT (μs)
			},
			// measurementDelay = 0
			// Difference: |1000 - 1100| = 100ms
			expectedResult: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.measurement.GetAdjustedTimeDifference()
			if result != tt.expectedResult {
				t.Errorf("GetAdjustedTimeDifference() = %d, expected %d", result, tt.expectedResult)
			}
		})
	}
}

func TestGetOptimalMeasurement(t *testing.T) {
	tests := []struct {
		name         string
		measurements []SyncMeasurement
		expectedIdx  int // Expected index of optimal measurement, -1 for nil
	}{
		{
			name:         "Empty measurements",
			measurements: []SyncMeasurement{},
			expectedIdx:  -1,
		},
		{
			name: "Single measurement",
			measurements: []SyncMeasurement{
				{LocalTime1: 1000, ResponseTime1: 100000, LocalTime2: 1100, ResponseTime2: 100000},
			},
			expectedIdx: 0,
		},
		{
			name: "Select measurement with minimum total delay",
			measurements: []SyncMeasurement{
				{LocalTime1: 1000, ResponseTime1: 100000, LocalTime2: 1100, ResponseTime2: 100000}, // Total: 200000μs
				{LocalTime1: 1000, ResponseTime1: 50000, LocalTime2: 1100, ResponseTime2: 50000},   // Total: 100000μs (best)
				{LocalTime1: 1000, ResponseTime1: 150000, LocalTime2: 1100, ResponseTime2: 150000}, // Total: 300000μs
			},
			expectedIdx: 1,
		},
		{
			name: "Select measurement with minimum delay difference when total delay is equal",
			measurements: []SyncMeasurement{
				{LocalTime1: 1000, ResponseTime1: 100000, LocalTime2: 1100, ResponseTime2: 100000}, // Total: 200000μs, Diff: 0 (best)
				{LocalTime1: 1000, ResponseTime1: 50000, LocalTime2: 1100, ResponseTime2: 150000},  // Total: 200000μs, Diff: 100000μs
				{LocalTime1: 1000, ResponseTime1: 80000, LocalTime2: 1100, ResponseTime2: 120000},  // Total: 200000μs, Diff: 40000μs
			},
			expectedIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetOptimalMeasurement(tt.measurements)
			if tt.expectedIdx == -1 {
				if result != nil {
					t.Errorf("GetOptimalMeasurement() = %v, expected nil", result)
				}
			} else {
				expected := &tt.measurements[tt.expectedIdx]
				if result != expected {
					t.Errorf("GetOptimalMeasurement() returned wrong measurement. Expected index %d", tt.expectedIdx)
				}
			}
		})
	}
}

func TestNewSyncMeasurementFromRecord(t *testing.T) {
	device1Timestamp := int64(1000)
	device2Timestamp := int64(1100)
	device1RTT := int64(100000)
	device2RTT := int64(150000)

	tests := []struct {
		name     string
		record   *TimeSyncRecord
		expected *SyncMeasurement
	}{
		{
			name: "Valid record with all fields",
			record: &TimeSyncRecord{
				Device1Timestamp: &device1Timestamp,
				Device2Timestamp: &device2Timestamp,
				Device1RTT:       &device1RTT,
				Device2RTT:       &device2RTT,
			},
			expected: &SyncMeasurement{
				LocalTime1:    1000,
				ResponseTime1: 100000,
				LocalTime2:    1100,
				ResponseTime2: 150000,
			},
		},
		{
			name: "Record with missing timestamp",
			record: &TimeSyncRecord{
				Device1Timestamp: nil,
				Device2Timestamp: &device2Timestamp,
				Device1RTT:       &device1RTT,
				Device2RTT:       &device2RTT,
			},
			expected: nil,
		},
		{
			name: "Record with missing RTT",
			record: &TimeSyncRecord{
				Device1Timestamp: &device1Timestamp,
				Device2Timestamp: &device2Timestamp,
				Device1RTT:       nil,
				Device2RTT:       &device2RTT,
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewSyncMeasurementFromRecord(tt.record)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("NewSyncMeasurementFromRecord() = %v, expected nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("NewSyncMeasurementFromRecord() = nil, expected %v", tt.expected)
					return
				}
				if result.LocalTime1 != tt.expected.LocalTime1 ||
					result.LocalTime2 != tt.expected.LocalTime2 ||
					result.ResponseTime1 != tt.expected.ResponseTime1 ||
					result.ResponseTime2 != tt.expected.ResponseTime2 {
					t.Errorf("NewSyncMeasurementFromRecord() = %v, expected %v", result, tt.expected)
				}
			}
		})
	}
}
