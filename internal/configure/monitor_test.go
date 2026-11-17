package configure

import (
	"encoding/json"
	"testing"

	"github.com/dingodb/dingocli/internal/configure/topology"
)

func TestMonitorTargetsIncludeConfiguredClientAndCacheHosts(t *testing.T) {
	ctx := topology.NewContext()
	for host, ip := range map[string]string{"meta": "10.0.0.1", "cache": "10.0.0.2", "client": "2001:db8::1"} {
		ctx.Add(host, ip)
	}
	c := &Monitor{NodeExporter: service{Config: map[string]interface{}{KEY_LISTEN_PORT: 9100, KEY_HOST: []string{"meta"}},
		Deploy: []deploy{{Host: "meta"}, {Host: "cache"}, {Host: "client"}, {Host: "client"}}}}
	ips, encoded, err := monitorNodeTargets(c, ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	var groups []serviceTarget
	if err := json.Unmarshal([]byte(encoded), &groups); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 3 || len(ips) != 3 {
		t.Fatalf("missing monitoring hosts: %s", encoded)
	}
	if groups[2].Targets[0] != "[2001:db8::1]:9100" {
		t.Fatal(encoded)
	}
	for i, host := range []string{"meta", "cache", "client"} {
		if groups[i].Labels["node"] != host || groups[i].Labels["cluster"] != "test" {
			t.Fatalf("missing stable labels: %s", encoded)
		}
	}
}

func TestMonitorTargetsRejectUnknownHostsAndAddressAliases(t *testing.T) {
	ctx := topology.NewContext()
	ctx.Add("meta", "10.0.0.1")
	ctx.Add("alias", "10.0.0.1")
	for _, hosts := range [][]deploy{{}, {{Host: "unknown"}}, {{Host: "meta"}, {Host: "alias"}}} {
		c := &Monitor{NodeExporter: service{Config: map[string]interface{}{KEY_LISTEN_PORT: 9100}, Deploy: hosts}}
		if _, _, err := monitorNodeTargets(c, ctx, "test"); err == nil {
			t.Fatalf("accepted invalid hosts: %v", hosts)
		}
	}
}

func TestMonitorCentralRolesHonorExplicitDeployment(t *testing.T) {
	c := &Monitor{
		NodeExporter: service{Config: map[string]interface{}{}, Deploy: []deploy{{Host: "client"}, {Host: "meta"}}},
		Prometheus:   service{Config: map[string]interface{}{}, Deploy: []deploy{{Host: "meta"}}},
		Grafana:      service{Config: map[string]interface{}{}, Deploy: []deploy{{Host: "meta"}}},
		MonitroSync:  service{Config: map[string]interface{}{}, Deploy: []deploy{{Host: "meta"}}},
	}
	for _, role := range []string{ROLE_PROMETHEUS, ROLE_GRAFANA, ROLE_MONITOR_SYNC} {
		hosts := getHost(c, role)
		if len(hosts) != 1 || hosts[0] != "meta" {
			t.Fatalf("%s selected %v", role, hosts)
		}
	}
	if len(getHost(c, ROLE_NODE_EXPORTER)) != 2 {
		t.Fatal("node exporter coverage changed")
	}
}

func TestMonitorPlacementRejectsSplitLocalEndpoints(t *testing.T) {
	ctx := topology.NewContext()
	ctx.Add("meta", "10.0.0.1")
	ctx.Add("other", "10.0.0.2")
	c := &Monitor{NodeExporter: service{Deploy: []deploy{{Host: "meta"}, {Host: "other"}}}}
	roles := []string{ROLE_PROMETHEUS, ROLE_GRAFANA, ROLE_MONITOR_SYNC}
	if err := validateMonitorPlacement(c, ctx, roles); err != nil {
		t.Fatal(err)
	}
	c.Grafana.Deploy = []deploy{{Host: "other"}}
	if err := validateMonitorPlacement(c, ctx, roles); err == nil {
		t.Fatal("accepted split localhost endpoints")
	}
}
