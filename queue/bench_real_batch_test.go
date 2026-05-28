package queue

import "testing"

func TestSPSCRealBatchSizingCoversAllOps(t *testing.T) {
	tests := []struct {
		name        string
		totalOps    int
		opsPerBatch int
		wantSizes   []int
	}{
		{
			name:        "empty",
			totalOps:    0,
			opsPerBatch: 256,
			wantSizes:   nil,
		},
		{
			name:        "single partial batch",
			totalOps:    17,
			opsPerBatch: 256,
			wantSizes:   []int{17},
		},
		{
			name:        "exact batches",
			totalOps:    512,
			opsPerBatch: 256,
			wantSizes:   []int{256, 256},
		},
		{
			name:        "partial final batch",
			totalOps:    600,
			opsPerBatch: 256,
			wantSizes:   []int{256, 256, 88},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSamples := spscRealBatchSampleCount(tt.totalOps, tt.opsPerBatch)
			if gotSamples != len(tt.wantSizes) {
				t.Fatalf("sample count = %d, want %d", gotSamples, len(tt.wantSizes))
			}

			total := 0
			for sample := range gotSamples {
				got := spscRealBatchSize(tt.totalOps, tt.opsPerBatch, sample)
				if got != tt.wantSizes[sample] {
					t.Fatalf("sample %d size = %d, want %d", sample, got, tt.wantSizes[sample])
				}
				total += got
			}
			if total != tt.totalOps {
				t.Fatalf("covered %d ops, want %d", total, tt.totalOps)
			}
		})
	}
}
