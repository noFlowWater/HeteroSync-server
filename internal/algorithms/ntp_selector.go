package algorithms

import (
	"fmt"
	"math"
	"sort"

	"time-sync-server/internal/models"
)

// NTPSelector implements the NTP clock selection algorithm
// It filters and selects the best time synchronization measurements
// based on RTT, symmetry, and statistical analysis
type NTPSelector struct {
	config models.NTPFilterConfig
}

// NewNTPSelector creates a new NTPSelector with the given configuration
// If config values are zero, sensible defaults are applied
func NewNTPSelector(config models.NTPFilterConfig) *NTPSelector {
	// Apply default values
	if config.MinSamples == 0 {
		config.MinSamples = 3
	}
	if config.OutlierThreshold == 0 {
		config.OutlierThreshold = 2.0 // 2 standard deviations
	}
	if config.TopPercentile == 0 {
		config.TopPercentile = 0.5 // Top 50%
	}

	return &NTPSelector{config: config}
}

// SelectBestMeasurements applies the complete NTP selection algorithm
// Steps:
// 1. Filter by RTT (select top N% with lowest RTT)
// 2. Sort by RTT symmetry (prefer symmetric network delays)
// 3. Remove statistical outliers based on offset
// 4. Calculate median offset and statistics
func (s *NTPSelector) SelectBestMeasurements(records []*models.TimeSyncRecord) (*models.AggregatedSyncResult, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("no measurements provided")
	}

	// Step 1: RTT filtering
	analyses := s.FilterByRTT(records)
	if len(analyses) == 0 {
		return nil, fmt.Errorf("no valid samples with RTT data")
	}

	// Step 2: RTT symmetry filtering
	analyses = s.FilterByRTTSymmetry(analyses)

	// Step 3: Outlier removal
	validAnalyses := s.RemoveOutliers(analyses)

	// Step 4: Calculate statistics
	result := s.calculateStatistics(records, analyses, validAnalyses)

	return result, nil
}

// FilterByRTT filters measurements by RTT, keeping the top N% with lowest total RTT
// This is NTP Step 1: Select candidates with minimum network delay
func (s *NTPSelector) FilterByRTT(records []*models.TimeSyncRecord) []*models.SampleAnalysis {
	analyses := make([]*models.SampleAnalysis, 0, len(records))

	for _, record := range records {
		// Skip records without RTT data
		if record.Device1RTT == nil || record.Device2RTT == nil {
			continue
		}
		if record.TimeDifference == nil {
			continue
		}

		totalRTT := *record.Device1RTT + *record.Device2RTT
		rttDiff := abs(*record.Device1RTT - *record.Device2RTT)

		// Apply network delay compensation (NTP standard method)
		// TimeDifference contains raw offset (Device1Time - Device2Time)
		// We need to compensate for one-way network delays
		delay1 := float64(*record.Device1RTT) / 2000.0 // RTT/2 -> one-way delay, μs → ms
		delay2 := float64(*record.Device2RTT) / 2000.0

		// Adjust raw offset by removing network delay difference
		// If Device1 has longer delay, it appears to be ahead (needs negative correction)
		// If Device2 has longer delay, it appears to be behind (needs positive correction)
		rawOffset := float64(*record.TimeDifference)
		adjustedOffset := rawOffset - (delay1 - delay2)

		analyses = append(analyses, &models.SampleAnalysis{
			Record:        record,
			TotalRTT:      totalRTT,
			RTTDifference: rttDiff,
			Offset:        int64(math.Round(adjustedOffset)), // Network-compensated offset
			IsOutlier:     false,
		})
	}

	if len(analyses) == 0 {
		return analyses
	}

	// Sort by total RTT (ascending - lower is better)
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].TotalRTT < analyses[j].TotalRTT
	})

	// Select top N%
	cutoff := int(math.Ceil(float64(len(analyses)) * s.config.TopPercentile))
	if cutoff < s.config.MinSamples {
		cutoff = min(s.config.MinSamples, len(analyses))
	}

	return analyses[:cutoff]
}

// FilterByRTTSymmetry assigns scores based on RTT symmetry
// Samples with more symmetric RTT (smaller difference between Device1 and Device2)
// get better scores. This is NTP Step 2: Prefer symmetric network paths
func (s *NTPSelector) FilterByRTTSymmetry(analyses []*models.SampleAnalysis) []*models.SampleAnalysis {
	// Assign selection score: TotalRTT + (RTTDifference * 2)
	// Asymmetric RTT gets penalty
	for _, analysis := range analyses {
		analysis.SelectionScore = float64(analysis.TotalRTT) +
			float64(analysis.RTTDifference)*2.0
	}

	// Re-sort by selection score (ascending - lower is better)
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].SelectionScore < analyses[j].SelectionScore
	})

	return analyses
}

// RemoveOutliers removes statistical outliers based on offset values
// Uses standard deviation to identify samples that deviate significantly
// This is NTP Step 3: Statistical filtering
func (s *NTPSelector) RemoveOutliers(analyses []*models.SampleAnalysis) []*models.SampleAnalysis {
	if len(analyses) < s.config.MinSamples {
		return analyses // Not enough samples to filter
	}

	// Calculate mean and standard deviation of offsets
	mean, stdDev := calculateOffsetStats(analyses)

	// Mark outliers
	threshold := stdDev * s.config.OutlierThreshold
	filtered := make([]*models.SampleAnalysis, 0, len(analyses))

	for _, analysis := range analyses {
		deviation := math.Abs(float64(analysis.Offset) - mean)
		if deviation <= threshold {
			filtered = append(filtered, analysis)
			analysis.IsOutlier = false
		} else {
			analysis.IsOutlier = true
		}
	}

	// If too many samples filtered out, use original
	if len(filtered) < s.config.MinSamples {
		// Reset outlier flags
		for _, analysis := range analyses {
			analysis.IsOutlier = false
		}
		return analyses
	}

	return filtered
}

// calculateStatistics computes all statistics for the aggregated result
func (s *NTPSelector) calculateStatistics(
	allRecords []*models.TimeSyncRecord,
	selectedAnalyses []*models.SampleAnalysis,
	validAnalyses []*models.SampleAnalysis,
) *models.AggregatedSyncResult {

	// Calculate median offset (NTP Step 4)
	medianOffset := calculateMedianOffset(validAnalyses)

	// Calculate mean and standard deviation
	meanOffset, offsetStdDev := calculateOffsetStats(validAnalyses)

	// Calculate RTT statistics
	minRTT, maxRTT, meanRTT, jitter := calculateRTTStats(validAnalyses)

	// Calculate confidence score
	confidence := calculateConfidence(validAnalyses, offsetStdDev, jitter)

	return &models.AggregatedSyncResult{
		BestOffset:   medianOffset,
		MedianOffset: medianOffset,
		MeanOffset:   meanOffset,
		OffsetStdDev: offsetStdDev,
		MinRTT:       minRTT,
		MaxRTT:       maxRTT,
		MeanRTT:      meanRTT,
		Confidence:   confidence,
		Jitter:       jitter,
		TotalSamples: len(allRecords),
		ValidSamples: len(validAnalyses),
		OutlierCount: len(selectedAnalyses) - len(validAnalyses),
		Measurements: allRecords,
	}
}

// Helper functions

// calculateOffsetStats calculates mean and standard deviation of offsets
func calculateOffsetStats(analyses []*models.SampleAnalysis) (mean, stdDev float64) {
	if len(analyses) == 0 {
		return 0, 0
	}

	// Calculate mean
	sum := 0.0
	for _, a := range analyses {
		sum += float64(a.Offset)
	}
	mean = sum / float64(len(analyses))

	// Calculate standard deviation
	varianceSum := 0.0
	for _, a := range analyses {
		diff := float64(a.Offset) - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(analyses))
	stdDev = math.Sqrt(variance)

	return mean, stdDev
}

// calculateMedianOffset calculates the median offset from analyses
func calculateMedianOffset(analyses []*models.SampleAnalysis) int64 {
	if len(analyses) == 0 {
		return 0
	}

	// Extract and sort offsets
	offsets := make([]int64, len(analyses))
	for i, analysis := range analyses {
		offsets[i] = analysis.Offset
	}
	sort.Slice(offsets, func(i, j int) bool {
		return offsets[i] < offsets[j]
	})

	// Return median
	mid := len(offsets) / 2
	if len(offsets)%2 == 0 {
		return (offsets[mid-1] + offsets[mid]) / 2
	}
	return offsets[mid]
}

// calculateRTTStats calculates RTT statistics including jitter
func calculateRTTStats(analyses []*models.SampleAnalysis) (minRTT, maxRTT int64, meanRTT, jitter float64) {
	if len(analyses) == 0 {
		return 0, 0, 0, 0
	}

	minRTT = analyses[0].TotalRTT
	maxRTT = analyses[0].TotalRTT
	sum := 0.0

	for _, a := range analyses {
		if a.TotalRTT < minRTT {
			minRTT = a.TotalRTT
		}
		if a.TotalRTT > maxRTT {
			maxRTT = a.TotalRTT
		}
		sum += float64(a.TotalRTT)
	}

	meanRTT = sum / float64(len(analyses))

	// Calculate jitter (standard deviation of RTT)
	varianceSum := 0.0
	for _, a := range analyses {
		diff := float64(a.TotalRTT) - meanRTT
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(analyses))
	jitter = math.Sqrt(variance)

	return minRTT, maxRTT, meanRTT, jitter
}

// calculateConfidence calculates a confidence score (0.0 to 1.0)
// Higher confidence means more reliable synchronization
// Factors: low offset variance, low jitter, sufficient samples
func calculateConfidence(analyses []*models.SampleAnalysis, offsetStdDev, jitter float64) float64 {
	if len(analyses) == 0 {
		return 0.0
	}

	// Factor 1: Sample count (more samples = higher confidence)
	sampleFactor := math.Min(float64(len(analyses))/10.0, 1.0)

	// Factor 2: Offset consistency (lower stddev = higher confidence)
	// Assume good if stddev < 5ms, poor if > 20ms
	offsetFactor := 1.0 - math.Min(offsetStdDev/20.0, 1.0)

	// Factor 3: Network stability (lower jitter = higher confidence)
	// Assume good if jitter < 1000μs, poor if > 10000μs
	jitterFactor := 1.0 - math.Min(jitter/10000.0, 1.0)

	// Weighted average
	confidence := (sampleFactor*0.3 + offsetFactor*0.4 + jitterFactor*0.3)

	return math.Max(0.0, math.Min(1.0, confidence))
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
