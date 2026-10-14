//go:build !linux || !cgo || !rjgt102

package securechip

import "fmt"

type Identity struct {
	USID string
}

func Authenticate() (Identity, error) {
	return Identity{}, fmt.Errorf("RJGT102 support is not included in this build")
}
