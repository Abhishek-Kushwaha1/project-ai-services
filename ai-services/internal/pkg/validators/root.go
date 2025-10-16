package validators

import (
	"fmt"
	"os"
)

func RootUser() (int, error) {
	euid := os.Geteuid()

	if euid == 0 {
		return euid, nil
	}
	return euid, fmt.Errorf("current user is not root, euid: %d", euid)
}
