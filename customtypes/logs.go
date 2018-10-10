package customtypes

// Log is a log with a container
// name attached.
type Log struct {
	ContainerID   string  `json:"ContainerID,omitempty"`
	ContainerName string  `json:"ContainerName,omitempty"`
	Log           string  `json:"msg"`
	ServiceName   string  `json:"sourceServiceName"`
	LogTimestamp  uint64  `json:"rt"`
	//LogTimestamp  int64   `json:"rt"`
}
