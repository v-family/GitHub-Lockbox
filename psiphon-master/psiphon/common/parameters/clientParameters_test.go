/*
 * Copyright (c) 2018, Psiphon Inc.
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

package parameters

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common"
	"github.com/Psiphon-Labs/psiphon-tunnel-core/psiphon/common/protocol"
)

func TestGetDefaultParameters(t *testing.T) {

	p, err := NewClientParameters(nil)
	if err != nil {
		t.Fatalf("NewClientParameters failed: %s", err)
	}

	for name, defaults := range defaultClientParameters {
		switch v := defaults.value.(type) {
		case string:
			g := p.Get().String(name)
			if v != g {
				t.Fatalf("String returned %+v expected %+v", v, g)
			}
		case int:
			g := p.Get().Int(name)
			if v != g {
				t.Fatalf("Int returned %+v expected %+v", v, g)
			}
		case float64:
			g := p.Get().Float(name)
			if v != g {
				t.Fatalf("Float returned %+v expected %+v", v, g)
			}
		case bool:
			g := p.Get().Bool(name)
			if v != g {
				t.Fatalf("Bool returned %+v expected %+v", v, g)
			}
		case time.Duration:
			g := p.Get().Duration(name)
			if v != g {
				t.Fatalf("Duration returned %+v expected %+v", v, g)
			}
		case protocol.TunnelProtocols:
			g := p.Get().TunnelProtocols(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("TunnelProtocols returned %+v expected %+v", v, g)
			}
		case protocol.TLSProfiles:
			g := p.Get().TLSProfiles(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("TLSProfiles returned %+v expected %+v", v, g)
			}
		case protocol.QUICVersions:
			g := p.Get().QUICVersions(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("QUICVersions returned %+v expected %+v", v, g)
			}
		case DownloadURLs:
			g := p.Get().DownloadURLs(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("DownloadURLs returned %+v expected %+v", v, g)
			}
		case common.RateLimits:
			g := p.Get().RateLimits(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("RateLimits returned %+v expected %+v", v, g)
			}
		case http.Header:
			g := p.Get().HTTPHeaders(name)
			if !reflect.DeepEqual(v, g) {
				t.Fatalf("HTTPHeaders returned %+v expected %+v", v, g)
			}
		default:
			t.Fatalf("Unhandled default type: %s", name)
		}
	}
}

func TestGetValueLogger(t *testing.T) {

	loggerCalled := false

	p, err := NewClientParameters(
		func(error) {
			loggerCalled = true
		})
	if err != nil {
		t.Fatalf("NewClientParameters failed: %s", err)
	}

	p.Get().Int("unknown-parameter-name")

	if !loggerCalled {
		t.Fatalf("logged not called")
	}
}

func TestOverrides(t *testing.T) {

	tag := "tag"
	applyParameters := make(map[string]interface{})

	// Below minimum, should not apply
	defaultConnectionWorkerPoolSize := defaultClientParameters[ConnectionWorkerPoolSize].value.(int)
	minimumConnectionWorkerPoolSize := defaultClientParameters[ConnectionWorkerPoolSize].minimum.(int)
	newConnectionWorkerPoolSize := minimumConnectionWorkerPoolSize - 1
	applyParameters[ConnectionWorkerPoolSize] = newConnectionWorkerPoolSize

	// Above minimum, should apply
	defaultInitialLimitTunnelProtocolsCandidateCount := defaultClientParameters[InitialLimitTunnelProtocolsCandidateCount].value.(int)
	minimumInitialLimitTunnelProtocolsCandidateCount := defaultClientParameters[InitialLimitTunnelProtocolsCandidateCount].minimum.(int)
	newInitialLimitTunnelProtocolsCandidateCount := minimumInitialLimitTunnelProtocolsCandidateCount + 1
	applyParameters[InitialLimitTunnelProtocolsCandidateCount] = newInitialLimitTunnelProtocolsCandidateCount

	p, err := NewClientParameters(nil)
	if err != nil {
		t.Fatalf("NewClientParameters failed: %s", err)
	}

	// No skip on error; should fail and not apply any changes

	_, err = p.Set(tag, false, applyParameters)
	if err == nil {
		t.Fatalf("Set succeeded unexpectedly")
	}

	if p.Get().Tag() != "" {
		t.Fatalf("GetTag returned unexpected value")
	}

	v := p.Get().Int(ConnectionWorkerPoolSize)
	if v != defaultConnectionWorkerPoolSize {
		t.Fatalf("GetInt returned unexpected ConnectionWorkerPoolSize: %d", v)
	}

	v = p.Get().Int(InitialLimitTunnelProtocolsCandidateCount)
	if v != defaultInitialLimitTunnelProtocolsCandidateCount {
		t.Fatalf("GetInt returned unexpected InitialLimitTunnelProtocolsCandidateCount: %d", v)
	}

	// Skip on error; should skip ConnectionWorkerPoolSize and apply InitialLimitTunnelProtocolsCandidateCount

	counts, err := p.Set(tag, true, applyParameters)
	if err != nil {
		t.Fatalf("Set failed: %s", err)
	}

	if counts[0] != 1 {
		t.Fatalf("Apply returned unexpected count: %d", counts[0])
	}

	v = p.Get().Int(ConnectionWorkerPoolSize)
	if v != defaultConnectionWorkerPoolSize {
		t.Fatalf("GetInt returned unexpected ConnectionWorkerPoolSize: %d", v)
	}

	v = p.Get().Int(InitialLimitTunnelProtocolsCandidateCount)
	if v != newInitialLimitTunnelProtocolsCandidateCount {
		t.Fatalf("GetInt returned unexpected InitialLimitTunnelProtocolsCandidateCount: %d", v)
	}
}

func TestNetworkLatencyMultiplier(t *testing.T) {
	p, err := NewClientParameters(nil)
	if err != nil {
		t.Fatalf("NewClientParameters failed: %s", err)
	}

	timeout1 := p.Get().Duration(TunnelConnectTimeout)

	applyParameters := map[string]interface{}{"NetworkLatencyMultiplier": 2.0}

	_, err = p.Set("", false, applyParameters)
	if err != nil {
		t.Fatalf("Set failed: %s", err)
	}

	timeout2 := p.Get().Duration(TunnelConnectTimeout)

	if 2*timeout1 != timeout2 {
		t.Fatalf("Unexpected timeouts: 2 * %s != %s", timeout1, timeout2)
	}
}

func TestLimitTunnelProtocolProbability(t *testing.T) {
	p, err := NewClientParameters(nil)
	if err != nil {
		t.Fatalf("NewClientParameters failed: %s", err)
	}

	// Default probability should be 1.0 and always return tunnelProtocols

	tunnelProtocols := protocol.TunnelProtocols{"OSSH", "SSH"}

	applyParameters := map[string]interface{}{
		"LimitTunnelProtocols": tunnelProtocols,
	}

	_, err = p.Set("", false, applyParameters)
	if err != nil {
		t.Fatalf("Set failed: %s", err)
	}

	for i := 0; i < 1000; i++ {
		l := p.Get().TunnelProtocols(LimitTunnelProtocols)
		if !reflect.DeepEqual(l, tunnelProtocols) {
			t.Fatalf("unexpected %+v != %+v", l, tunnelProtocols)
		}
	}

	// With probability set to 0.5, should return tunnelProtocols ~50%

	defaultLimitTunnelProtocols := protocol.TunnelProtocols{}

	applyParameters = map[string]interface{}{
		"LimitTunnelProtocolsProbability": 0.5,
		"LimitTunnelProtocols":            tunnelProtocols,
	}

	_, err = p.Set("", false, applyParameters)
	if err != nil {
		t.Fatalf("Set failed: %s", err)
	}

	matchCount := 0

	for i := 0; i < 1000; i++ {
		l := p.Get().TunnelProtocols(LimitTunnelProtocols)
		if reflect.DeepEqual(l, tunnelProtocols) {
			matchCount += 1
		} else if !reflect.DeepEqual(l, defaultLimitTunnelProtocols) {
			t.Fatalf("unexpected %+v != %+v", l, defaultLimitTunnelProtocols)
		}
	}

	if matchCount < 250 || matchCount > 750 {
		t.Fatalf("Unexpected probability result: %d", matchCount)
	}
}
