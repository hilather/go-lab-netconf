package netconfssh

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func loadUsers(users []User) (map[string]*loadedUser, error) {
	out := make(map[string]*loadedUser, len(users))
	for _, u := range users {
		if u.Name == "" {
			return nil, fmt.Errorf("netconfssh: user name is required")
		}
		if _, dup := out[u.Name]; dup {
			return nil, fmt.Errorf("netconfssh: duplicate user %q", u.Name)
		}
		if u.PasswordFile == "" && u.AuthorizedKeysFile == "" {
			return nil, fmt.Errorf("netconfssh: user %q needs passwordFile and/or authorizedKeysFile", u.Name)
		}
		lu := &loadedUser{name: u.Name}
		if u.PasswordFile != "" {
			b, err := os.ReadFile(u.PasswordFile)
			if err != nil {
				return nil, fmt.Errorf("netconfssh: user %q password: %w", u.Name, err)
			}
			b = bytes.TrimRight(b, "\r\n")
			if len(b) == 0 {
				return nil, fmt.Errorf("netconfssh: user %q password file is empty", u.Name)
			}
			lu.password = b
		}
		if u.AuthorizedKeysFile != "" {
			raw, err := os.ReadFile(u.AuthorizedKeysFile)
			if err != nil {
				return nil, fmt.Errorf("netconfssh: user %q authorized keys: %w", u.Name, err)
			}
			if len(bytes.TrimSpace(raw)) == 0 {
				return nil, fmt.Errorf("netconfssh: user %q authorized keys file is empty", u.Name)
			}
			rest := raw
			for len(bytes.TrimSpace(rest)) > 0 {
				key, _, _, next, err := ssh.ParseAuthorizedKey(rest)
				if err != nil {
					return nil, fmt.Errorf("netconfssh: user %q authorized keys: %w", u.Name, err)
				}
				lu.keys = append(lu.keys, key)
				rest = next
			}
			if len(lu.keys) == 0 {
				return nil, fmt.Errorf("netconfssh: user %q authorized keys file is empty", u.Name)
			}
		}
		out[u.Name] = lu
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("netconfssh: at least one user is required")
	}
	return out, nil
}

func (s *Server) passwordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	u, ok := s.users[conn.User()]
	if !ok || len(u.password) == 0 {
		return nil, fmt.Errorf("denied")
	}
	if subtle.ConstantTimeCompare(u.password, password) != 1 {
		return nil, fmt.Errorf("denied")
	}
	return nil, nil
}

func (s *Server) publicKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	u, ok := s.users[conn.User()]
	if !ok {
		return nil, fmt.Errorf("denied")
	}
	want := key.Marshal()
	for _, k := range u.keys {
		if k.Type() == key.Type() && subtle.ConstantTimeCompare(k.Marshal(), want) == 1 {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("denied")
}
