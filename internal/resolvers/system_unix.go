//go:build !windows

package resolvers

import "os"

// System returns OS-configured IPv4 resolvers from /etc/resolv.conf.
func System() ([]Resolver, error) {
	const path = "/etc/resolv.conf"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return SystemFromResolvConf(path)
}
