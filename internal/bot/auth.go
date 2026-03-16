package bot

type Authorizer struct {
	allowedUsers map[int64]bool
}

func NewAuthorizer(allowedUsers []int64) *Authorizer {
	m := make(map[int64]bool)
	for _, id := range allowedUsers {
		m[id] = true
	}
	return &Authorizer{allowedUsers: m}
}

func (a *Authorizer) IsAllowed(userID int64) bool {
	return a.allowedUsers[userID]
}
