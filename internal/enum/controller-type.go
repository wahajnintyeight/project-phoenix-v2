package enum

type ControllerType int

const (
	// Define constants for each service type
	SessionController ControllerType = iota
	UserTripController
	UserController
	UserLocationController
	UserTripHistoryController
	LoginActivityController
	CaptureScreenController
)
