package utils

import (
	"hash/fnv"
	"strconv"
)

const (
	localPortRangeBase = 40000
	localPortRangeSize = 1000
)

// LocalPortFor deterministically derives a local port for a given remote
// endpoint, so the same database always maps to the same local port —
// across tunnel restarts and across team members (e.g. for DBeaver connections).
func LocalPortFor(endpoint string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(endpoint))
	port := localPortRangeBase + int(h.Sum32()%localPortRangeSize)
	return strconv.Itoa(port)
}
