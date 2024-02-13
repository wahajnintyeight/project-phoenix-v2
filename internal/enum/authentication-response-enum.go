package enum

type AuthenticationResponseEnum int

const (
	ERROR              AuthenticationResponseEnum = -1
	USER_LOGGED_IN     AuthenticationResponseEnum = iota + 1001 // 1001
	USER_FAILED_TO_GET AuthenticationResponseEnum = iota + 1002 //1004
	INVALID_PASSWORD   AuthenticationResponseEnum = iota + 1012 //1015
	USER_LOGIN_FAILED  AuthenticationResponseEnum = iota + 1010
	REGISTERED_SUCCESS AuthenticationResponseEnum = iota + 1011
	USER_LOGGED_OUT    AuthenticationResponseEnum = iota + 1015
	EMAIL_EXISTS       AuthenticationResponseEnum = iota + 1010
	REGISTER_FAILED    AuthenticationResponseEnum = iota + 1011
	PASSWORD_MISMATCH  AuthenticationResponseEnum = iota + 1013
)
