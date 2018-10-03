package customtypes

// ContainerLog is a log with a container
// name attached.
type ContainerLog struct {
	ContainerID   string  `json:"ContainerID"`
	ContainerName string  `json:"ContainerName"`
	Log           string  `json:"msg"`
	ServiceName   string  `json:"sourceServiceName,omitempty"`
	LogTimestamp  float64 `json:"rt,omitempty"`
}
