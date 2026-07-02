package auth

// SessionOptions carries optional fields for session creation.
type SessionOptions struct {
	UserID   *string
	UserType *string
}

// SessionOption is a functional option for CreateSession.
type SessionOption func(*SessionOptions)

// WithSessionUser sets the user_id and user_type on the session.
func WithSessionUser(userID, userType string) SessionOption {
	return func(o *SessionOptions) {
		o.UserID = &userID
		o.UserType = &userType
	}
}

// TokenOptions carries optional fields for auth token insertion.
type TokenOptions struct {
	UserType *string
}

// TokenOption is a functional option for InsertAuthToken.
type TokenOption func(*TokenOptions)

// WithTokenUserType sets the user_type on the auth token.
func WithTokenUserType(userType string) TokenOption {
	return func(o *TokenOptions) {
		o.UserType = &userType
	}
}
