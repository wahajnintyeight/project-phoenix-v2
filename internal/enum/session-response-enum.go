package enum

type SessionResponseEnum int

const (
	SESSION_NOT_FOUND SessionResponseEnum = iota + 1006
	SESSION_CREATED
	SESSION_NOT_CREATED
	SESSIONS_LISTED
)
