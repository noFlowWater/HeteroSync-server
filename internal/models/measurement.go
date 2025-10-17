package models

import "math"

// SyncMeasurement represents a single time synchronization measurement with network delay information
// This structure is used for advanced time synchronization with network delay compensation
type SyncMeasurement struct {
	LocalTime1     int64 // Device 1 local time, Milliseconds
	ResponseTime1  int64 // Device 1 round-trip time (RTT), Microseconds
	LocalTime2     int64 // Device 2 local time, Milliseconds
	ResponseTime2  int64 // Device 2 round-trip time (RTT), Microseconds
	TimeDifference int64 // Time difference between devices (processed), Milliseconds
}

// GetAdjustedTimeDifference calculates the time difference between two devices
// with network delay compensation. It adjusts LocalTime values based on the
// network delay to get a more accurate time difference.
// Returns the adjusted time difference in milliseconds.
func (m *SyncMeasurement) GetAdjustedTimeDifference() int64 {
	// RTT(μs) → one-way delay(ms) 변환
	delay1 := float64(m.ResponseTime1) / 2000.0 // RTT/2 -> one-way, μs → ms
	delay2 := float64(m.ResponseTime2) / 2000.0

	// 각 디바이스의 로컬 시간 보정 (네트워크 지연 고려)
	adj1 := float64(m.LocalTime1) - delay1
	adj2 := float64(m.LocalTime2) - delay2

	// 보정된 두 시간의 차이 계산 (signed)
	diff := adj1 - adj2

	// int64 변환 (반올림 후)
	return int64(math.Round(diff))
}

// func (m *SyncMeasurement) GetAdjustedTimeDifference() int64 {
// 	// Calculate one-way network delay for each device (half of round-trip time)
// 	// Convert from microseconds to milliseconds for comparison with LocalTime
// 	networkDelay1 := m.ResponseTime1 / 2 / 1000 // μs -> ms
// 	networkDelay2 := m.ResponseTime2 / 2 / 1000 // μs -> ms

// 	// Store original local times
// 	prevLocalTime1 := m.LocalTime1
// 	prevLocalTime2 := m.LocalTime2

// 	// Calculate measurement delay difference between devices
// 	measurementDelay := networkDelay2 - networkDelay1
// 	// Note:
// 	// negative: Device 1 started measurement later than Device 2
// 	// 0: Both devices measured at the same time
// 	// positive: Device 1 started measurement earlier than Device 2

// 	// Adjust the local time of the later device to align with the earlier one
// 	if measurementDelay > 0 { // Device 1 measured earlier, adjust Device 2
// 		prevLocalTime2 -= measurementDelay // Subtract positive delay to reduce LocalTime2
// 		adjustedLocalTime2 := prevLocalTime2
// 		return abs(m.LocalTime1 - adjustedLocalTime2)
// 	} else if measurementDelay < 0 { // Device 2 measured earlier, adjust Device 1
// 		prevLocalTime1 += measurementDelay // Add negative delay to reduce LocalTime1
// 		adjustedLocalTime1 := prevLocalTime1
// 		return abs(m.LocalTime2 - adjustedLocalTime1)
// 	} else { // Both devices measured at the same time
// 		return abs(m.LocalTime1 - m.LocalTime2)
// 	}
// }

// GetOptimalMeasurement returns the most reliable measurement from multiple attempts
// It selects the measurement with the minimum total network delay and minimum delay difference
func GetOptimalMeasurement(measurements []SyncMeasurement) *SyncMeasurement {
	if len(measurements) == 0 {
		return nil
	}

	bestMeasurement := &measurements[0]
	minNetworkDelay := bestMeasurement.ResponseTime1 + bestMeasurement.ResponseTime2
	minDelayDiff := abs(bestMeasurement.ResponseTime1 - bestMeasurement.ResponseTime2)

	// Select measurement with minimum total network delay and minimum delay difference
	for i := range measurements[1:] {
		m := &measurements[i+1]
		totalDelay := m.ResponseTime1 + m.ResponseTime2
		delayDiff := abs(m.ResponseTime1 - m.ResponseTime2)

		// Choose measurement with smaller total delay, or if equal, smaller delay difference
		if totalDelay < minNetworkDelay || (totalDelay == minNetworkDelay && delayDiff < minDelayDiff) {
			bestMeasurement = m
			minNetworkDelay = totalDelay
			minDelayDiff = delayDiff
		}
	}

	return bestMeasurement
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// NewSyncMeasurementFromRecord creates a SyncMeasurement from a TimeSyncRecord
// This is a helper function to convert TimeSyncRecord data into SyncMeasurement
// for advanced time difference calculation with network delay compensation.
func NewSyncMeasurementFromRecord(record *TimeSyncRecord) *SyncMeasurement {
	// Validate that we have all required data
	if record.Device1Timestamp == nil || record.Device2Timestamp == nil {
		return nil
	}
	if record.Device1RTT == nil || record.Device2RTT == nil {
		return nil
	}

	return &SyncMeasurement{
		LocalTime1:    *record.Device1Timestamp, // Already in milliseconds
		ResponseTime1: *record.Device1RTT,       // Already in microseconds
		LocalTime2:    *record.Device2Timestamp, // Already in milliseconds
		ResponseTime2: *record.Device2RTT,       // Already in microseconds
	}
}
