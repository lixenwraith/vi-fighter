package components

import "time"

// DrainComponent represents a drain entity that moves toward the cursor
// and destroys entities in a 3x3 area while draining score.
// The drain is active when score > 0 and despawns when score reaches 0.
type DrainComponent struct {
	X             int       // Current X position
	Y             int       // Current Y position
	LastMoveTime  time.Time // For 250ms movement timing
	LastDrainTime time.Time // For 250ms drain timing
	IsActive      bool      // Track if drain is active
}
