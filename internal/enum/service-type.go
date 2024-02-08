package enum

type ServiceType int

const (
	// Define constants for each service type
	APIGateway ServiceType = iota
	Location
)

// String provides the string representation of the ServiceType for easier debugging and logging
func (st ServiceType) String() string {
	return [...]string{"APIGateway", "Location"}[st]
}
