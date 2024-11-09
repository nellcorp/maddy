package model

type (
	User struct {
		Username string `json:"username" validate:"email"`
		Password string `json:"password,omitempty"`
	}

	Password struct {
		Password string `json:"password,omitempty" validate:"required"`
	}
)
