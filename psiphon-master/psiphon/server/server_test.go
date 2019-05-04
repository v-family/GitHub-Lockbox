/*
 * Copyright (c) 2016, Psiphon Inc.
 * All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package server

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/accesscontrol"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/marionette"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/parameters"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/prng"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/protocol"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/tactics"
	"golang.org/x/net/proxy"
)

var serverIPAddress, testDataDirName string
var mockWebServerURL, mockWebServerExpectedResponse string
var mockWebServerPort = 8080

func TestMain(m *testing.M) {
	flag.Parse()

	var err error
	for _, interfaceName := range []string{"eth0", "en0"} {
		var serverIPv4Address, serverIPv6Address net.IP
		serverIPv4Address, serverIPv6Address, err = common.GetInterfaceIPAddresses(interfaceName)
		if err == nil {
			if serverIPv4Address != nil {
				serverIPAddress = serverIPv4Address.String()
			} else {
				serverIPAddress = serverIPv6Address.String()
			}
			break
		}
	}
	if err != nil {
		fmt.Printf("error getting server IP address: %s\n", err)
		os.Exit(1)
	}

	testDataDirName, err = ioutil.TempDir("", "psiphon-server-test")
	if err != nil {
		fmt.Printf("TempDir failed: %s\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(testDataDirName)

	psiphon.SetEmitDiagnosticNotices(true)

	mockWebServerURL, mockWebServerExpectedResponse = runMockWebServer()

	os.Exit(m.Run())
}

func runMockWebServer() (string, string) {

	responseBody := prng.HexString(100000)

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(responseBody))
	})
	webServerAddress := fmt.Sprintf("%s:%d", serverIPAddress, mockWebServerPort)
	server := &http.Server{
		Addr:    webServerAddress,
		Handler: serveMux,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			fmt.Printf("error running mock web server: %s\n", err)
			os.Exit(1)
		}
	}()

	// TODO: properly synchronize with web server readiness
	time.Sleep(1 * time.Second)

	return fmt.Sprintf("http://%s/", webServerAddress), responseBody
}

// Note: not testing fronting meek protocols, which client is
// hard-wired to except running on privileged ports 80 and 443.

func TestSSH(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "SSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestOSSH(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestFragmentedOSSH(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     true,
			forceLivenessTest:    false,
		})
}

func TestUnfrontedMeek(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "UNFRONTED-MEEK-OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUnfrontedMeekHTTPS(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "UNFRONTED-MEEK-HTTPS-OSSH",
			tlsProfile:           protocol.TLS_PROFILE_RANDOMIZED,
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUnfrontedMeekHTTPSTLS13(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "UNFRONTED-MEEK-HTTPS-OSSH",
			tlsProfile:           protocol.TLS_PROFILE_TLS13_RANDOMIZED,
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUnfrontedMeekSessionTicket(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "UNFRONTED-MEEK-SESSION-TICKET-OSSH",
			tlsProfile:           protocol.TLS_PROFILE_RANDOMIZED,
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUnfrontedMeekSessionTicketTLS13(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "UNFRONTED-MEEK-SESSION-TICKET-OSSH",
			tlsProfile:           protocol.TLS_PROFILE_TLS13_RANDOMIZED,
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestQUICOSSH(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "QUIC-OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestMarionetteOSSH(t *testing.T) {
	if !marionette.Enabled() {
		t.Skip("Marionette is not enabled")
	}
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "MARIONETTE-OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestWebTransportAPIRequests(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: false,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: false,
			omitAuthorization:    true,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestHotReload(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestDefaultSponsorID(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   true,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestDenyTrafficRules(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   false,
			denyTrafficRules:     true,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestOmitAuthorization(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    true,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestNoAuthorization(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: false,
			omitAuthorization:    true,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUnusedAuthorization(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          true,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: false,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestTCPOnlySLOK(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: false,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestUDPOnlySLOK(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: false,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    false,
		})
}

func TestLivenessTest(t *testing.T) {
	runServer(t,
		&runServerConfig{
			tunnelProtocol:       "OSSH",
			enableSSHAPIRequests: true,
			doHotReload:          false,
			doDefaultSponsorID:   false,
			denyTrafficRules:     false,
			requireAuthorization: true,
			omitAuthorization:    false,
			doTunneledWebRequest: true,
			doTunneledNTPRequest: true,
			forceFragmenting:     false,
			forceLivenessTest:    true,
		})
}

type runServerConfig struct {
	tunnelProtocol       string
	tlsProfile           string
	enableSSHAPIRequests bool
	doHotReload          bool
	doDefaultSponsorID   bool
	denyTrafficRules     bool
	requireAuthorization bool
	omitAuthorization    bool
	doTunneledWebRequest bool
	doTunneledNTPRequest bool
	forceFragmenting     bool
	forceLivenessTest    bool
}

var (
	testSSHClientVersions = []string{"SSH-2.0-A", "SSH-2.0-B", "SSH-2.0-C"}
	testUserAgents        = []string{"ua1", "ua2", "ua3"}
)

func runServer(t *testing.T, runConfig *runServerConfig) {

	// configure authorized access

	accessType := "test-access-type"

	accessControlSigningKey, accessControlVerificationKey, err := accesscontrol.NewKeyPair(accessType)
	if err != nil {
		t.Fatalf("error creating access control key pair: %s", err)
	}

	accessControlVerificationKeyRing := accesscontrol.VerificationKeyRing{
		Keys: []*accesscontrol.VerificationKey{accessControlVerificationKey},
	}

	var authorizationID [32]byte

	clientAuthorization, err := accesscontrol.IssueAuthorization(
		accessControlSigningKey,
		authorizationID[:],
		time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("error issuing authorization: %s", err)
	}

	// Enable tactics when the test protocol is meek. Both the client and the
	// server will be configured to support tactics. The client config will be
	// set with a nonfunctional config so that the tactics request must
	// succeed, overriding the nonfunctional values, for the tunnel to
	// establish.

	doClientTactics := protocol.TunnelProtocolUsesMeek(runConfig.tunnelProtocol)
	doServerTactics := doClientTactics || runConfig.forceFragmenting

	// All servers require a tactics config with valid keys.
	tacticsRequestPublicKey, tacticsRequestPrivateKey, tacticsRequestObfuscatedKey, err :=
		tactics.GenerateKeys()
	if err != nil {
		t.Fatalf("error generating tactics keys: %s", err)
	}

	livenessTestSize := 0
	if doClientTactics || runConfig.forceLivenessTest {
		livenessTestSize = 1048576
	}

	// create a server

	psiphonServerIPAddress := serverIPAddress
	if protocol.TunnelProtocolUsesQUIC(runConfig.tunnelProtocol) ||
		protocol.TunnelProtocolUsesMarionette(runConfig.tunnelProtocol) {
		// Workaround for macOS firewall.
		psiphonServerIPAddress = "127.0.0.1"
	}

	generateConfigParams := &GenerateConfigParams{
		ServerIPAddress:      psiphonServerIPAddress,
		EnableSSHAPIRequests: runConfig.enableSSHAPIRequests,
		WebServerPort:        8000,
		TunnelProtocolPorts:  map[string]int{runConfig.tunnelProtocol: 4000},
	}

	if protocol.TunnelProtocolUsesMarionette(runConfig.tunnelProtocol) {
		generateConfigParams.TunnelProtocolPorts[runConfig.tunnelProtocol] = 0
		generateConfigParams.MarionetteFormat = "http_simple_nonblocking"
	}

	if doServerTactics {
		generateConfigParams.TacticsRequestPublicKey = tacticsRequestPublicKey
		generateConfigParams.TacticsRequestObfuscatedKey = tacticsRequestObfuscatedKey
	}

	serverConfigJSON, _, _, _, encodedServerEntry, err := GenerateConfig(generateConfigParams)
	if err != nil {
		t.Fatalf("error generating server config: %s", err)
	}

	// customize server config

	// Pave psinet with random values to test handshake homepages.
	psinetFilename := filepath.Join(testDataDirName, "psinet.json")
	sponsorID, expectedHomepageURL := pavePsinetDatabaseFile(
		t, runConfig.doDefaultSponsorID, psinetFilename)

	// Pave OSL config for SLOK testing
	oslConfigFilename := filepath.Join(testDataDirName, "osl_config.json")
	propagationChannelID := paveOSLConfigFile(t, oslConfigFilename)

	// Pave traffic rules file which exercises handshake parameter filtering. Client
	// must handshake with specified sponsor ID in order to allow ports for tunneled
	// requests.
	trafficRulesFilename := filepath.Join(testDataDirName, "traffic_rules.json")
	paveTrafficRulesFile(
		t, trafficRulesFilename, propagationChannelID, accessType,
		runConfig.requireAuthorization, runConfig.denyTrafficRules, livenessTestSize)

	var tacticsConfigFilename string

	// Only pave the tactics config when tactics are required. This exercises the
	// case where the tactics config is omitted.
	if doServerTactics {
		tacticsConfigFilename = filepath.Join(testDataDirName, "tactics_config.json")
		paveTacticsConfigFile(
			t, tacticsConfigFilename,
			tacticsRequestPublicKey, tacticsRequestPrivateKey, tacticsRequestObfuscatedKey,
			runConfig.tunnelProtocol,
			propagationChannelID,
			livenessTestSize)
	}

	blocklistFilename := filepath.Join(testDataDirName, "blocklist.csv")
	paveBlocklistFile(t, blocklistFilename)

	var serverConfig map[string]interface{}
	json.Unmarshal(serverConfigJSON, &serverConfig)
	serverConfig["GeoIPDatabaseFilename"] = ""
	serverConfig["PsinetDatabaseFilename"] = psinetFilename
	serverConfig["TrafficRulesFilename"] = trafficRulesFilename
	serverConfig["OSLConfigFilename"] = oslConfigFilename
	if doServerTactics {
		serverConfig["TacticsConfigFilename"] = tacticsConfigFilename
	}
	serverConfig["BlocklistFilename"] = blocklistFilename

	serverConfig["LogFilename"] = filepath.Join(testDataDirName, "psiphond.log")
	serverConfig["LogLevel"] = "debug"

	serverConfig["AccessControlVerificationKeyRing"] = accessControlVerificationKeyRing

	// Set this parameter so at least the semaphore functions are called.
	// TODO: test that the concurrency limit is correctly enforced.
	serverConfig["MaxConcurrentSSHHandshakes"] = 1

	// Exercise this option.
	serverConfig["PeriodicGarbageCollectionSeconds"] = 1

	serverConfigJSON, _ = json.Marshal(serverConfig)

	serverConnectedLog := make(chan map[string]interface{}, 1)
	serverTunnelLog := make(chan map[string]interface{}, 1)

	setLogCallback(func(log []byte) {

		logFields := make(map[string]interface{})

		err := json.Unmarshal(log, &logFields)
		if err != nil {
			return
		}

		if logFields["event_name"] == nil {
			return
		}

		switch logFields["event_name"].(string) {
		case "connected":
			select {
			case serverConnectedLog <- logFields:
			default:
			}
		case "server_tunnel":
			select {
			case serverTunnelLog <- logFields:
			default:
			}
		}
	})

	// run server

	serverWaitGroup := new(sync.WaitGroup)
	serverWaitGroup.Add(1)
	go func() {
		defer serverWaitGroup.Done()
		err := RunServices(serverConfigJSON)
		if err != nil {
			// TODO: wrong goroutine for t.FatalNow()
			t.Fatalf("error running server: %s", err)
		}
	}()

	stopServer := func() {

		// Test: orderly server shutdown

		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)

		shutdownTimeout := time.NewTimer(5 * time.Second)

		shutdownOk := make(chan struct{}, 1)
		go func() {
			serverWaitGroup.Wait()
			shutdownOk <- *new(struct{})
		}()

		select {
		case <-shutdownOk:
		case <-shutdownTimeout.C:
			t.Fatalf("server shutdown timeout exceeded")
		}
	}

	// Stop server on early exits due to failure.
	defer func() {
		if stopServer != nil {
			stopServer()
		}
	}()

	// TODO: monitor logs for more robust wait-until-loaded. For example,
	// especially with the race detector on, QUIC-OSSH tests can fail as the
	// client sends its initial pacjet before the server is ready.
	time.Sleep(1 * time.Second)

	// Test: hot reload (of psinet and traffic rules)

	if runConfig.doHotReload {

		// Pave new config files with different random values.
		sponsorID, expectedHomepageURL = pavePsinetDatabaseFile(
			t, runConfig.doDefaultSponsorID, psinetFilename)

		propagationChannelID = paveOSLConfigFile(t, oslConfigFilename)

		paveTrafficRulesFile(
			t, trafficRulesFilename, propagationChannelID, accessType,
			runConfig.requireAuthorization, runConfig.denyTrafficRules,
			livenessTestSize)

		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGUSR1)

		// TODO: monitor logs for more robust wait-until-reloaded
		time.Sleep(1 * time.Second)

		// After reloading psinet, the new sponsorID/expectedHomepageURL
		// should be active, as tested in the client "Homepage" notice
		// handler below.
	}

	// Exercise server_load logging
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGUSR2)

	// configure client

	psiphon.RegisterSSHClientVersionPicker(func() string {
		return testSSHClientVersions[prng.Intn(len(testSSHClientVersions))]
	})

	psiphon.RegisterUserAgentPicker(func() string {
		return testUserAgents[prng.Intn(len(testUserAgents))]
	})

	// TODO: currently, TargetServerEntry only works with one tunnel
	numTunnels := 1
	localSOCKSProxyPort := 1081
	localHTTPProxyPort := 8081

	// Use a distinct prefix for network ID for each test run to
	// ensure tactics from different runs don't apply; this is
	// a workaround for the singleton datastore.
	jsonNetworkID := fmt.Sprintf(`,"NetworkID" : "%s-%s"`, time.Now().String(), "NETWORK1")

	jsonLimitTLSProfiles := ""
	if runConfig.tlsProfile != "" {
		jsonLimitTLSProfiles = fmt.Sprintf(`,"LimitTLSProfiles" : ["%s"]`, runConfig.tlsProfile)
	}

	clientConfigJSON := fmt.Sprintf(`
    {
        "ClientPlatform" : "Windows",
        "ClientVersion" : "0",
        "SponsorId" : "0",
        "PropagationChannelId" : "0",
        "TunnelWholeDevice" : 1,
        "DeviceRegion" : "US",
        "DisableRemoteServerListFetcher" : true,
        "EstablishTunnelPausePeriodSeconds" : 1,
        "ConnectionWorkerPoolSize" : %d,
        "LimitTunnelProtocols" : ["%s"]
        %s
        %s
    }`, numTunnels, runConfig.tunnelProtocol, jsonLimitTLSProfiles, jsonNetworkID)

	clientConfig, err := psiphon.LoadConfig([]byte(clientConfigJSON))
	if err != nil {
		t.Fatalf("error processing configuration file: %s", err)
	}

	clientConfig.DataStoreDirectory = testDataDirName

	if !runConfig.doDefaultSponsorID {
		clientConfig.SponsorId = sponsorID
	}
	clientConfig.PropagationChannelId = propagationChannelID
	clientConfig.TunnelPoolSize = numTunnels
	clientConfig.TargetServerEntry = string(encodedServerEntry)
	clientConfig.LocalSocksProxyPort = localSOCKSProxyPort
	clientConfig.LocalHttpProxyPort = localHTTPProxyPort
	clientConfig.EmitSLOKs = true

	if !runConfig.omitAuthorization {
		clientConfig.Authorizations = []string{clientAuthorization}
	}

	err = clientConfig.Commit()
	if err != nil {
		t.Fatalf("error committing configuration file: %s", err)
	}

	if doClientTactics {
		// Configure nonfunctional values that must be overridden by tactics.

		applyParameters := make(map[string]interface{})

		applyParameters[parameters.TunnelConnectTimeout] = "1s"
		applyParameters[parameters.TunnelRateLimits] = common.RateLimits{WriteBytesPerSecond: 1}

		err = clientConfig.SetClientParameters("", true, applyParameters)
		if err != nil {
			t.Fatalf("SetClientParameters failed: %s", err)
		}

	} else {

		// Directly apply same parameters that would've come from tactics.

		applyParameters := make(map[string]interface{})

		if runConfig.forceFragmenting {
			applyParameters[parameters.FragmentorLimitProtocols] = protocol.TunnelProtocols{runConfig.tunnelProtocol}
			applyParameters[parameters.FragmentorProbability] = 1.0
			applyParameters[parameters.FragmentorMinTotalBytes] = 1000
			applyParameters[parameters.FragmentorMaxTotalBytes] = 2000
			applyParameters[parameters.FragmentorMinWriteBytes] = 1
			applyParameters[parameters.FragmentorMaxWriteBytes] = 100
			applyParameters[parameters.FragmentorMinDelay] = 1 * time.Millisecond
			applyParameters[parameters.FragmentorMaxDelay] = 10 * time.Millisecond
		}

		if runConfig.forceLivenessTest {
			applyParameters[parameters.LivenessTestMinUpstreamBytes] = livenessTestSize
			applyParameters[parameters.LivenessTestMaxUpstreamBytes] = livenessTestSize
			applyParameters[parameters.LivenessTestMinDownstreamBytes] = livenessTestSize
			applyParameters[parameters.LivenessTestMaxDownstreamBytes] = livenessTestSize
		}

		err = clientConfig.SetClientParameters("", true, applyParameters)
		if err != nil {
			t.Fatalf("SetClientParameters failed: %s", err)
		}
	}

	// connect to server with client

	err = psiphon.OpenDataStore(clientConfig)
	if err != nil {
		t.Fatalf("error initializing client datastore: %s", err)
	}
	defer psiphon.CloseDataStore()

	psiphon.DeleteSLOKs()

	controller, err := psiphon.NewController(clientConfig)
	if err != nil {
		t.Fatalf("error creating client controller: %s", err)
	}

	tunnelsEstablished := make(chan struct{}, 1)
	homepageReceived := make(chan struct{}, 1)
	slokSeeded := make(chan struct{}, 1)
	clientConnectedNotice := make(chan map[string]interface{}, 1)

	psiphon.SetNoticeWriter(psiphon.NewNoticeReceiver(
		func(notice []byte) {

			//fmt.Printf("%s\n", string(notice))

			noticeType, payload, err := psiphon.GetNotice(notice)
			if err != nil {
				return
			}

			switch noticeType {

			case "Tunnels":
				count := int(payload["count"].(float64))
				if count >= numTunnels {
					sendNotificationReceived(tunnelsEstablished)
				}

			case "Homepage":
				homepageURL := payload["url"].(string)
				if homepageURL != expectedHomepageURL {
					// TODO: wrong goroutine for t.FatalNow()
					t.Fatalf("unexpected homepage: %s", homepageURL)
				}
				sendNotificationReceived(homepageReceived)

			case "SLOKSeeded":
				sendNotificationReceived(slokSeeded)

			case "ConnectedServer":
				select {
				case clientConnectedNotice <- payload:
				default:
				}
			}
		}))

	ctx, cancelFunc := context.WithCancel(context.Background())

	controllerWaitGroup := new(sync.WaitGroup)

	controllerWaitGroup.Add(1)
	go func() {
		defer controllerWaitGroup.Done()
		controller.Run(ctx)
	}()

	stopClient := func() {
		cancelFunc()

		shutdownTimeout := time.NewTimer(20 * time.Second)

		shutdownOk := make(chan struct{}, 1)
		go func() {
			controllerWaitGroup.Wait()
			shutdownOk <- *new(struct{})
		}()

		select {
		case <-shutdownOk:
		case <-shutdownTimeout.C:
			t.Fatalf("controller shutdown timeout exceeded")
		}
	}

	// Stop client on early exits due to failure.
	defer func() {
		if stopClient != nil {
			stopClient()
		}
	}()

	// Test: tunnels must be established, and correct homepage
	// must be received, within 30 seconds

	timeoutSignal := make(chan struct{})
	go func() {
		timer := time.NewTimer(30 * time.Second)
		<-timer.C
		close(timeoutSignal)
	}()

	waitOnNotification(t, tunnelsEstablished, timeoutSignal, "tunnel establish timeout exceeded")
	waitOnNotification(t, homepageReceived, timeoutSignal, "homepage received timeout exceeded")

	expectTrafficFailure := runConfig.denyTrafficRules || (runConfig.omitAuthorization && runConfig.requireAuthorization)

	if runConfig.doTunneledWebRequest {

		// Test: tunneled web site fetch

		err = makeTunneledWebRequest(
			t, localHTTPProxyPort, mockWebServerURL, mockWebServerExpectedResponse)

		if err == nil {
			if expectTrafficFailure {
				t.Fatalf("unexpected tunneled web request success")
			}
		} else {
			if !expectTrafficFailure {
				t.Fatalf("tunneled web request failed: %s", err)
			}
		}
	}

	if runConfig.doTunneledNTPRequest {

		// Test: tunneled UDP packets

		udpgwServerAddress := serverConfig["UDPInterceptUdpgwServerAddress"].(string)

		err = makeTunneledNTPRequest(t, localSOCKSProxyPort, udpgwServerAddress)

		if err == nil {
			if expectTrafficFailure {
				t.Fatalf("unexpected tunneled NTP request success")
			}
		} else {
			if !expectTrafficFailure {
				t.Fatalf("tunneled NTP request failed: %s", err)
			}
		}
	}

	// Test: await SLOK payload

	if !expectTrafficFailure {

		time.Sleep(1 * time.Second)
		waitOnNotification(t, slokSeeded, timeoutSignal, "SLOK seeded timeout exceeded")

		numSLOKs := psiphon.CountSLOKs()
		if numSLOKs != expectedNumSLOKs {
			t.Fatalf("unexpected number of SLOKs: %d", numSLOKs)
		}
	}

	// Shutdown to ensure logs/notices are flushed

	stopClient()
	stopClient = nil
	stopServer()
	stopServer = nil

	// TODO: stops should be fully synchronous, but, intermittently,
	// server_tunnel fails to appear ("missing server tunnel log")
	// without this delay.
	time.Sleep(100 * time.Millisecond)

	// Test: all expected logs/notices were emitted

	select {
	case <-clientConnectedNotice:
	default:
		t.Fatalf("missing client connected notice")
	}

	select {
	case logFields := <-serverConnectedLog:
		err := checkExpectedLogFields(runConfig, logFields)
		if err != nil {
			t.Fatalf("invalid server connected log fields: %s", err)
		}
	default:
		t.Fatalf("missing server connected log")
	}

	select {
	case logFields := <-serverTunnelLog:
		err := checkExpectedLogFields(runConfig, logFields)
		if err != nil {
			t.Fatalf("invalid server tunnel log fields: %s", err)
		}
	default:
		t.Fatalf("missing server tunnel log")
	}
}

func checkExpectedLogFields(runConfig *runServerConfig, fields map[string]interface{}) error {

	// Limitations:
	//
	// - client_build_rev not set in test build (see common/buildinfo.go)
	// - egress_region, upstream_proxy_type, upstream_proxy_custom_header_names not exercised in test
	// - meek_dial_ip_address/meek_resolved_ip_address only logged for FRONTED meek protocols

	for _, name := range []string{
		"session_id",
		"last_connected",
		"establishment_duration",
		"propagation_channel_id",
		"sponsor_id",
		"client_platform",
		"relay_protocol",
		"tunnel_whole_device",
		"device_region",
		"ssh_client_version",
		"server_entry_region",
		"server_entry_source",
		"server_entry_timestamp",
		"dial_port_number",
		"is_replay",
		"dial_duration",
		"candidate_number",
	} {
		if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
			return fmt.Errorf("missing expected field '%s'", name)
		}
	}

	if fields["relay_protocol"] != runConfig.tunnelProtocol {
		return fmt.Errorf("unexpected relay_protocol '%s'", fields["relay_protocol"])
	}

	if !common.Contains(testSSHClientVersions, fields["ssh_client_version"].(string)) {
		return fmt.Errorf("unexpected relay_protocol '%s'", fields["ssh_client_version"])
	}

	if protocol.TunnelProtocolUsesObfuscatedSSH(runConfig.tunnelProtocol) {

		for _, name := range []string{
			"padding",
			"pad_response",
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}
	}

	if protocol.TunnelProtocolUsesMeek(runConfig.tunnelProtocol) {

		for _, name := range []string{
			"user_agent",
			"meek_transformed_host_name",
			tactics.APPLIED_TACTICS_TAG_PARAMETER_NAME,
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}

		if !common.Contains(testUserAgents, fields["user_agent"].(string)) {
			return fmt.Errorf("unexpected user_agent '%s'", fields["user_agent"])
		}
	}

	if protocol.TunnelProtocolUsesMeekHTTP(runConfig.tunnelProtocol) {

		for _, name := range []string{
			"meek_host_header",
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}

		for _, name := range []string{
			"meek_dial_ip_address",
			"meek_resolved_ip_address",
		} {
			if fields[name] != nil {
				return fmt.Errorf("unexpected field '%s'", name)
			}
		}
	}

	if protocol.TunnelProtocolUsesMeekHTTPS(runConfig.tunnelProtocol) {

		for _, name := range []string{
			"tls_profile",
			"meek_sni_server_name",
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}

		for _, name := range []string{
			"meek_dial_ip_address",
			"meek_resolved_ip_address",
			"meek_host_header",
		} {
			if fields[name] != nil {
				return fmt.Errorf("unexpected field '%s'", name)
			}
		}

		if !common.Contains(protocol.SupportedTLSProfiles, fields["tls_profile"].(string)) {
			return fmt.Errorf("unexpected tls_profile '%s'", fields["tls_profile"])
		}

	}

	if protocol.TunnelProtocolUsesQUIC(runConfig.tunnelProtocol) {

		for _, name := range []string{
			"quic_version",
			"quic_dial_sni_address",
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}

		if !common.Contains(protocol.SupportedQUICVersions, fields["quic_version"].(string)) {
			return fmt.Errorf("unexpected quic_version '%s'", fields["quic_version"])
		}
	}

	if runConfig.forceFragmenting {

		for _, name := range []string{
			"upstream_bytes_fragmented",
			"upstream_min_bytes_written",
			"upstream_max_bytes_written",
			"upstream_min_delayed",
			"upstream_max_delayed",
		} {
			if fields[name] == nil || fmt.Sprintf("%s", fields[name]) == "" {
				return fmt.Errorf("missing expected field '%s'", name)
			}
		}
	}

	return nil
}

func makeTunneledWebRequest(
	t *testing.T,
	localHTTPProxyPort int,
	requestURL, expectedResponseBody string) error {

	roundTripTimeout := 30 * time.Second

	proxyUrl, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localHTTPProxyPort))
	if err != nil {
		return fmt.Errorf("error initializing proxied HTTP request: %s", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		},
		Timeout: roundTripTimeout,
	}

	response, err := httpClient.Get(requestURL)
	if err != nil {
		return fmt.Errorf("error sending proxied HTTP request: %s", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading proxied HTTP response: %s", err)
	}
	response.Body.Close()

	if string(body) != expectedResponseBody {
		return fmt.Errorf("unexpected proxied HTTP response")
	}

	return nil
}

func makeTunneledNTPRequest(t *testing.T, localSOCKSProxyPort int, udpgwServerAddress string) error {

	timeout := 20 * time.Second
	var err error

	for _, testHostname := range []string{"time.google.com", "time.nist.gov", "pool.ntp.org"} {
		err = makeTunneledNTPRequestAttempt(t, testHostname, timeout, localSOCKSProxyPort, udpgwServerAddress)
		if err == nil {
			break
		}
		t.Logf("makeTunneledNTPRequestAttempt failed: %s", err)
	}

	return err
}

var nextUDPProxyPort = 7300

func makeTunneledNTPRequestAttempt(
	t *testing.T, testHostname string, timeout time.Duration, localSOCKSProxyPort int, udpgwServerAddress string) error {

	nextUDPProxyPort++
	localUDPProxyAddress, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", nextUDPProxyPort))
	if err != nil {
		return fmt.Errorf("ResolveUDPAddr failed: %s", err)
	}

	// Note: this proxy is intended for this test only -- it only accepts a single connection,
	// handles it, and then terminates.

	localUDPProxy := func(destinationIP net.IP, destinationPort uint16, waitGroup *sync.WaitGroup) {

		if waitGroup != nil {
			defer waitGroup.Done()
		}

		destination := net.JoinHostPort(destinationIP.String(), strconv.Itoa(int(destinationPort)))

		serverUDPConn, err := net.ListenUDP("udp", localUDPProxyAddress)
		if err != nil {
			t.Logf("ListenUDP for %s failed: %s", destination, err)
			return
		}
		defer serverUDPConn.Close()

		udpgwPreambleSize := 11 // see writeUdpgwPreamble
		buffer := make([]byte, udpgwProtocolMaxMessageSize)
		packetSize, clientAddr, err := serverUDPConn.ReadFromUDP(
			buffer[udpgwPreambleSize:])
		if err != nil {
			t.Logf("serverUDPConn.Read for %s failed: %s", destination, err)
			return
		}

		socksProxyAddress := fmt.Sprintf("127.0.0.1:%d", localSOCKSProxyPort)

		dialer, err := proxy.SOCKS5("tcp", socksProxyAddress, nil, proxy.Direct)
		if err != nil {
			t.Logf("proxy.SOCKS5 for %s failed: %s", destination, err)
			return
		}

		socksTCPConn, err := dialer.Dial("tcp", udpgwServerAddress)
		if err != nil {
			t.Logf("dialer.Dial for %s failed: %s", destination, err)
			return
		}
		defer socksTCPConn.Close()

		flags := uint8(0)
		if destinationPort == 53 {
			flags = udpgwProtocolFlagDNS
		}

		err = writeUdpgwPreamble(
			udpgwPreambleSize,
			flags,
			0,
			destinationIP,
			destinationPort,
			uint16(packetSize),
			buffer)
		if err != nil {
			t.Logf("writeUdpgwPreamble for %s failed: %s", destination, err)
			return
		}

		_, err = socksTCPConn.Write(buffer[0 : udpgwPreambleSize+packetSize])
		if err != nil {
			t.Logf("socksTCPConn.Write for %s failed: %s", destination, err)
			return
		}

		udpgwProtocolMessage, err := readUdpgwMessage(socksTCPConn, buffer)
		if err != nil {
			t.Logf("readUdpgwMessage for %s failed: %s", destination, err)
			return
		}

		_, err = serverUDPConn.WriteToUDP(udpgwProtocolMessage.packet, clientAddr)
		if err != nil {
			t.Logf("serverUDPConn.Write for %s failed: %s", destination, err)
			return
		}
	}

	// Tunneled DNS request

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(1)
	go localUDPProxy(
		net.IP(make([]byte, 4)), // ignored due to transparent DNS forwarding
		53,
		waitGroup)
	// TODO: properly synchronize with local UDP proxy startup
	time.Sleep(1 * time.Second)

	clientUDPConn, err := net.DialUDP("udp", nil, localUDPProxyAddress)
	if err != nil {
		return fmt.Errorf("DialUDP failed: %s", err)
	}

	clientUDPConn.SetReadDeadline(time.Now().Add(timeout))
	clientUDPConn.SetWriteDeadline(time.Now().Add(timeout))

	addrs, _, err := psiphon.ResolveIP(testHostname, clientUDPConn)

	clientUDPConn.Close()

	if err == nil && (len(addrs) == 0 || len(addrs[0]) < 4) {
		err = errors.New("no address")
	}
	if err != nil {
		return fmt.Errorf("ResolveIP failed: %s", err)
	}

	waitGroup.Wait()

	// Tunneled NTP request

	waitGroup = new(sync.WaitGroup)
	waitGroup.Add(1)
	go localUDPProxy(
		addrs[0][len(addrs[0])-4:],
		123,
		waitGroup)
	// TODO: properly synchronize with local UDP proxy startup
	time.Sleep(1 * time.Second)

	clientUDPConn, err = net.DialUDP("udp", nil, localUDPProxyAddress)
	if err != nil {
		return fmt.Errorf("DialUDP failed: %s", err)
	}

	clientUDPConn.SetReadDeadline(time.Now().Add(timeout))
	clientUDPConn.SetWriteDeadline(time.Now().Add(timeout))

	// NTP protocol code from: https://groups.google.com/d/msg/golang-nuts/FlcdMU5fkLQ/CAeoD9eqm-IJ

	ntpData := make([]byte, 48)
	ntpData[0] = 3<<3 | 3

	_, err = clientUDPConn.Write(ntpData)
	if err != nil {
		clientUDPConn.Close()
		return fmt.Errorf("NTP Write failed: %s", err)
	}

	_, err = clientUDPConn.Read(ntpData)
	if err != nil {
		clientUDPConn.Close()
		return fmt.Errorf("NTP Read failed: %s", err)
	}

	clientUDPConn.Close()

	var sec, frac uint64
	sec = uint64(ntpData[43]) | uint64(ntpData[42])<<8 | uint64(ntpData[41])<<16 | uint64(ntpData[40])<<24
	frac = uint64(ntpData[47]) | uint64(ntpData[46])<<8 | uint64(ntpData[45])<<16 | uint64(ntpData[44])<<24

	nsec := sec * 1e9
	nsec += (frac * 1e9) >> 32

	ntpNow := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(nsec)).Local()

	now := time.Now()

	diff := ntpNow.Sub(now)
	if diff < 0 {
		diff = -diff
	}

	if diff > 1*time.Minute {
		return fmt.Errorf("Unexpected NTP time: %s; local time: %s", ntpNow, now)
	}

	waitGroup.Wait()

	return nil
}

func pavePsinetDatabaseFile(
	t *testing.T, useDefaultSponsorID bool, psinetFilename string) (string, string) {

	sponsorID := prng.HexString(8)

	fakeDomain := prng.HexString(4)
	fakePath := prng.HexString(4)
	expectedHomepageURL := fmt.Sprintf("https://%s.com/%s", fakeDomain, fakePath)

	psinetJSONFormat := `
    {
        "default_sponsor_id" : "%s",
        "sponsors": {
            "%s": {
                "home_pages": {
                    "None": [
                        {
                            "region": null,
                            "url": "%s"
                        }
                    ]
                }
            }
        }
    }
	`

	defaultSponsorID := ""
	if useDefaultSponsorID {
		defaultSponsorID = sponsorID
	}

	psinetJSON := fmt.Sprintf(
		psinetJSONFormat, defaultSponsorID, sponsorID, expectedHomepageURL)

	err := ioutil.WriteFile(psinetFilename, []byte(psinetJSON), 0600)
	if err != nil {
		t.Fatalf("error paving psinet database file: %s", err)
	}

	return sponsorID, expectedHomepageURL
}

func paveTrafficRulesFile(
	t *testing.T, trafficRulesFilename, propagationChannelID, accessType string,
	requireAuthorization, deny bool,
	livenessTestSize int) {

	allowTCPPorts := fmt.Sprintf("%d", mockWebServerPort)
	allowUDPPorts := "53, 123"

	if deny {
		allowTCPPorts = "0"
		allowUDPPorts = "0"
	}

	authorizationFilterFormat := `,
                    "AuthorizedAccessTypes" : ["%s"]
	`

	authorizationFilter := ""
	if requireAuthorization {
		authorizationFilter = fmt.Sprintf(authorizationFilterFormat, accessType)
	}

	trafficRulesJSONFormat := `
    {
        "DefaultRules" :  {
            "RateLimits" : {
                "ReadBytesPerSecond": 16384,
                "WriteBytesPerSecond": 16384,
                "ReadUnthrottledBytes": %d,
                "WriteUnthrottledBytes": %d
            },
            "AllowTCPPorts" : [0],
            "AllowUDPPorts" : [0],
            "MeekRateLimiterHistorySize" : 10,
            "MeekRateLimiterThresholdSeconds" : 1,
            "MeekRateLimiterGarbageCollectionTriggerCount" : 1,
            "MeekRateLimiterReapHistoryFrequencySeconds" : 1,
            "MeekRateLimiterRegions" : []
        },
        "FilteredRules" : [
            {
                "Filter" : {
                    "HandshakeParameters" : {
                        "propagation_channel_id" : ["%s"]
                    }%s
                },
                "Rules" : {
                    "RateLimits" : {
                        "ReadBytesPerSecond": 2097152,
                        "WriteBytesPerSecond": 2097152
                    },
                    "AllowTCPPorts" : [%s],
                    "AllowUDPPorts" : [%s]
                }
            }
        ]
    }
    `

	trafficRulesJSON := fmt.Sprintf(
		trafficRulesJSONFormat,
		livenessTestSize, livenessTestSize,
		propagationChannelID, authorizationFilter, allowTCPPorts, allowUDPPorts)

	err := ioutil.WriteFile(trafficRulesFilename, []byte(trafficRulesJSON), 0600)
	if err != nil {
		t.Fatalf("error paving traffic rules file: %s", err)
	}
}

var expectedNumSLOKs = 3

func paveOSLConfigFile(t *testing.T, oslConfigFilename string) string {

	oslConfigJSONFormat := `
    {
      "Schemes" : [
        {
          "Epoch" : "%s",
          "Regions" : [],
          "PropagationChannelIDs" : ["%s"],
          "MasterKey" : "wFuSbqU/pJ/35vRmoM8T9ys1PgDa8uzJps1Y+FNKa5U=",
          "SeedSpecs" : [
            {
              "ID" : "IXHWfVgWFkEKvgqsjmnJuN3FpaGuCzQMETya+DSQvsk=",
              "UpstreamSubnets" : ["0.0.0.0/0"],
              "Targets" :
              {
                  "BytesRead" : 1,
                  "BytesWritten" : 1,
                  "PortForwardDurationNanoseconds" : 1
              }
            },
            {
              "ID" : "qvpIcORLE2Pi5TZmqRtVkEp+OKov0MhfsYPLNV7FYtI=",
              "UpstreamSubnets" : ["0.0.0.0/0"],
              "Targets" :
              {
                  "BytesRead" : 1,
                  "BytesWritten" : 1,
                  "PortForwardDurationNanoseconds" : 1
              }
            }
          ],
          "SeedSpecThreshold" : 2,
          "SeedPeriodNanoseconds" : 2592000000000000,
          "SeedPeriodKeySplits": [
            {
              "Total": 2,
              "Threshold": 2
            }
          ]
        },
        {
          "Epoch" : "%s",
          "Regions" : [],
          "PropagationChannelIDs" : ["%s"],
          "MasterKey" : "HDc/mvd7e+lKDJD0fMpJW66YJ/VW4iqDRjeclEsMnro=",
          "SeedSpecs" : [
            {
              "ID" : "/M0vsT0IjzmI0MvTI9IYe8OVyeQGeaPZN2xGxfLw/UQ=",
              "UpstreamSubnets" : ["0.0.0.0/0"],
              "Targets" :
              {
                  "BytesRead" : 1,
                  "BytesWritten" : 1,
                  "PortForwardDurationNanoseconds" : 1
              }
            }
          ],
          "SeedSpecThreshold" : 1,
          "SeedPeriodNanoseconds" : 2592000000000000,
          "SeedPeriodKeySplits": [
            {
              "Total": 1,
              "Threshold": 1
            }
          ]
        }
      ]
    }
    `

	propagationChannelID := prng.HexString(8)

	now := time.Now().UTC()
	epoch := now.Truncate(720 * time.Hour)
	epochStr := epoch.Format(time.RFC3339Nano)

	oslConfigJSON := fmt.Sprintf(
		oslConfigJSONFormat,
		epochStr, propagationChannelID,
		epochStr, propagationChannelID)

	err := ioutil.WriteFile(oslConfigFilename, []byte(oslConfigJSON), 0600)
	if err != nil {
		t.Fatalf("error paving osl config file: %s", err)
	}

	return propagationChannelID
}

func paveTacticsConfigFile(
	t *testing.T, tacticsConfigFilename string,
	tacticsRequestPublicKey, tacticsRequestPrivateKey, tacticsRequestObfuscatedKey string,
	tunnelProtocol string,
	propagationChannelID string,
	livenessTestSize int) {

	// Setting LimitTunnelProtocols passively exercises the
	// server-side LimitTunnelProtocols enforcement.

	tacticsConfigJSONFormat := `
    {
      "RequestPublicKey" : "%s",
      "RequestPrivateKey" : "%s",
      "RequestObfuscatedKey" : "%s",
      "EnforceServerSide" : true,
      "DefaultTactics" : {
        "TTL" : "60s",
        "Probability" : 1.0,
        "Parameters" : {
          "LimitTunnelProtocols" : ["%s"],
          "FragmentorLimitProtocols" : ["%s"],
          "FragmentorProbability" : 1.0,
          "FragmentorMinTotalBytes" : 1000,
          "FragmentorMaxTotalBytes" : 2000,
          "FragmentorMinWriteBytes" : 1,
          "FragmentorMaxWriteBytes" : 100,
          "FragmentorMinDelay" : "1ms",
          "FragmentorMaxDelay" : "10ms",
          "FragmentorDownstreamLimitProtocols" : ["%s"],
          "FragmentorDownstreamProbability" : 1.0,
          "FragmentorDownstreamMinTotalBytes" : 1000,
          "FragmentorDownstreamMaxTotalBytes" : 2000,
          "FragmentorDownstreamMinWriteBytes" : 1,
          "FragmentorDownstreamMaxWriteBytes" : 100,
          "FragmentorDownstreamMinDelay" : "1ms",
          "FragmentorDownstreamMaxDelay" : "10ms",
          "LivenessTestMinUpstreamBytes" : %d,
          "LivenessTestMaxUpstreamBytes" : %d,
          "LivenessTestMinDownstreamBytes" : %d,
          "LivenessTestMaxDownstreamBytes" : %d
        }
      },
      "FilteredTactics" : [
        {
          "Filter" : {
            "APIParameters" : {"propagation_channel_id" : ["%s"]},
            "SpeedTestRTTMilliseconds" : {
              "Aggregation" : "Median",
              "AtLeast" : 1
            }
          },
          "Tactics" : {
            "Parameters" : {
              "TunnelConnectTimeout" : "20s",
              "TunnelRateLimits" : {"WriteBytesPerSecond": 1000000},
              "TransformHostNameProbability" : 1.0,
              "PickUserAgentProbability" : 1.0
            }
          }
        }
      ]
    }
    `

	tacticsConfigJSON := fmt.Sprintf(
		tacticsConfigJSONFormat,
		tacticsRequestPublicKey, tacticsRequestPrivateKey, tacticsRequestObfuscatedKey,
		tunnelProtocol,
		tunnelProtocol,
		tunnelProtocol,
		livenessTestSize, livenessTestSize, livenessTestSize, livenessTestSize,
		propagationChannelID)

	err := ioutil.WriteFile(tacticsConfigFilename, []byte(tacticsConfigJSON), 0600)
	if err != nil {
		t.Fatalf("error paving tactics config file: %s", err)
	}
}

func paveBlocklistFile(t *testing.T, blocklistFilename string) {

	blocklistContent := "255.255.255.255,test-source,test-subject\n"

	err := ioutil.WriteFile(blocklistFilename, []byte(blocklistContent), 0600)
	if err != nil {
		t.Fatalf("error paving blocklist file: %s", err)
	}
}

func sendNotificationReceived(c chan<- struct{}) {
	select {
	case c <- *new(struct{}):
	default:
	}
}

func waitOnNotification(t *testing.T, c, timeoutSignal <-chan struct{}, timeoutMessage string) {
	select {
	case <-c:
	case <-timeoutSignal:
		t.Fatalf(timeoutMessage)
	}
}
