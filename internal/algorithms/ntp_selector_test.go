package algorithms

import (
	"testing"

	"time-sync-server/internal/models"
)

// Helper function to create test records
func createTestRecord(id int64, device1RTT, device2RTT, timeDiff int64) *models.TimeSyncRecord {
	return &models.TimeSyncRecord{
		ID:             id,
		Device1RTT:     &device1RTT,
		Device2RTT:     &device2RTT,
		TimeDifference: &timeDiff,
		Status:         models.SyncStatusSuccess,
	}
}

func TestNTPSelector_FilterByRTT(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       3,
		OutlierThreshold: 2.0,
		TopPercentile:    0.5, // Top 50%
	})

	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),  // Total: 11000 - should be selected
		createTestRecord(2, 10000, 15000, -155), // Total: 25000 - should be filtered
		createTestRecord(3, 4000, 5000, -148),  // Total: 9000 - should be selected (best)
		createTestRecord(4, 20000, 25000, -160), // Total: 45000 - should be filtered
	}

	analyses := selector.FilterByRTT(records)

	// Should select top 50% = 2 records, but MinSamples is 3, so it will be 3
	if len(analyses) < 2 {
		t.Errorf("Expected at least 2 analyses, got %d", len(analyses))
	}

	// Should be sorted by RTT, so first should have lowest total RTT
	if analyses[0].TotalRTT != 9000 {
		t.Errorf("Expected first analysis to have RTT 9000, got %d", analyses[0].TotalRTT)
	}
}

func TestNTPSelector_FilterByRTTSymmetry(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{})

	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 15000, -150), // Asymmetric: diff = 10000
		createTestRecord(2, 8000, 9000, -155),  // Symmetric: diff = 1000
	}

	analyses := selector.FilterByRTT(records)
	analyses = selector.FilterByRTTSymmetry(analyses)

	// More symmetric one should have better (lower) score
	if analyses[0].Record.ID != 2 {
		t.Errorf("Expected record 2 (symmetric) to be first, got record %d", analyses[0].Record.ID)
	}

	// Check that asymmetric one has higher score
	if analyses[0].SelectionScore >= analyses[1].SelectionScore {
		t.Errorf("Symmetric record should have lower score")
	}
}

func TestNTPSelector_RemoveOutliers(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       3,
		OutlierThreshold: 2.0, // 2 standard deviations
	})

	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),
		createTestRecord(2, 5000, 6000, -151),
		createTestRecord(3, 5000, 6000, -149),
		createTestRecord(4, 5000, 6000, -500), // Outlier: very different offset
	}

	analyses := selector.FilterByRTT(records)
	filtered := selector.RemoveOutliers(analyses)

	// Outlier should be removed
	if len(filtered) != 3 {
		t.Errorf("Expected 3 valid analyses after outlier removal, got %d", len(filtered))
	}

	// Check that outlier is marked
	for _, a := range analyses {
		if a.Offset == -500 && !a.IsOutlier {
			t.Errorf("Expected record with offset -500 to be marked as outlier")
		}
	}
}

func TestNTPSelector_CalculateMedianOffset(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{})

	// Odd number of samples
	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),
		createTestRecord(2, 5000, 6000, -151),
		createTestRecord(3, 5000, 6000, -149),
	}

	result, err := selector.SelectBestMeasurements(records)
	if err != nil {
		t.Fatalf("SelectBestMeasurements failed: %v", err)
	}

	// Median of [-150, -151, -149] = -150
	if result.MedianOffset != -150 {
		t.Errorf("Expected median offset -150, got %d", result.MedianOffset)
	}

	// Even number of samples
	records2 := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),
		createTestRecord(2, 5000, 6000, -152),
		createTestRecord(3, 5000, 6000, -148),
		createTestRecord(4, 5000, 6000, -146),
	}

	result2, err := selector.SelectBestMeasurements(records2)
	if err != nil {
		t.Fatalf("SelectBestMeasurements failed: %v", err)
	}

	// After filtering and sorting, median calculation
	// The median should be reasonable given the input
	if result2.MedianOffset < -152 || result2.MedianOffset > -146 {
		t.Errorf("Expected median offset between -152 and -146, got %d", result2.MedianOffset)
	}
}

func TestNTPSelector_SelectBestMeasurements_Integration(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       3,
		OutlierThreshold: 2.0,
		TopPercentile:    0.5,
	})

	// Create 10 samples with varying RTT and offsets
	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),
		createTestRecord(2, 4000, 5000, -151),
		createTestRecord(3, 6000, 7000, -149),
		createTestRecord(4, 15000, 20000, -155), // High RTT
		createTestRecord(5, 5500, 6500, -150),
		createTestRecord(6, 5000, 6000, -500),   // Outlier offset
		createTestRecord(7, 4500, 5500, -152),
		createTestRecord(8, 25000, 30000, -160), // Very high RTT
		createTestRecord(9, 5000, 6000, -148),
		createTestRecord(10, 4000, 5000, -151),
	}

	result, err := selector.SelectBestMeasurements(records)
	if err != nil {
		t.Fatalf("SelectBestMeasurements failed: %v", err)
	}

	// Validate result structure
	if result.TotalSamples != 10 {
		t.Errorf("Expected 10 total samples, got %d", result.TotalSamples)
	}

	if result.ValidSamples <= 0 {
		t.Errorf("Expected positive valid samples, got %d", result.ValidSamples)
	}

	if result.ValidSamples > result.TotalSamples {
		t.Errorf("Valid samples (%d) cannot exceed total samples (%d)", result.ValidSamples, result.TotalSamples)
	}

	// Confidence should be between 0 and 1
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence should be between 0 and 1, got %f", result.Confidence)
	}

	// Offset should be reasonable (around -150ms, not -500ms outlier)
	if result.BestOffset < -200 || result.BestOffset > -100 {
		t.Errorf("Expected offset around -150ms, got %d", result.BestOffset)
	}

	// Standard deviation should be calculated
	if result.OffsetStdDev < 0 {
		t.Errorf("Standard deviation should be non-negative, got %f", result.OffsetStdDev)
	}

	// Jitter should be calculated
	if result.Jitter < 0 {
		t.Errorf("Jitter should be non-negative, got %f", result.Jitter)
	}

	// Min RTT should be less than Max RTT
	if result.MinRTT >= result.MaxRTT {
		t.Errorf("MinRTT (%d) should be less than MaxRTT (%d)", result.MinRTT, result.MaxRTT)
	}
}

func TestNTPSelector_EmptyRecords(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{})

	records := []*models.TimeSyncRecord{}

	_, err := selector.SelectBestMeasurements(records)
	if err == nil {
		t.Errorf("Expected error for empty records, got nil")
	}
}

func TestNTPSelector_NoValidRTT(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{})

	// Records without RTT data
	records := []*models.TimeSyncRecord{
		{
			ID:             1,
			Device1RTT:     nil, // No RTT
			Device2RTT:     nil,
			TimeDifference: ptrInt64(-150),
			Status:         models.SyncStatusPartial,
		},
	}

	_, err := selector.SelectBestMeasurements(records)
	if err == nil {
		t.Errorf("Expected error for records without RTT, got nil")
	}
}

func TestCalculateConfidence(t *testing.T) {
	// Test high confidence scenario: many samples, low variance
	analyses := []*models.SampleAnalysis{
		{Offset: -150, TotalRTT: 10000},
		{Offset: -151, TotalRTT: 10100},
		{Offset: -149, TotalRTT: 10200},
		{Offset: -150, TotalRTT: 10050},
		{Offset: -151, TotalRTT: 10150},
		{Offset: -150, TotalRTT: 10100},
		{Offset: -149, TotalRTT: 10000},
		{Offset: -150, TotalRTT: 10200},
	}

	confidence := calculateConfidence(analyses, 1.0, 500.0)

	if confidence < 0.7 {
		t.Errorf("Expected high confidence (>0.7) for consistent measurements, got %f", confidence)
	}

	// Test low confidence scenario: few samples, high variance
	analyses2 := []*models.SampleAnalysis{
		{Offset: -150, TotalRTT: 10000},
		{Offset: -200, TotalRTT: 50000},
	}

	confidence2 := calculateConfidence(analyses2, 25.0, 20000.0)

	if confidence2 > 0.3 {
		t.Errorf("Expected low confidence (<0.3) for inconsistent measurements, got %f", confidence2)
	}
}

func TestNTPSelector_NetworkCompensation(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       2,
		OutlierThreshold: 2.0,
		TopPercentile:    1.0, // Select all
	})

	// Scenario: Same raw offset, different RTTs
	// Device1 and Device2 both show -150ms raw difference
	// But different network delays should result in different adjusted offsets
	records := []*models.TimeSyncRecord{
		// Sample 1: Low RTT, symmetric
		// Raw: -150ms, Delay1: 2.5ms, Delay2: 3ms
		// Adjusted: -150 - (2.5 - 3.0) = -150 + 0.5 = -149.5ms
		createTestRecord(1, 5000, 6000, -150),

		// Sample 2: High RTT, asymmetric
		// Raw: -150ms, Delay1: 10ms, Delay2: 15ms
		// Adjusted: -150 - (10 - 15) = -150 + 5 = -145ms
		createTestRecord(2, 20000, 30000, -150),

		// Sample 3: Medium RTT, symmetric
		// Raw: -150ms, Delay1: 5ms, Delay2: 6ms
		// Adjusted: -150 - (5 - 6) = -150 + 1 = -149ms
		createTestRecord(3, 10000, 12000, -150),
	}

	analyses := selector.FilterByRTT(records)

	// All should be selected (TopPercentile = 1.0)
	if len(analyses) != 3 {
		t.Fatalf("Expected 3 analyses, got %d", len(analyses))
	}

	// Check that network compensation was applied
	// Sample 1 (low RTT) should have offset closest to raw (-149.5 ≈ -150)
	// Sample 2 (high RTT, asymmetric) should have offset adjusted more (-145)

	// Find sample 1 and sample 2
	var sample1, sample2 *models.SampleAnalysis
	for _, a := range analyses {
		if a.Record.ID == 1 {
			sample1 = a
		} else if a.Record.ID == 2 {
			sample2 = a
		}
	}

	if sample1 == nil || sample2 == nil {
		t.Fatal("Could not find sample 1 or sample 2")
	}

	// Sample 1: -150 - (2.5 - 3.0) = -149.5 ≈ -150
	if sample1.Offset < -151 || sample1.Offset > -149 {
		t.Errorf("Sample 1 offset expected around -150ms, got %dms", sample1.Offset)
	}

	// Sample 2: -150 - (10 - 15) = -145
	if sample2.Offset < -146 || sample2.Offset > -144 {
		t.Errorf("Sample 2 offset expected around -145ms, got %dms", sample2.Offset)
	}

	// Sample 2 should have larger (less negative) offset due to asymmetry
	if sample2.Offset <= sample1.Offset {
		t.Errorf("Sample 2 (asymmetric) should have larger offset than Sample 1, got %d vs %d",
			sample2.Offset, sample1.Offset)
	}
}

func TestNTPSelector_NetworkCompensation_PreferLowRTT(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{
		MinSamples:       2,
		OutlierThreshold: 2.0,
		TopPercentile:    0.5, // Select top 50%
	})

	// All have same raw offset, but different RTTs
	records := []*models.TimeSyncRecord{
		createTestRecord(1, 5000, 6000, -150),   // Total RTT: 11ms (best)
		createTestRecord(2, 50000, 60000, -150), // Total RTT: 110ms (worst)
		createTestRecord(3, 10000, 12000, -150), // Total RTT: 22ms (good)
		createTestRecord(4, 30000, 35000, -150), // Total RTT: 65ms (bad)
	}

	analyses := selector.FilterByRTT(records)

	// Should select top 50% = 2 samples with lowest RTT
	if len(analyses) != 2 {
		t.Fatalf("Expected 2 analyses (top 50%%), got %d", len(analyses))
	}

	// First should be sample 1 (lowest RTT)
	if analyses[0].Record.ID != 1 {
		t.Errorf("First analysis should be sample 1 (lowest RTT), got sample %d", analyses[0].Record.ID)
	}

	// Second should be sample 3 (second lowest RTT)
	if analyses[1].Record.ID != 3 {
		t.Errorf("Second analysis should be sample 3, got sample %d", analyses[1].Record.ID)
	}

	// Verify that low RTT samples have less adjustment
	sample1Adjustment := analyses[0].Offset - (-150)
	sample3Adjustment := analyses[1].Offset - (-150)

	// Sample 1: delay diff = (2.5 - 3.0) = -0.5
	// Sample 3: delay diff = (5.0 - 6.0) = -1.0
	// Both should have small positive adjustments
	if sample1Adjustment < 0 || sample1Adjustment > 1 {
		t.Errorf("Sample 1 adjustment expected 0-1ms, got %dms", sample1Adjustment)
	}
	if sample3Adjustment < 0 || sample3Adjustment > 2 {
		t.Errorf("Sample 3 adjustment expected 0-2ms, got %dms", sample3Adjustment)
	}
}

func TestNTPSelector_RawVsAdjusted(t *testing.T) {
	selector := NewNTPSelector(models.NTPFilterConfig{})

	// Test that raw and adjusted offsets differ appropriately
	// Large RTT asymmetry should cause large adjustment
	records := []*models.TimeSyncRecord{
		// Very asymmetric: 5ms vs 50ms
		// Raw: -150ms
		// Delay1: 2.5ms, Delay2: 25ms
		// Adjusted: -150 - (2.5 - 25) = -150 + 22.5 = -127.5ms
		createTestRecord(1, 5000, 50000, -150),
	}

	result, err := selector.SelectBestMeasurements(records)
	if err != nil {
		t.Fatalf("SelectBestMeasurements failed: %v", err)
	}

	// Raw offset is -150, but adjusted should be around -128
	if result.BestOffset > -126 || result.BestOffset < -130 {
		t.Errorf("Expected adjusted offset around -128ms (from raw -150ms with asymmetry), got %dms",
			result.BestOffset)
	}

	// The adjustment should be significant (> 20ms)
	adjustment := result.BestOffset - (-150)
	if adjustment < 20 || adjustment > 25 {
		t.Errorf("Expected adjustment around 22-23ms, got %dms", adjustment)
	}
}

// Helper function to create int64 pointer
func ptrInt64(v int64) *int64 {
	return &v
}
