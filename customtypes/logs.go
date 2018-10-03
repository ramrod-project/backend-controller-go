package customtypes

// ContainerLog is a log with a container
// name attached.
type ContainerLog struct {
	ContainerID   string `json:"containerID"`
	ContainerName string `json:"containerName"`
	Log           string `json:"msg"`
	ServiceName   string `json:"sourceServiceName,omitempty"`
	LogTimestamp  int32  `json:"rt,omitempty"`
}
