package cache

import "time"

// processArtworkVersionFloor invalidates browser and Plex transcode keys after restart.
// Unix microseconds remain exact in JavaScript numbers and stay stable for this process.
var processArtworkVersionFloor = time.Now().UnixMicro()

func hydratedVersion(reported, generationFloor int64) int64 {
	if reported < generationFloor {
		return generationFloor
	}
	return reported
}

func nextVersion(current, now int64) int64 {
	if current >= now {
		return current + 1
	}
	return now
}
