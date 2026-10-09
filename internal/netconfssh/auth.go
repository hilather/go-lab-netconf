package netconfssh

import (
	"bytes"
	"crypto/subtle"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

// credGenExt is the SSH extension that carries a user's credential
// generation. It is not a secret. A missing or unparsable value means
// the connection is closed rather than treated as generation 0.
const credGenExt = "labnetconf-cred-gen"

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
				key, _, options, next, err := ssh.ParseAuthorizedKey(rest)
				if err != nil {
					return nil, fmt.Errorf("netconfssh: user %q authorized keys: %w", u.Name, err)
				}
				lu.keys = append(lu.keys, authKey{key: key, options: append([]string(nil), options...)})
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
	users := s.userMap()
	u, ok := users[conn.User()]
	if !ok || len(u.password) == 0 {
		return nil, fmt.Errorf("denied")
	}
	if subtle.ConstantTimeCompare(u.password, password) != 1 {
		return nil, fmt.Errorf("denied")
	}
	return u.permissions(), nil
}

func (s *Server) publicKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	users := s.userMap()
	u, ok := users[conn.User()]
	if !ok {
		return nil, fmt.Errorf("denied")
	}
	want := key.Marshal()
	for _, k := range u.keys {
		if k.key.Type() == key.Type() && subtle.ConstantTimeCompare(k.key.Marshal(), want) == 1 {
			return u.permissions(), nil
		}
	}
	return nil, fmt.Errorf("denied")
}

func (u *loadedUser) permissions() *ssh.Permissions {
	if u == nil || u.gen == 0 {
		return nil
	}
	return &ssh.Permissions{
		Extensions: map[string]string{
			credGenExt: strconv.FormatUint(u.gen, 10),
		},
	}
}

// credentialsEqual reports whether password bytes and the authorized-key
// multiset match. Each key is the parsed line's key material plus its
// options (from=, command=, restrict, and the rest) in the order
// ParseAuthorizedKey returned them. Line order, blank lines, and comment
// text are not part of the comparison. Options are not enforced at
// handshake; a change still advances the user's generation so existing
// SSH connections close.
func credentialsEqual(a, b *loadedUser) bool {
	if a == nil || b == nil {
		return false
	}
	if !bytes.Equal(a.password, b.password) {
		return false
	}
	if len(a.keys) != len(b.keys) {
		return false
	}
	counts := make(map[string]int, len(a.keys))
	for _, k := range a.keys {
		counts[credentialID(k)]++
	}
	for _, k := range b.keys {
		id := credentialID(k)
		n := counts[id]
		if n == 0 {
			return false
		}
		counts[id] = n - 1
	}
	return true
}

func credentialID(k authKey) string {
	blob := k.key.Marshal()
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(blob)))
	b.WriteByte(':')
	b.Write(blob)
	for _, opt := range k.options {
		b.WriteByte(0)
		b.WriteString(strconv.Itoa(len(opt)))
		b.WriteByte(':')
		b.WriteString(opt)
	}
	return b.String()
}
