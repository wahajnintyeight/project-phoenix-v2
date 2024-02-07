package enum

type ServiceType int

const (
	// Define constants for each service type
	APIGateway ServiceType = iota
	Location
)
