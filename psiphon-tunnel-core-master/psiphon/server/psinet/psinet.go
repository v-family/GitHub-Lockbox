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

// Package psinet implements psinet database services. The psinet database is a
// JSON-format file containing information about the Psiphon network, including
// sponsors, home pages, stats regexes, available upgrades, and other servers for
// discovery. This package also implements the Psiphon discovery algorithm.
package psinet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common"
)

// Database serves Psiphon API data requests. It's safe for
// concurrent usage. The Reload function supports hot reloading
// of Psiphon network data while the server is running.
type Database struct {
	common.ReloadableFile

	Hosts            map[string]Host            `json:"hosts"`
	Servers          []Server                   `json:"servers"`
	Sponsors         map[string]Sponsor         `json:"sponsors"`
	Versions         map[string][]ClientVersion `json:"client_versions"`
	DefaultSponsorID string                     `json:"default_sponsor_id"`
}

type Host struct {
	DatacenterName                string `json:"datacenter_name"`
	Id                            string `json:"id"`
	IpAddress                     string `json:"ip_address"`
	IsTCS                         bool   `json:"is_TCS"`
	MeekCookieEncryptionPublicKey string `json:"meek_cookie_encryption_public_key"`
	MeekServerObfuscatedKey       string `json:"meek_server_obfuscated_key"`
	MeekServerPort                int    `json:"meek_server_port"`
	TacticsRequestPublicKey       string `json:"tactics_request_public_key"`
	TacticsRequestObfuscatedKey   string `json:"tactics_request_obfuscated_key"`
	Region                        string `json:"region"`
}

type Server struct {
	AlternateSshObfuscatedPorts []string        `json:"alternate_ssh_obfuscated_ports"`
	Capabilities                map[string]bool `json:"capabilities"`
	DiscoveryDateRange          []string        `json:"discovery_date_range"`
	EgressIpAddress             string          `json:"egress_ip_address"`
	HostId                      string          `json:"host_id"`
	Id                          string          `json:"id"`
	InternalIpAddress           string          `json:"internal_ip_address"`
	IpAddress                   string          `json:"ip_address"`
	IsEmbedded                  bool            `json:"is_embedded"`
	IsPermanent                 bool            `json:"is_permanent"`
	PropogationChannelId        string          `json:"propagation_channel_id"`
	SshHostKey                  string          `json:"ssh_host_key"`
	SshObfuscatedKey            string          `json:"ssh_obfuscated_key"`
	SshObfuscatedPort           int             `json:"ssh_obfuscated_port"`
	SshObfuscatedQUICPort       int             `json:"ssh_obfuscated_quic_port"`
	SshObfuscatedTapdancePort   int             `json:"ssh_obfuscated_tapdance_port"`
	SshPassword                 string          `json:"ssh_password"`
	SshPort                     string          `json:"ssh_port"`
	SshUsername                 string          `json:"ssh_username"`
	WebServerCertificate        string          `json:"web_server_certificate"`
	WebServerPort               string          `json:"web_server_port"`
	WebServerSecret             string          `json:"web_server_secret"`
	ConfigurationVersion        int             `json:"configuration_version"`
}

type Sponsor struct {
	Banner              string
	HomePages           map[string][]HomePage `json:"home_pages"`
	HttpsRequestRegexes []HttpsRequestRegex   `json:"https_request_regexes"`
	Id                  string                `json:"id"`
	MobileHomePages     map[string][]HomePage `json:"mobile_home_pages"`
	Name                string                `json:"name"`
	PageViewRegexes     []PageViewRegex       `json:"page_view_regexes"`
	WebsiteBanner       string                `json:"website_banner"`
	WebsiteBannerLink   string                `json:"website_banner_link"`
}

type ClientVersion struct {
	Version string `json:"version"`
}

type HomePage struct {
	Region string `json:"region"`
	Url    string `json:"url"`
}

type HttpsRequestRegex struct {
	Regex   string `json:"regex"`
	Replace string `json:"replace"`
}

type MobileHomePage struct {
	Region string `json:"region"`
	Url    string `json:"url"`
}

type PageViewRegex struct {
	Regex   string `json:"regex"`
	Replace string `json:"replace"`
}

// NewDatabase initializes a Database, calling Reload on the specified
// filename.
func NewDatabase(filename string) (*Database, error) {

	database := &Database{}

	database.ReloadableFile = common.NewReloadableFile(
		filename,
		true,
		func(fileContent []byte) error {
			var newDatabase Database
			err := json.Unmarshal(fileContent, &newDatabase)
			if err != nil {
				return common.ContextError(err)
			}
			// Note: an unmarshal directly into &database would fail
			// to reset to zero value fields not present in the JSON.
			database.Hosts = newDatabase.Hosts
			database.Servers = newDatabase.Servers
			database.Sponsors = newDatabase.Sponsors
			database.Versions = newDatabase.Versions
			database.DefaultSponsorID = newDatabase.DefaultSponsorID

			return nil
		})

	_, err := database.Reload()
	if err != nil {
		return nil, common.ContextError(err)
	}

	return database, nil
}

// GetRandomizedHomepages returns a randomly ordered list of home pages
// for the specified sponsor, region, and platform.
func (db *Database) GetRandomizedHomepages(sponsorID, clientRegion string, isMobilePlatform bool) []string {
	homepages := db.GetHomepages(sponsorID, clientRegion, isMobilePlatform)
	if len(homepages) > 1 {
		shuffledHomepages := make([]string, len(homepages))
		perm := rand.Perm(len(homepages))
		for i, v := range perm {
			shuffledHomepages[v] = homepages[i]
		}
		return shuffledHomepages
	}
	return homepages
}

// GetHomepages returns a list of home pages for the specified sponsor,
// region, and platform.
func (db *Database) GetHomepages(sponsorID, clientRegion string, isMobilePlatform bool) []string {
	db.ReloadableFile.RLock()
	defer db.ReloadableFile.RUnlock()

	sponsorHomePages := make([]string, 0)

	// Sponsor id does not exist: fail gracefully
	sponsor, ok := db.Sponsors[sponsorID]
	if !ok {
		sponsor, ok = db.Sponsors[db.DefaultSponsorID]
		if !ok {
			return sponsorHomePages
		}
	}

	homePages := sponsor.HomePages

	if isMobilePlatform {
		if len(sponsor.MobileHomePages) > 0 {
			homePages = sponsor.MobileHomePages
		}
	}

	// Case: lookup succeeded and corresponding homepages found for region
	homePagesByRegion, ok := homePages[clientRegion]
	if ok {
		for _, homePage := range homePagesByRegion {
			sponsorHomePages = append(sponsorHomePages, strings.Replace(homePage.Url, "client_region=XX", "client_region="+clientRegion, 1))
		}
	}

	// Case: lookup failed or no corresponding homepages found for region --> use default
	if len(sponsorHomePages) == 0 {
		defaultHomePages, ok := homePages["None"]
		if ok {
			for _, homePage := range defaultHomePages {
				// client_region query parameter substitution
				sponsorHomePages = append(sponsorHomePages, strings.Replace(homePage.Url, "client_region=XX", "client_region="+clientRegion, 1))
			}
		}
	}

	return sponsorHomePages
}

// GetUpgradeClientVersion returns a new client version when an upgrade is
// indicated for the specified client current version. The result is "" when
// no upgrade is available. Caller should normalize clientPlatform.
func (db *Database) GetUpgradeClientVersion(clientVersion, clientPlatform string) string {
	db.ReloadableFile.RLock()
	defer db.ReloadableFile.RUnlock()

	// Check lastest version number against client version number

	clientVersions, ok := db.Versions[clientPlatform]
	if !ok {
		return ""
	}

	if len(clientVersions) == 0 {
		return ""
	}

	// NOTE: Assumes versions list is in ascending version order
	lastVersion := clientVersions[len(clientVersions)-1].Version

	lastVersionInt, err := strconv.Atoi(lastVersion)
	if err != nil {
		return ""
	}
	clientVersionInt, err := strconv.Atoi(clientVersion)
	if err != nil {
		return ""
	}

	// Return latest version if upgrade needed
	if lastVersionInt > clientVersionInt {
		return lastVersion
	}

	return ""
}

// GetHttpsRequestRegexes returns bytes transferred stats regexes for the
// specified sponsor.
func (db *Database) GetHttpsRequestRegexes(sponsorID string) []map[string]string {
	db.ReloadableFile.RLock()
	defer db.ReloadableFile.RUnlock()

	regexes := make([]map[string]string, 0)

	sponsor, ok := db.Sponsors[sponsorID]
	if !ok {
		sponsor, _ = db.Sponsors[db.DefaultSponsorID]
	}

	// If neither sponsorID or DefaultSponsorID were found, sponsor will be the
	// zero value of the map, an empty Sponsor struct.
	for _, sponsorRegex := range sponsor.HttpsRequestRegexes {
		regex := make(map[string]string)
		regex["replace"] = sponsorRegex.Replace
		regex["regex"] = sponsorRegex.Regex
		regexes = append(regexes, regex)
	}

	return regexes
}

// DiscoverServers selects new encoded server entries to be "discovered" by
// the client, using the discoveryValue -- a function of the client's IP
// address -- as the input into the discovery algorithm.
// The server list (db.Servers) loaded from JSON is stored as an array instead of
// a map to ensure servers are discovered deterministically. Each iteration over a
// map in go is seeded with a random value which causes non-deterministic ordering.
func (db *Database) DiscoverServers(discoveryValue int) []string {
	db.ReloadableFile.RLock()
	defer db.ReloadableFile.RUnlock()

	var servers []Server

	discoveryDate := time.Now().UTC()
	candidateServers := make([]Server, 0)

	for _, server := range db.Servers {
		var start time.Time
		var end time.Time
		var err error

		// All servers that are discoverable on this day are eligible for discovery
		if len(server.DiscoveryDateRange) != 0 {
			start, err = time.Parse("2006-01-02T15:04:05", server.DiscoveryDateRange[0])
			if err != nil {
				continue
			}
			end, err = time.Parse("2006-01-02T15:04:05", server.DiscoveryDateRange[1])
			if err != nil {
				continue
			}
			if discoveryDate.After(start) && discoveryDate.Before(end) {
				candidateServers = append(candidateServers, server)
			}
		}
	}

	timeInSeconds := int(discoveryDate.Unix())
	servers = selectServers(candidateServers, timeInSeconds, discoveryValue)

	encodedServerEntries := make([]string, 0)

	for _, server := range servers {
		encodedServerEntries = append(encodedServerEntries, db.getEncodedServerEntry(server))
	}

	return encodedServerEntries
}

// Combine client IP address and time-of-day strategies to give out different
// discovery servers to different clients. The aim is to achieve defense against
// enumerability. We also want to achieve a degree of load balancing clients
// and these strategies are expected to have reasonably random distribution,
// even for a cluster of users coming from the same network.
//
// We only select one server: multiple results makes enumeration easier; the
// strategies have a built-in load balancing effect; and date range discoverability
// means a client will actually learn more servers later even if they happen to
// always pick the same result at this point.
//
// This is a blended strategy: as long as there are enough servers to pick from,
// both aspects determine which server is selected. IP address is given the
// priority: if there are only a couple of servers, for example, IP address alone
// determines the outcome.
func selectServers(servers []Server, timeInSeconds, discoveryValue int) []Server {
	TIME_GRANULARITY := 3600

	if len(servers) == 0 {
		return nil
	}

	// Time truncated to an hour
	timeStrategyValue := timeInSeconds / TIME_GRANULARITY

	// Divide servers into buckets. The bucket count is chosen such that the number
	// of buckets and the number of items in each bucket are close (using sqrt).
	// IP address selects the bucket, time selects the item in the bucket.

	// NOTE: this code assumes that the range of possible timeStrategyValues
	// and discoveryValues are sufficient to index to all bucket items.

	bucketCount := calculateBucketCount(len(servers))

	buckets := bucketizeServerList(servers, bucketCount)

	if len(buckets) == 0 {
		return nil
	}

	bucket := buckets[discoveryValue%len(buckets)]

	if len(bucket) == 0 {
		return nil
	}

	server := bucket[timeStrategyValue%len(bucket)]

	serverList := make([]Server, 1)
	serverList[0] = server

	return serverList
}

// Number of buckets such that first strategy picks among about the same number
// of choices as the second strategy. Gives an edge to the "outer" strategy.
func calculateBucketCount(length int) int {
	return int(math.Ceil(math.Sqrt(float64(length))))
}

// bucketizeServerList creates nearly equal sized slices of the input list.
func bucketizeServerList(servers []Server, bucketCount int) [][]Server {

	// This code creates the same partitions as legacy servers:
	// https://bitbucket.org/psiphon/psiphon-circumvention-system/src/03bc1a7e51e7c85a816e370bb3a6c755fd9c6fee/Automation/psi_ops_discovery.py
	//
	// Both use the same algorithm from:
	// http://stackoverflow.com/questions/2659900/python-slicing-a-list-into-n-nearly-equal-length-partitions

	// TODO: this partition is constant for fixed Database content, so it could
	// be done once and cached in the Database ReloadableFile reloadAction.

	buckets := make([][]Server, bucketCount)

	division := float64(len(servers)) / float64(bucketCount)

	for i := 0; i < bucketCount; i++ {
		start := int((division * float64(i)) + 0.5)
		end := int((division * (float64(i) + 1)) + 0.5)
		buckets[i] = servers[start:end]
	}

	return buckets
}

// Return hex encoded server entry string for comsumption by client.
// Newer clients ignore the legacy fields and only utilize the extended (new) config.
func (db *Database) getEncodedServerEntry(server Server) string {

	host, hostExists := db.Hosts[server.HostId]
	if !hostExists {
		return ""
	}

	// TCS web server certificate has PEM headers and newlines, so strip those now
	// for legacy format compatibility
	webServerCertificate := server.WebServerCertificate
	if host.IsTCS {
		splitCert := strings.Split(server.WebServerCertificate, "\n")
		if len(splitCert) <= 2 {
			webServerCertificate = ""
		} else {
			webServerCertificate = strings.Join(splitCert[1:len(splitCert)-2], "")
		}
	}

	// Double-check that we're not giving our blank server credentials
	if len(server.IpAddress) <= 1 || len(server.WebServerPort) <= 1 || len(server.WebServerSecret) <= 1 || len(webServerCertificate) <= 1 {
		return ""
	}

	// Extended (new) entry fields are in a JSON string
	var extendedConfig struct {
		IpAddress                     string   `json:"ipAddress"`
		WebServerPort                 string   `json:"webServerPort"` // not an int
		WebServerSecret               string   `json:"webServerSecret"`
		WebServerCertificate          string   `json:"webServerCertificate"`
		SshPort                       int      `json:"sshPort"`
		SshUsername                   string   `json:"sshUsername"`
		SshPassword                   string   `json:"sshPassword"`
		SshHostKey                    string   `json:"sshHostKey"`
		SshObfuscatedPort             int      `json:"sshObfuscatedPort"`
		SshObfuscatedQUICPort         int      `json:"sshObfuscatedQUICPort"`
		SshObfuscatedTapdancePort     int      `json:"sshObfuscatedTapdancePort"`
		SshObfuscatedKey              string   `json:"sshObfuscatedKey"`
		Capabilities                  []string `json:"capabilities"`
		Region                        string   `json:"region"`
		MeekServerPort                int      `json:"meekServerPort"`
		MeekCookieEncryptionPublicKey string   `json:"meekCookieEncryptionPublicKey"`
		MeekObfuscatedKey             string   `json:"meekObfuscatedKey"`
		TacticsRequestPublicKey       string   `json:"tacticsRequestPublicKey"`
		TacticsRequestObfuscatedKey   string   `json:"tacticsRequestObfuscatedKey"`
		ConfigurationVersion          int      `json:"configurationVersion"`
	}

	// NOTE: also putting original values in extended config for easier parsing by new clients
	extendedConfig.IpAddress = server.IpAddress
	extendedConfig.WebServerPort = server.WebServerPort
	extendedConfig.WebServerSecret = server.WebServerSecret
	extendedConfig.WebServerCertificate = webServerCertificate

	sshPort, err := strconv.Atoi(server.SshPort)
	if err != nil {
		extendedConfig.SshPort = 0
	} else {
		extendedConfig.SshPort = sshPort
	}

	extendedConfig.SshUsername = server.SshUsername
	extendedConfig.SshPassword = server.SshPassword

	sshHostKeyType, sshHostKey := parseSshKeyString(server.SshHostKey)

	if strings.Compare(sshHostKeyType, "ssh-rsa") == 0 {
		extendedConfig.SshHostKey = sshHostKey
	} else {
		extendedConfig.SshHostKey = ""
	}

	extendedConfig.SshObfuscatedPort = server.SshObfuscatedPort
	// Use the latest alternate port unless tunneling through meek
	if len(server.AlternateSshObfuscatedPorts) > 0 && !server.Capabilities["UNFRONTED-MEEK"] {
		port, err := strconv.Atoi(server.AlternateSshObfuscatedPorts[len(server.AlternateSshObfuscatedPorts)-1])
		if err == nil {
			extendedConfig.SshObfuscatedPort = port
		}
	}

	extendedConfig.SshObfuscatedQUICPort = server.SshObfuscatedQUICPort
	extendedConfig.SshObfuscatedTapdancePort = server.SshObfuscatedTapdancePort

	extendedConfig.SshObfuscatedKey = server.SshObfuscatedKey
	extendedConfig.Region = host.Region
	extendedConfig.MeekCookieEncryptionPublicKey = host.MeekCookieEncryptionPublicKey
	extendedConfig.MeekServerPort = host.MeekServerPort
	extendedConfig.MeekObfuscatedKey = host.MeekServerObfuscatedKey
	extendedConfig.TacticsRequestPublicKey = host.TacticsRequestPublicKey
	extendedConfig.TacticsRequestObfuscatedKey = host.TacticsRequestObfuscatedKey

	serverCapabilities := make(map[string]bool, 0)
	for capability, enabled := range server.Capabilities {
		serverCapabilities[capability] = enabled
	}

	if serverCapabilities["UNFRONTED-MEEK"] && host.MeekServerPort == 443 {
		serverCapabilities["UNFRONTED-MEEK"] = false
		serverCapabilities["UNFRONTED-MEEK-HTTPS"] = true
	}

	for capability, enabled := range serverCapabilities {
		if enabled == true {
			extendedConfig.Capabilities = append(extendedConfig.Capabilities, capability)
		}
	}

	extendedConfig.ConfigurationVersion = server.ConfigurationVersion

	jsonDump, err := json.Marshal(extendedConfig)
	if err != nil {
		return ""
	}

	// Legacy format + extended (new) config
	prefixString := fmt.Sprintf("%s %s %s %s ", server.IpAddress, server.WebServerPort, server.WebServerSecret, webServerCertificate)

	return hex.EncodeToString(append([]byte(prefixString)[:], []byte(jsonDump)[:]...))
}

// Parse string of format "ssh-key-type ssh-key".
func parseSshKeyString(sshKeyString string) (keyType string, key string) {
	sshKeyArr := strings.Split(sshKeyString, " ")
	if len(sshKeyArr) != 2 {
		return "", ""
	}

	return sshKeyArr[0], sshKeyArr[1]
}
