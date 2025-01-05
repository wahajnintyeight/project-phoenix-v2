package enum

type APIResponseEnum int

const (
	USER_FETCHED APIResponseEnum = iota + 1000 // 1000
	USER_LOGGED_IN
	ERROR
	USER_FAILED_TO_GET
	NOTIFICATIONS_FETCHED
	NOTIFICATIONS_FETCHED_FAILED
	SESSION_NOT_FOUND
	SESSION_CREATED
	SESSION_NOT_CREATED
	DATA_FETCHED
	DATA_NOT_FETCHED
	INSERTION_SUCCESS
	INSERTION_FAILED
	LOGIN_FAILED
	INVALID_PASSWORD
	REGISTERED_SUCCESS
	EMAIL_EXISTS
	WELCOME
	MESSAGE_RECEIVED
	USER_NOT_FOUND
	USER_LOGGED_OUT
	JOB_CREATED
	JOB_NOT_CREATED
	JOB_FETCHED
	JOB_NOT_FETCHED
	JOB_DELETED
	JOB_NOT_DELETED
	JOB_DOES_NOT_EXIST
	JOB_EXPIRED
	JOB_NOT_EXPIRED
	JOB_ALREADY_EXPIRED
	NOT_LOGGED_IN
	JOB_ALREADY_DELETED
	JOBS_DELETED
	JOB_ALREADY_EXPIRED_OR_NON_EXISTENT
	TRIP_NOT_CREATED
	TRIP_CREATED
	LOCATION_TRACKING_STARTED
	LOCATION_TRACKING_NOT_STARTED
	OTP_WILL_BE_SENT
	OTP_VERIFIED
	TRIP_FOUND
	TRIP_NOT_FOUND
	TRIP_NOT_DELETED
	TRIP_DELETED
	USER_LOCATION_FETCHED
	USER_LOCATION_NOT_FETCHED
	LOCATION_TRACKING_STOPPED
	LOCATION_TRACKING_NOT_STOPPED
	NOTIFICATIONS_ENABLED
	NOTIFICATIONS_DISABLED
	NOTIFICATIONS_NOT_TOGGLED
	USER_LOCATION_HISTORY_FETCHED
	USER_LOCATION_HISTORY_NOT_FETCHED
	USER_TRIP_HISTORY_FETCHED
	USER_TRIP_HISTORY_NOT_FETCHED
	TOTAL_DISTANCE_NOT_FETCHED
	TOTAL_DISTANCE_FETCHED
	GEO_FENCE_ADDED
	GEO_FENCE_NOT_ADDED
	GEO_FENCE_FETCHED
	GEO_FENCE_NOT_FETCHED
	GEO_FENCE_DELETED
	GEO_FENCE_NOT_DELETED
	GEO_FENCE_ALREADY_EXIST
	SESSIONS_LISTED
	PASSWORD_MISMATCH
	REGISTER_FAILED
	SESSION_HEADER_NOT_FOUND
	LOGIN_SESSION_EXPIRED
	CAPTURE_SCREEN_EVENT_SENT
	CAPTURE_SCREEN_EVENT_FAILED
	SCAN_DEVICE_EVENT_SENT
	SCAN_DEVICE_EVENT_FAILED
	DEVICE_NAME_FETCHED
	DEVICE_NAME_FAILED
	DEVICE_NOT_CREATED
	DEVICE_DELETED
	DEVICE_FAILED_TO_DELETE
	DEVICE_ID_NOT_SET
	DEVICE_FOUND
	DEVICE_NOT_FOUND
	PING_DEVICE_EVENT_SENT
	PING_DEVICE_EVENT_FAILED
	DEVICES_FOUND
	ROOM_NOT_CREATED
	ROOM_FOUND
	ROOM_NOT_FOUND
	ROOM_CREATED
	ROOM_DELETED
	ROOM_NOT_DELETED
	ROOM_JOINED
	ROOM_UPDATED
	ROOM_NOT_UPDATED
)
