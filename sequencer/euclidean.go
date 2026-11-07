package sequencer

// GenerateEuclidean generates a Euclidean rhythm pattern using Bresenham's line algorithm
// This distributes k pulses as evenly as possible across n steps
// k = number of pulses (hits)
// n = total number of steps
// Returns a slice of step indices that should be active
func GenerateEuclidean(k, n int) []int {
	if k <= 0 || n <= 0 || k > n {
		return []int{}
	}

	if k == n {
		// All steps active
		indices := make([]int, n)
		for i := 0; i < n; i++ {
			indices[i] = i
		}
		return indices
	}

	// Use Bresenham's line algorithm to distribute pulses evenly
	// This is equivalent to Bjorklund but much simpler to implement correctly
	indices := []int{}
	slope := float64(k) / float64(n)

	for i := 0; i < n; i++ {
		// Check if we should place a pulse at this step
		// Compare the "ideal" position with actual count
		expectedPulses := int(float64(i+1) * slope)
		actualPulses := len(indices)

		if expectedPulses > actualPulses {
			indices = append(indices, i)
		}
	}

	return indices
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
