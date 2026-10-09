package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hilather/go-lab-netconf/internal/app"
	"github.com/hilather/go-lab-netconf/internal/auth"
	"github.com/hilather/go-lab-netconf/internal/control/mcp"
	"github.com/hilather/go-lab-netconf/internal/control/rest"
	"github.com/hilather/go-lab-netconf/internal/datastore"
	"github.com/hilather/go-lab-netconf/internal/model"
	"github.com/hilather/go-lab-netconf/internal/ncserver"
	"github.com/hilather/go-lab-netconf/internal/netconfssh"
	"github.com/hilather/go-lab-netconf/internal/notif"
	"github.com/hilather/go-lab-netconf/internal/observability"
	"github.com/hilather/go-lab-netconf/internal/restconf"
	"github.com/hilather/go-lab-netconf/internal/snapshot"
	"github.com/hilather/go-lab-netconf/internal/web"
)

type serveFlags struct {
	Config           string
	NetconfListen    string
	RestconfListen   string
	ManagementListen string
}

func parseServeFlags(args []string, stderr io.Writer) (serveFlags, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to bootstrap YAML or JSON")
	netconfListen := fs.String("netconf-listen", "", "override NETCONF listen address; off leaves it unbound")
	restconfListen := fs.String("restconf-listen", "", "override RESTCONF listen address; off leaves it unbound")
	mgmtListen := fs.String("management-listen", "off", "override management listen address; off/none/- leaves it unbound")
	if err := fs.Parse(args); err != nil {
		return serveFlags{}, err
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "labnetconf serve: --config is required")
		return serveFlags{}, fmt.Errorf("missing --config")
	}
	return serveFlags{
		Config:           *path,
		NetconfListen:    *netconfListen,
		RestconfListen:   *restconfListen,
		ManagementListen: *mgmtListen,
	}, nil
}

func listenOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "none", "-":
		return true
	default:
		return false
	}
}

func managementOff(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "off", "none", "-":
		return true
	default:
		return false
	}
}

func resolveListener(flag, yamlAddr string, yamlEnabled bool) (addr string, enabled bool) {
	if listenOff(flag) {
		return "", false
	}
	if strings.TrimSpace(flag) == "" {
		if !yamlEnabled {
			return "", false
		}
		return yamlAddr, true
	}
	return strings.TrimSpace(flag), true
}

// productionSink is the one object cmd injects into datastore.New, SSH, and APP.
// GA fails if this still returns Nop.
func productionSink() notif.Sink {
	return notif.New(0)
}

func serveCmd(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, err := parseServeFlags(args, stderr)
	if err != nil {
		return 2
	}
	sink := productionSink()
	metrics := observability.NewRegistry()
	logger := observability.NewStderrLogger(observability.LevelInfo)
	svc, err := app.Boot(ctx, app.Options{
		BootstrapPath: flags.Config,
		Sink:          sink,
		Metrics:       metrics,
		Logger:        logger,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf serve: %v\n", err)
		return 1
	}
	snap := svc.Active()
	if snap == nil || snap.Canonical == nil {
		_, _ = fmt.Fprintln(stderr, "labnetconf serve: no active snapshot")
		return 1
	}
	baseDir := filepath.Dir(flags.Config)
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}

	ncAddr, ncOn := resolveListener(flags.NetconfListen, snap.NetconfAddress, snap.Canonical.Spec.Listeners.Netconf.Enabled)
	rcAddr, rcOn := resolveListener(flags.RestconfListen, snap.RestconfAddress, snap.Canonical.Spec.Listeners.Restconf.Enabled)
	mgmtOff := managementOff(flags.ManagementListen)

	ncUsers, sshUsers, rcUsers, err := dataPlaneUsers(svc, snap, baseDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labnetconf serve: %v\n", err)
		return 1
	}

	ncs := ncserver.New(ncserver.Config{
		Users:   ncUsers,
		Sink:    sink,
		Metrics: metrics,
		HandleFor: func(username string) (datastore.Handle, bool) {
			if h, ok := svc.UserDatastore(username); ok {
				return h, true
			}
			live := svc.Active()
			if live == nil {
				return nil, false
			}
			u, ok := live.UserNamed(username)
			if !ok {
				return nil, false
			}
			return svc.Datastore(u.Profile)
		},
	})
	svc.SetSessionTable(ncSessions{srv: ncs})

	var ncBound, rcBound atomic.Bool
	var sshSrv *netconfssh.Server
	var rcSrv *restconf.Server
	var rcLn net.Listener
	var restSrv *rest.Server
	var mgmtLn net.Listener

	shutdown := func() {
		shctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if restSrv != nil {
			_ = restSrv.Shutdown(shctx)
		}
		if rcLn != nil {
			_ = rcLn.Close()
		}
		if sshSrv != nil {
			_ = sshSrv.Close()
		}
		if mgmtLn != nil {
			_ = mgmtLn.Close()
		}
	}

	if ncOn {
		hostKey := resolvePath(snap.HostKeyFile, baseDir)
		if hostKey == "" {
			_, _ = fmt.Fprintln(stderr, "labnetconf serve: hostKeyFile is required when NETCONF is enabled")
			return 1
		}
		sshSrv, err = netconfssh.New(netconfssh.Config{
			Address:     ncAddr,
			HostKeyFile: hostKey,
			Users:       sshUsers,
			AllowCIDRs:  sshCIDRs(snap),
			Handler:     ncs.Serve,
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: netconf: %v\n", err)
			return 1
		}
		ncBound.Store(true)
		_, _ = fmt.Fprintf(stdout, "labnetconf netconf listen=%s\n", sshSrv.Addr().String())
		go func() {
			if err := sshSrv.Serve(ctx); err != nil && ctx.Err() == nil {
				_, _ = fmt.Fprintf(stderr, "labnetconf netconf: %v\n", err)
			}
		}()
	} else {
		_, _ = fmt.Fprintln(stdout, "labnetconf netconf: not bound")
	}

	if rcOn {
		rcSrv, err = restconf.New(restconf.Config{
			Addr:             rcAddr,
			Users:            rcUsers,
			AllowClientCidrs: restconfCIDRs(snap),
			Metrics:          metrics,
			HandleFor: func(username, profile string) (datastore.Handle, bool) {
				if h, ok := svc.UserDatastore(username); ok {
					return h, true
				}
				return svc.Datastore(profile)
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: restconf: %v\n", err)
			shutdown()
			return 1
		}
		rcLn, err = net.Listen("tcp", rcAddr)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: restconf: %v\n", err)
			shutdown()
			return 1
		}
		rcBound.Store(true)
		_, _ = fmt.Fprintf(stdout, "labnetconf restconf listen=%s\n", rcLn.Addr().String())
		go func() {
			if err := rcSrv.Serve(ctx, rcLn); err != nil && ctx.Err() == nil {
				_, _ = fmt.Fprintf(stderr, "labnetconf restconf: %v\n", err)
			}
		}()
	} else {
		_, _ = fmt.Fprintln(stdout, "labnetconf restconf: not bound")
	}

	var reloadMu sync.Mutex
	reload := func() {
		reloadDataPlane(svc, baseDir, ncs, sshSrv, rcSrv, &reloadMu, stderr)
	}
	svc.OnApply(reload)
	svc.OnReset(reload)

	ready := func() bool {
		if svc.Active() == nil {
			return false
		}
		if ncOn && !ncBound.Load() {
			return false
		}
		if rcOn && !rcBound.Load() {
			return false
		}
		if !mgmtOff && (restSrv == nil || !restSrv.Bound()) {
			return false
		}
		return true
	}

	if !mgmtOff {
		verifier, origins, allowLegacy, err := managementAuth(snap, baseDir)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: management: %v\n", err)
			shutdown()
			return 1
		}
		mcpSrv, err := mcp.New(mcp.Config{
			Service:            svc,
			AllowedOrigins:     origins,
			AllowLegacyClients: allowLegacy,
			Auth:               verifier,
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: mcp: %v\n", err)
			shutdown()
			return 1
		}
		restSrv, err = rest.New(rest.Config{
			Addr:           flags.ManagementListen,
			Service:        svc,
			AllowedOrigins: origins,
			Live:           func() bool { return true },
			Ready:          ready,
			Auth:           verifier,
			Metrics:        metrics,
			Logger:         logger,
			UI:             web.NewHandler(nil),
			MCP:            mcpSrv.Handler(),
			UIEnabled: func() bool {
				live := svc.Active()
				if live == nil || live.Canonical == nil {
					return false
				}
				return live.Canonical.Spec.UI.Enabled
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: management: %v\n", err)
			shutdown()
			return 1
		}
		mgmtLn, err = net.Listen("tcp", flags.ManagementListen)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "labnetconf serve: management: %v\n", err)
			shutdown()
			return 1
		}
		go func() {
			if err := restSrv.Serve(mgmtLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				_, _ = fmt.Fprintf(stderr, "labnetconf management: %v\n", err)
			}
		}()
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && !restSrv.Bound() {
			time.Sleep(5 * time.Millisecond)
		}
		_, _ = fmt.Fprintf(stdout, "labnetconf management listen=%s\n", restSrv.Addr())
	} else {
		_, _ = fmt.Fprintln(stdout, "labnetconf management: not bound")
	}

	<-ctx.Done()
	shutdown()
	_, _ = fmt.Fprintln(stdout, "labnetconf: shutting down")
	return 0
}

// reloadDataPlaneHook is nil in production. Tests set it to overlap
// publishers: it runs after Active and before any listener publish.
var reloadDataPlaneHook func()

// reloadDataPlane pushes the active snapshot's users and admission into
// the running listeners. mu serializes publishers. baseDir is the
// absolute directory of the bootstrap file. Reload failures are written
// to stderr, the writer passed into serveCmd.
func reloadDataPlane(svc *app.App, baseDir string, ncSrv *ncserver.Server, sshSrv *netconfssh.Server, rcSrv *restconf.Server, mu *sync.Mutex, stderr io.Writer) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	if svc == nil {
		return
	}
	snap := svc.Active()
	if reloadDataPlaneHook != nil {
		reloadDataPlaneHook()
	}
	if snap == nil {
		return
	}
	ncUsers, sshUsers, rcUsers, err := dataPlaneUsers(svc, snap, baseDir)
	if err != nil {
		logDataPlaneReload(stderr, err)
		logDataPlaneReload(stderr, failDataPlane(snap, ncSrv, sshSrv, rcSrv))
		return
	}
	if ncSrv != nil {
		ncSrv.ReplaceUsers(ncUsers)
	}
	if sshSrv != nil {
		if err := sshSrv.ReplaceUsers(sshUsers); err != nil {
			logDataPlaneReload(stderr, err)
			logDataPlaneReload(stderr, failDataPlane(snap, ncSrv, sshSrv, rcSrv))
			return
		}
		sshSrv.SetAllow(sshCIDRs(snap))
	}
	if rcSrv != nil {
		rcSrv.ReplaceUsers(rcUsers)
		if err := rcSrv.SetAdmission(restconfCIDRs(snap)); err != nil {
			logDataPlaneReload(stderr, err)
			logDataPlaneReload(stderr, failDataPlane(snap, ncSrv, sshSrv, rcSrv))
			return
		}
	}
}

func logDataPlaneReload(stderr io.Writer, err error) {
	if err == nil {
		return
	}
	if stderr == nil {
		stderr = io.Discard
	}
	_, _ = fmt.Fprintf(stderr, "labnetconf serve: data-plane reload: %v\n", err)
}

func failDataPlane(snap *snapshot.Snapshot, ncSrv *ncserver.Server, sshSrv *netconfssh.Server, rcSrv *restconf.Server) error {
	if sshSrv != nil {
		sshSrv.ClearUsers()
		sshSrv.CloseConns()
		sshSrv.SetAllow(sshCIDRs(snap))
	}
	var admErr error
	if rcSrv != nil {
		rcSrv.ReplaceUsers(nil)
		if err := rcSrv.SetAdmission(restconfCIDRs(snap)); err != nil {
			admErr = err
		}
	}
	if ncSrv != nil {
		ncSrv.ReplaceUsers(nil)
	}
	return admErr
}

func dataPlaneUsers(svc *app.App, snap *snapshot.Snapshot, baseDir string) ([]ncserver.User, []netconfssh.User, []restconf.User, error) {
	ncUsers := make([]ncserver.User, 0, len(snap.Users))
	sshUsers := make([]netconfssh.User, 0, len(snap.Users))
	rcUsers := make([]restconf.User, 0, len(snap.Users))
	for _, u := range snap.Users {
		h, ok := svc.UserDatastore(u.Name)
		if !ok {
			h, _ = svc.Datastore(u.Profile)
		}
		ncUsers = append(ncUsers, ncserver.User{
			Name:       u.Name,
			Profile:    u.Profile,
			Access:     u.Access,
			Handle:     h,
			Namespaces: namespaces(snap, u.Profile),
		})
		sshUsers = append(sshUsers, netconfssh.User{
			Name:               u.Name,
			PasswordFile:       resolvePath(u.PasswordFile, baseDir),
			AuthorizedKeysFile: resolvePath(u.AuthorizedKeysFile, baseDir),
		})
		if u.PasswordFile == "" {
			continue
		}
		pw, err := os.ReadFile(resolvePath(u.PasswordFile, baseDir))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("user %q password: %w", u.Name, err)
		}
		rcUsers = append(rcUsers, restconf.User{
			Name:     u.Name,
			Password: pw,
			Profile:  u.Profile,
			Access:   u.Access,
		})
	}
	return ncUsers, sshUsers, rcUsers, nil
}

func namespaces(snap *snapshot.Snapshot, profile string) map[string]string {
	p, ok := snap.ProfileNamed(profile)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(p.Modules))
	for _, m := range p.Modules {
		if m.Name != "" && m.Namespace != "" {
			out[m.Name] = m.Namespace
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolvePath(p, baseDir string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}

func sshCIDRs(snap *snapshot.Snapshot) []netip.Prefix {
	if snap.AllowDenyAll {
		return []netip.Prefix{}
	}
	return append([]netip.Prefix(nil), snap.Allow...)
}

func managementAuth(snap *snapshot.Snapshot, baseDir string) (*auth.Verifier, []string, bool, error) {
	if snap == nil || snap.Canonical == nil {
		return nil, nil, false, fmt.Errorf("no active snapshot")
	}
	spec := snap.Canonical.Spec
	tokens := make([]model.TokenSpec, len(spec.Auth.Tokens))
	copy(tokens, spec.Auth.Tokens)
	for i := range tokens {
		tokens[i].SecretFile = resolvePath(tokens[i].SecretFile, baseDir)
	}
	authSpec := spec.Auth
	authSpec.Tokens = tokens
	v, err := auth.FromSpec(authSpec)
	if err != nil {
		return nil, nil, false, err
	}
	if err := v.RequireListen(); err != nil {
		return nil, nil, false, err
	}
	origins := append([]string(nil), spec.Management.AllowedOrigins...)
	return v, origins, spec.Management.MCP.AllowLegacyClients, nil
}

func restconfCIDRs(snap *snapshot.Snapshot) []string {
	if snap.AllowDenyAll {
		return []string{}
	}
	out := make([]string, 0, len(snap.Allow))
	for _, p := range snap.Allow {
		out = append(out, p.String())
	}
	return out
}

// ncSessions adapts the NETCONF session table. Kill ends that NETCONF
// session. It does not close the SSH connection.
type ncSessions struct{ srv *ncserver.Server }

func (n ncSessions) ListSessions() []app.Session {
	if n.srv == nil {
		return []app.Session{}
	}
	rows := n.srv.Sessions()
	out := make([]app.Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.Session{ID: row.ID, User: row.Username, Profile: row.Profile})
	}
	return out
}

func (n ncSessions) KillSession(id string) bool {
	if n.srv == nil {
		return false
	}
	return n.srv.Kill(id) == nil
}
