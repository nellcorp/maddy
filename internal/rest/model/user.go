package model

type (
	User struct {
		Username string `json:"username" validate:"email"`
		Password string `json:"password,omitempty"`
	}

	CreateUserDto struct {
		Username        string `json:"username" validate:"email"`
		Password        string `json:"password,omitempty"`
		CreateMailboxes bool   `json:"createMailboxes"`
	}

	Password struct {
		Password string `json:"password,omitempty" validate:"required"`
	}
)
