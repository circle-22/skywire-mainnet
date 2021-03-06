package visor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/rpc"
	"sync"
	"time"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/google/uuid"

	"github.com/SkycoinProject/skywire-mainnet/pkg/app"
	"github.com/SkycoinProject/skywire-mainnet/pkg/router"
	"github.com/SkycoinProject/skywire-mainnet/pkg/routing"
	"github.com/SkycoinProject/skywire-mainnet/pkg/snet/snettest"
	"github.com/SkycoinProject/skywire-mainnet/pkg/transport"
	"github.com/SkycoinProject/skywire-mainnet/pkg/util/buildinfo"
	"github.com/SkycoinProject/skywire-mainnet/pkg/util/updater"
)

var (
	// ErrAlreadyServing is returned when an operation fails due to an operation
	// that is currently running.
	ErrAlreadyServing = errors.New("already serving")
)

// RPCClient represents a RPC Client implementation.
type RPCClient interface {
	Summary() (*Summary, error)

	Health() (*HealthInfo, error)
	Uptime() (float64, error)

	Apps() ([]*AppState, error)
	StartApp(appName string) error
	StopApp(appName string) error
	SetAutoStart(appName string, autostart bool) error
	SetSocksPassword(password string) error
	SetSocksClientPK(pk cipher.PubKey) error
	LogsSince(timestamp time.Time, appName string) ([]string, error)

	TransportTypes() ([]string, error)
	Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error)
	Transport(tid uuid.UUID) (*TransportSummary, error)
	AddTransport(remote cipher.PubKey, tpType string, public bool, timeout time.Duration) (*TransportSummary, error)
	RemoveTransport(tid uuid.UUID) error

	DiscoverTransportsByPK(pk cipher.PubKey) ([]*transport.EntryWithStatus, error)
	DiscoverTransportByID(id uuid.UUID) (*transport.EntryWithStatus, error)

	RoutingRules() ([]routing.Rule, error)
	RoutingRule(key routing.RouteID) (routing.Rule, error)
	SaveRoutingRule(rule routing.Rule) error
	RemoveRoutingRule(key routing.RouteID) error

	RouteGroups() ([]RouteGroupInfo, error)

	Restart() error
	Exec(command string) ([]byte, error)
	Update() (bool, error)
	UpdateAvailable() (*updater.Version, error)
}

// RPCClient provides methods to call an RPC Server.
// It implements RPCClient
type rpcClient struct {
	client *rpc.Client
	prefix string
}

// NewRPCClient creates a new RPCClient.
func NewRPCClient(rc *rpc.Client, prefix string) RPCClient {
	return &rpcClient{client: rc, prefix: prefix}
}

// Call calls the internal rpc.Client with the serviceMethod arg prefixed.
func (rc *rpcClient) Call(method string, args, reply interface{}) error {
	return rc.client.Call(rc.prefix+"."+method, args, reply)
}

// Summary calls Summary.
func (rc *rpcClient) Summary() (*Summary, error) {
	out := new(Summary)
	err := rc.Call("Summary", &struct{}{}, out)
	return out, err
}

// Health calls Health
func (rc *rpcClient) Health() (*HealthInfo, error) {
	hi := &HealthInfo{}
	err := rc.Call("Health", &struct{}{}, hi)
	return hi, err
}

// Uptime calls Uptime
func (rc *rpcClient) Uptime() (float64, error) {
	var out float64
	err := rc.Call("Uptime", &struct{}{}, &out)
	return out, err
}

// Apps calls Apps.
func (rc *rpcClient) Apps() ([]*AppState, error) {
	states := make([]*AppState, 0)
	err := rc.Call("Apps", &struct{}{}, &states)
	return states, err
}

// StartApp calls StartApp.
func (rc *rpcClient) StartApp(appName string) error {
	return rc.Call("StartApp", &appName, &struct{}{})
}

// StopApp calls StopApp.
func (rc *rpcClient) StopApp(appName string) error {
	return rc.Call("StopApp", &appName, &struct{}{})
}

// SetAutoStart calls SetAutoStart.
func (rc *rpcClient) SetAutoStart(appName string, autostart bool) error {
	return rc.Call("SetAutoStart", &SetAutoStartIn{
		AppName:   appName,
		AutoStart: autostart,
	}, &struct{}{})
}

// SetSocksPassword calls SetSocksPassword.
func (rc *rpcClient) SetSocksPassword(password string) error {
	return rc.Call("SetSocksPassword", &password, &struct{}{})
}

// SetSocksClientPK calls SetSocksClientPK.
func (rc *rpcClient) SetSocksClientPK(pk cipher.PubKey) error {
	return rc.Call("SetSocksClientPK", &pk, &struct{}{})
}

// LogsSince calls LogsSince
func (rc *rpcClient) LogsSince(timestamp time.Time, appName string) ([]string, error) {
	res := make([]string, 0)

	err := rc.Call("LogsSince", &AppLogsRequest{
		TimeStamp: timestamp,
		AppName:   appName,
	}, &res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// TransportTypes calls TransportTypes.
func (rc *rpcClient) TransportTypes() ([]string, error) {
	var types []string
	err := rc.Call("TransportTypes", &struct{}{}, &types)
	return types, err
}

// Transports calls Transports.
func (rc *rpcClient) Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error) {
	transports := make([]*TransportSummary, 0)
	err := rc.Call("Transports", &TransportsIn{
		FilterTypes:   types,
		FilterPubKeys: pks,
		ShowLogs:      logs,
	}, &transports)
	return transports, err
}

// Transport calls Transport.
func (rc *rpcClient) Transport(tid uuid.UUID) (*TransportSummary, error) {
	var summary TransportSummary
	err := rc.Call("Transport", &tid, &summary)
	return &summary, err
}

// AddTransport calls AddTransport.
func (rc *rpcClient) AddTransport(remote cipher.PubKey, tpType string, public bool, timeout time.Duration) (*TransportSummary, error) {
	var summary TransportSummary
	err := rc.Call("AddTransport", &AddTransportIn{
		RemotePK: remote,
		TpType:   tpType,
		Public:   public,
		Timeout:  timeout,
	}, &summary)
	return &summary, err
}

// RemoveTransport calls RemoveTransport.
func (rc *rpcClient) RemoveTransport(tid uuid.UUID) error {
	return rc.Call("RemoveTransport", &tid, &struct{}{})
}

func (rc *rpcClient) DiscoverTransportsByPK(pk cipher.PubKey) ([]*transport.EntryWithStatus, error) {
	entries := make([]*transport.EntryWithStatus, 0)
	err := rc.Call("DiscoverTransportsByPK", &pk, &entries)
	return entries, err
}

func (rc *rpcClient) DiscoverTransportByID(id uuid.UUID) (*transport.EntryWithStatus, error) {
	var entry transport.EntryWithStatus
	err := rc.Call("DiscoverTransportByID", &id, &entry)
	return &entry, err
}

// RoutingRules calls RoutingRules.
func (rc *rpcClient) RoutingRules() ([]routing.Rule, error) {
	entries := make([]routing.Rule, 0)
	err := rc.Call("RoutingRules", &struct{}{}, &entries)
	return entries, err
}

// RoutingRule calls RoutingRule.
func (rc *rpcClient) RoutingRule(key routing.RouteID) (routing.Rule, error) {
	var rule routing.Rule
	err := rc.Call("RoutingRule", &key, &rule)
	return rule, err
}

// SaveRoutingRule calls SaveRoutingRule.
func (rc *rpcClient) SaveRoutingRule(rule routing.Rule) error {
	return rc.Call("SaveRoutingRule", &rule, &struct{}{})
}

// RemoveRoutingRule calls RemoveRoutingRule.
func (rc *rpcClient) RemoveRoutingRule(key routing.RouteID) error {
	return rc.Call("RemoveRoutingRule", &key, &struct{}{})
}

// RouteGroups calls RouteGroups.
func (rc *rpcClient) RouteGroups() ([]RouteGroupInfo, error) {
	var routegroups []RouteGroupInfo
	err := rc.Call("RouteGroups", &struct{}{}, &routegroups)
	return routegroups, err
}

// Restart calls Restart.
func (rc *rpcClient) Restart() error {
	return rc.Call("Restart", &struct{}{}, &struct{}{})
}

// Exec calls Exec.
func (rc *rpcClient) Exec(command string) ([]byte, error) {
	output := make([]byte, 0)
	err := rc.Call("Exec", &command, &output)
	return output, err
}

// Update calls Update.
func (rc *rpcClient) Update() (bool, error) {
	var updated bool
	err := rc.Call("Update", &struct{}{}, &updated)
	return updated, err
}

// UpdateAvailable calls UpdateAvailable.
func (rc *rpcClient) UpdateAvailable() (*updater.Version, error) {
	var version, empty updater.Version
	err := rc.Call("UpdateAvailable", &struct{}{}, &version)
	if err != nil {
		return nil, err
	}

	if version == empty {
		return nil, nil
	}

	return &version, err
}

// MockRPCClient mocks RPCClient.
type mockRPCClient struct {
	startedAt time.Time
	s         *Summary
	tpTypes   []string
	rt        routing.Table
	appls     app.LogStore
	sync.RWMutex
}

// NewMockRPCClient creates a new mock RPCClient.
func NewMockRPCClient(r *rand.Rand, maxTps int, maxRules int) (cipher.PubKey, RPCClient, error) {
	log := logging.MustGetLogger("mock-rpc-client")

	types := []string{"messaging", "native"}
	localPK, _ := cipher.GenerateKeyPair()

	log.Infof("generating mock client with: localPK(%s) maxTps(%d) maxRules(%d)", localPK, maxTps, maxRules)

	tps := make([]*TransportSummary, r.Intn(maxTps+1))
	for i := range tps {
		remotePK, _ := cipher.GenerateKeyPair()
		tps[i] = &TransportSummary{
			ID:     transport.MakeTransportID(localPK, remotePK, types[r.Int()%len(types)]),
			Local:  localPK,
			Remote: remotePK,
			Type:   types[r.Int()%len(types)],
			Log:    new(transport.LogEntry),
		}
		log.Infof("tp[%2d]: %v", i, tps[i])
	}

	rt := routing.NewTable()
	ruleKeepAlive := router.DefaultRouteKeepAlive

	for i := 0; i < r.Intn(maxRules+1); i++ {
		remotePK, _ := cipher.GenerateKeyPair()
		var lpRaw, rpRaw [2]byte

		if _, err := r.Read(lpRaw[:]); err != nil {
			return cipher.PubKey{}, nil, err
		}

		if _, err := r.Read(rpRaw[:]); err != nil {
			return cipher.PubKey{}, nil, err
		}

		lp := routing.Port(binary.BigEndian.Uint16(lpRaw[:]))
		rp := routing.Port(binary.BigEndian.Uint16(rpRaw[:]))

		fwdRID, err := rt.ReserveKeys(1)
		if err != nil {
			panic(err)
		}

		keys := snettest.GenKeyPairs(2)

		fwdRule := routing.ForwardRule(ruleKeepAlive, fwdRID[0], routing.RouteID(r.Uint32()), uuid.New(), keys[0].PK, keys[1].PK, 0, 0)
		if err := rt.SaveRule(fwdRule); err != nil {
			panic(err)
		}

		appRID, err := rt.ReserveKeys(1)
		if err != nil {
			panic(err)
		}

		consumeRule := routing.ConsumeRule(ruleKeepAlive, appRID[0], localPK, remotePK, lp, rp)
		if err := rt.SaveRule(consumeRule); err != nil {
			panic(err)
		}

		log.Infof("rt[%2da]: %v %v", i, fwdRID, fwdRule.Summary().ForwardFields)
		log.Infof("rt[%2db]: %v %v", i, appRID[0], consumeRule.Summary().ConsumeFields)
	}

	log.Printf("rtCount: %d", rt.Count())

	client := &mockRPCClient{
		s: &Summary{
			PubKey:          localPK,
			BuildInfo:       buildinfo.Get(),
			AppProtoVersion: supportedProtocolVersion,
			Apps: []*AppState{
				{Name: "foo.v1.0", AutoStart: false, Port: 10},
				{Name: "bar.v2.0", AutoStart: false, Port: 20},
			},
			Transports:  tps,
			RoutesCount: rt.Count(),
		},
		tpTypes:   types,
		rt:        rt,
		startedAt: time.Now(),
	}

	return localPK, client, nil
}

func (mc *mockRPCClient) do(write bool, f func() error) error {
	if write {
		mc.Lock()
		defer mc.Unlock()
	} else {
		mc.RLock()
		defer mc.RUnlock()
	}
	return f()
}

// Summary implements RPCClient.
func (mc *mockRPCClient) Summary() (*Summary, error) {
	var out Summary
	err := mc.do(false, func() error {
		out = *mc.s
		for _, a := range mc.s.Apps {
			out.Apps = append(out.Apps, &(*a))
		}
		for _, tp := range mc.s.Transports {
			out.Transports = append(out.Transports, &(*tp))
		}
		out.RoutesCount = mc.s.RoutesCount
		return nil
	})
	return &out, err
}

// Health implements RPCClient
func (mc *mockRPCClient) Health() (*HealthInfo, error) {
	hi := &HealthInfo{
		TransportDiscovery: http.StatusOK,
		RouteFinder:        http.StatusOK,
		SetupNode:          http.StatusOK,
	}

	return hi, nil
}

// Uptime implements RPCClient
func (mc *mockRPCClient) Uptime() (float64, error) {
	return time.Since(mc.startedAt).Seconds(), nil
}

// Apps implements RPCClient.
func (mc *mockRPCClient) Apps() ([]*AppState, error) {
	var apps []*AppState
	err := mc.do(false, func() error {
		for _, a := range mc.s.Apps {
			apps = append(apps, &(*a))
		}
		return nil
	})
	return apps, err
}

// StartApp implements RPCClient.
func (*mockRPCClient) StartApp(string) error {
	return nil
}

// StopApp implements RPCClient.
func (*mockRPCClient) StopApp(string) error {
	return nil
}

// SetAutoStart implements RPCClient.
func (mc *mockRPCClient) SetAutoStart(appName string, autostart bool) error {
	return mc.do(true, func() error {
		for _, a := range mc.s.Apps {
			if a.Name == appName {
				a.AutoStart = autostart
				return nil
			}
		}
		return fmt.Errorf("app of name '%s' does not exist", appName)
	})
}

// SetSocksPassword implements RPCClient.
func (mc *mockRPCClient) SetSocksPassword(string) error {
	return mc.do(true, func() error {
		const socksName = "skysocks"

		for i := range mc.s.Apps {
			if mc.s.Apps[i].Name == socksName {
				return nil
			}
		}

		return fmt.Errorf("app of name '%s' does not exist", socksName)
	})
}

// SetSocksClientPK implements RPCClient.
func (mc *mockRPCClient) SetSocksClientPK(cipher.PubKey) error {
	return mc.do(true, func() error {
		const socksName = "skysocks-client"

		for i := range mc.s.Apps {
			if mc.s.Apps[i].Name == socksName {
				return nil
			}
		}

		return fmt.Errorf("app of name '%s' does not exist", socksName)
	})
}

// LogsSince implements RPCClient. Manually set (*mockRPPClient).appls before calling this function
func (mc *mockRPCClient) LogsSince(timestamp time.Time, _ string) ([]string, error) {
	return mc.appls.LogsSince(timestamp)
}

// TransportTypes implements RPCClient.
func (mc *mockRPCClient) TransportTypes() ([]string, error) {
	return mc.tpTypes, nil
}

// Transports implements RPCClient.
func (mc *mockRPCClient) Transports(types []string, pks []cipher.PubKey, logs bool) ([]*TransportSummary, error) {
	var summaries []*TransportSummary
	err := mc.do(false, func() error {
		for _, tp := range mc.s.Transports {
			if types != nil {
				for _, reqT := range types {
					if tp.Type == reqT {
						goto TypeOK
					}
				}
				continue
			}
		TypeOK:
			if pks != nil {
				for _, reqPK := range pks {
					if tp.Remote == reqPK || tp.Local == reqPK {
						goto PubKeyOK
					}
				}
				continue
			}
		PubKeyOK:
			if !logs {
				temp := *tp
				temp.Log = nil
				summaries = append(summaries, &temp)
			} else {
				summaries = append(summaries, &(*tp))
			}
		}
		return nil
	})
	return summaries, err
}

// Transport implements RPCClient.
func (mc *mockRPCClient) Transport(tid uuid.UUID) (*TransportSummary, error) {
	var summary TransportSummary
	err := mc.do(false, func() error {
		for _, tp := range mc.s.Transports {
			if tp.ID == tid {
				summary = *tp
				return nil
			}
		}
		return fmt.Errorf("transport of id '%s' is not found", tid)
	})
	return &summary, err
}

// AddTransport implements RPCClient.
func (mc *mockRPCClient) AddTransport(remote cipher.PubKey, tpType string, _ bool, _ time.Duration) (*TransportSummary, error) {
	summary := &TransportSummary{
		ID:     transport.MakeTransportID(mc.s.PubKey, remote, tpType),
		Local:  mc.s.PubKey,
		Remote: remote,
		Type:   tpType,
		Log:    new(transport.LogEntry),
	}
	return summary, mc.do(true, func() error {
		mc.s.Transports = append(mc.s.Transports, summary)
		return nil
	})
}

// RemoveTransport implements RPCClient.
func (mc *mockRPCClient) RemoveTransport(tid uuid.UUID) error {
	return mc.do(true, func() error {
		for i, tp := range mc.s.Transports {
			if tp.ID == tid {
				mc.s.Transports = append(mc.s.Transports[:i], mc.s.Transports[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("transport of id '%s' is not found", tid)
	})
}

func (mc *mockRPCClient) DiscoverTransportsByPK(cipher.PubKey) ([]*transport.EntryWithStatus, error) {
	return nil, ErrNotImplemented
}

func (mc *mockRPCClient) DiscoverTransportByID(uuid.UUID) (*transport.EntryWithStatus, error) {
	return nil, ErrNotImplemented
}

// RoutingRules implements RPCClient.
func (mc *mockRPCClient) RoutingRules() ([]routing.Rule, error) {
	return mc.rt.AllRules(), nil
}

// RoutingRule implements RPCClient.
func (mc *mockRPCClient) RoutingRule(key routing.RouteID) (routing.Rule, error) {
	return mc.rt.Rule(key)
}

// SaveRoutingRule implements RPCClient.
func (mc *mockRPCClient) SaveRoutingRule(rule routing.Rule) error {
	return mc.rt.SaveRule(rule)
}

// RemoveRoutingRule implements RPCClient.
func (mc *mockRPCClient) RemoveRoutingRule(key routing.RouteID) error {
	mc.rt.DelRules([]routing.RouteID{key})
	return nil
}

// RouteGroups implements RPCClient.
func (mc *mockRPCClient) RouteGroups() ([]RouteGroupInfo, error) {
	var routeGroups []RouteGroupInfo

	rules := mc.rt.AllRules()
	for _, rule := range rules {
		if rule.Type() != routing.RuleConsume {
			continue
		}

		fwdRID := rule.NextRouteID()
		fwdRule, err := mc.rt.Rule(fwdRID)
		if err != nil {
			return nil, err
		}
		routeGroups = append(routeGroups, RouteGroupInfo{
			ConsumeRule: rule,
			FwdRule:     fwdRule,
		})
	}

	return routeGroups, nil
}

// Restart implements RPCClient.
func (mc *mockRPCClient) Restart() error {
	return nil
}

// Exec implements RPCClient.
func (mc *mockRPCClient) Exec(string) ([]byte, error) {
	return []byte("mock"), nil
}

// Update implements RPCClient.
func (mc *mockRPCClient) Update() (bool, error) {
	return false, nil
}

// UpdateAvailable implements RPCClient.
func (mc *mockRPCClient) UpdateAvailable() (*updater.Version, error) {
	return nil, nil
}
