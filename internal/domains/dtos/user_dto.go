package dtos

type UserCreateRequest struct {
	Email     string `json:"email"`
	Handler   string `json:"handler"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

type UserUpdateRequest struct {
	ID        uint64
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Handler   string `json:"handler"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}
