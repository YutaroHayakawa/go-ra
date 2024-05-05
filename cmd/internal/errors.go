// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of go-ra

package internal

type Error struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Kind + ": " + e.Message
}
