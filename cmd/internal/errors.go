package internal

type Error struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Kind + ": " + e.Message
}
