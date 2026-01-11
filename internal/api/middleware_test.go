package api

import (
	"testing"
	"time"
)

func TestNewResponseTimeTracker(t *testing.T) {
	tracker := NewResponseTimeTracker(100)

	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tracker.maxSize != 100 {
		t.Errorf("expected max size 100, got %d", tracker.maxSize)
	}
	if len(tracker.samples) != 0 {
		t.Errorf("expected empty samples, got %d", len(tracker.samples))
	}
}

func TestResponseTimeTracker_Record(t *testing.T) {
	tracker := NewResponseTimeTracker(5)

	// Record some samples
	tracker.Record(10.0)
	tracker.Record(20.0)
	tracker.Record(30.0)

	if len(tracker.samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(tracker.samples))
	}
}

func TestResponseTimeTracker_Record_CircularBuffer(t *testing.T) {
	tracker := NewResponseTimeTracker(3)

	// Fill the buffer
	tracker.Record(10.0)
	tracker.Record(20.0)
	tracker.Record(30.0)

	// Add more - should wrap around
	tracker.Record(40.0)
	tracker.Record(50.0)

	if len(tracker.samples) != 3 {
		t.Errorf("expected 3 samples (max size), got %d", len(tracker.samples))
	}

	// Check that old values were replaced
	// The samples should contain the last 3 values in circular order
	found40 := false
	found50 := false
	for _, s := range tracker.samples {
		if s == 40.0 {
			found40 = true
		}
		if s == 50.0 {
			found50 = true
		}
	}
	if !found40 || !found50 {
		t.Error("expected recent values to be in buffer")
	}
}

func TestResponseTimeTracker_Percentiles_Empty(t *testing.T) {
	tracker := NewResponseTimeTracker(100)

	percentiles := tracker.Percentiles()
	if percentiles != nil {
		t.Error("expected nil percentiles for empty tracker")
	}
}

func TestResponseTimeTracker_Percentiles_SingleSample(t *testing.T) {
	tracker := NewResponseTimeTracker(100)
	tracker.Record(50.0)

	percentiles := tracker.Percentiles()
	if percentiles == nil {
		t.Fatal("expected non-nil percentiles")
	}
	if percentiles.P50 != 50.0 {
		t.Errorf("expected P50 = 50.0, got %f", percentiles.P50)
	}
	if percentiles.P95 != 50.0 {
		t.Errorf("expected P95 = 50.0, got %f", percentiles.P95)
	}
	if percentiles.P99 != 50.0 {
		t.Errorf("expected P99 = 50.0, got %f", percentiles.P99)
	}
}

func TestResponseTimeTracker_Percentiles_MultipleSamples(t *testing.T) {
	tracker := NewResponseTimeTracker(100)

	// Add 100 samples from 1 to 100
	for i := 1; i <= 100; i++ {
		tracker.Record(float64(i))
	}

	percentiles := tracker.Percentiles()
	if percentiles == nil {
		t.Fatal("expected non-nil percentiles")
	}

	// P50 should be around 50
	if percentiles.P50 < 49 || percentiles.P50 > 51 {
		t.Errorf("expected P50 around 50, got %f", percentiles.P50)
	}

	// P95 should be around 95
	if percentiles.P95 < 94 || percentiles.P95 > 96 {
		t.Errorf("expected P95 around 95, got %f", percentiles.P95)
	}

	// P99 should be around 99
	if percentiles.P99 < 98 || percentiles.P99 > 100 {
		t.Errorf("expected P99 around 99, got %f", percentiles.P99)
	}
}

func TestPercentile_EmptySlice(t *testing.T) {
	result := percentile([]float64{}, 50)
	if result != 0 {
		t.Errorf("expected 0 for empty slice, got %f", result)
	}
}

func TestPercentile_SingleValue(t *testing.T) {
	result := percentile([]float64{42.0}, 50)
	if result != 42.0 {
		t.Errorf("expected 42.0, got %f", result)
	}
}

func TestPercentile_Interpolation(t *testing.T) {
	// Test linear interpolation
	sorted := []float64{10.0, 20.0, 30.0, 40.0, 50.0}

	p50 := percentile(sorted, 50)
	if p50 != 30.0 {
		t.Errorf("expected P50 = 30.0, got %f", p50)
	}

	p25 := percentile(sorted, 25)
	if p25 != 20.0 {
		t.Errorf("expected P25 = 20.0, got %f", p25)
	}

	p75 := percentile(sorted, 75)
	if p75 != 40.0 {
		t.Errorf("expected P75 = 40.0, got %f", p75)
	}
}

func TestRecordEventResponseTime(t *testing.T) {
	// Reset the global tracker by recording a known value
	RecordEventResponseTime(100 * time.Millisecond)

	percentiles := GetEventResponseTimePercentiles()
	if percentiles == nil {
		t.Fatal("expected non-nil percentiles after recording")
	}
}

func TestGetEventResponseTimePercentiles_Initial(t *testing.T) {
	// Just verify it doesn't panic
	_ = GetEventResponseTimePercentiles()
}

func BenchmarkResponseTimeTracker_Record(b *testing.B) {
	tracker := NewResponseTimeTracker(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.Record(float64(i % 100))
	}
}

func BenchmarkResponseTimeTracker_Percentiles(b *testing.B) {
	tracker := NewResponseTimeTracker(10000)

	// Fill the tracker
	for i := 0; i < 10000; i++ {
		tracker.Record(float64(i % 100))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.Percentiles()
	}
}

func BenchmarkPercentile(b *testing.B) {
	sorted := make([]float64, 10000)
	for i := 0; i < 10000; i++ {
		sorted[i] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = percentile(sorted, 95)
	}
}
