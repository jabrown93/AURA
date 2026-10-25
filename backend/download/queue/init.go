package downloadqueue

type Status string

const (
	LAST_STATUS_SUCCESS    Status = "Success"
	LAST_STATUS_WARNING    Status = "Warning"
	LAST_STATUS_ERROR      Status = "Error"
	LAST_STATUS_IDLE       Status = "Idle - Queue Empty"
	LAST_STATUS_PROCESSING Status = "Processing"
)

type FileIssues struct {
	Errors   []string
	Warnings []string
}
