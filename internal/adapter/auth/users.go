package auth

// In-memory user store. For a demo / portfolio piece this beats wiring a
// users table + bcrypt rounds + signup flow that nobody is going to read.
// Production would put this in postgres and bcrypt the passwords.

type User struct {
	ID       string
	Username string
	Password string // plaintext; demo only
	Role     string // "user" or "admin"
}

var demoUsers = []User{
	{ID: "u_alice", Username: "alice", Password: "password123", Role: "user"},
	{ID: "u_bob", Username: "bob", Password: "password123", Role: "user"},
	{ID: "u_charlie", Username: "charlie", Password: "password123", Role: "user"},
	{ID: "u_admin", Username: "admin", Password: "admin123", Role: "admin"},
}

// Lookup returns the user matching username + password, or nil.
func Lookup(username, password string) *User {
	for i := range demoUsers {
		u := &demoUsers[i]
		if u.Username == username && u.Password == password {
			return u
		}
	}
	return nil
}

// AllUserIDs is used by the admin "send to user" panel to populate the
// recipient dropdown. Skips the admin row.
func AllUserIDs() []User {
	out := make([]User, 0, len(demoUsers))
	for _, u := range demoUsers {
		if u.Role != "admin" {
			out = append(out, u)
		}
	}
	return out
}
