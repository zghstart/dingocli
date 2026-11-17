/*
* Copyright (c) 2026 dingodb.com, Inc. All Rights Reserved
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
 */

package configure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/dingodb/dingocli/cli/cli"
	"github.com/dingodb/dingocli/internal/common"
	confHost "github.com/dingodb/dingocli/internal/configure/hosts"
	"github.com/dingodb/dingocli/internal/configure/topology"
	"github.com/dingodb/dingocli/internal/errno"
	"github.com/dingodb/dingocli/pkg/variable"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/spf13/viper"
)

const (
	ROLE_NODE_EXPORTER = "node_exporter"
	ROLE_PROMETHEUS    = "prometheus"
	ROLE_GRAFANA       = "grafana"
	ROLE_MONITOR_CONF  = "monitor_conf"
	ROLE_MONITOR_SYNC  = "monitor_sync"

	KEY_HOST              = "host"
	KEY_LISTEN_PORT       = "listen_port"
	KEY_RETENTION_TIME    = "retention.time"
	KEY_RETENTION_SIZE    = "retention.size"
	KEY_PROMETHEUS_TARGET = "target"
	KEY_GRAFANA_USER      = "username"
	KEY_GRAFANA_PASSWORD  = "password"

	KEY_NODE_IPS         = "node_ips"
	KEY_NODE_TARGETS     = "node_targets"
	KRY_NODE_LISTEN_PORT = "node_listen_port"
	KEY_PROMETHEUS_IP    = "prometheus_listen_ip"
	KEY_PROMETHEUS_PORT  = "prometheus_listen_port"

	KEY_ORIGIN_CONFIG_ID = "origin_config_id"

	INFO_TYPE_FILE = "file"
	INFO_TYPE_DATA = "data"
)

type (
	deploy struct {
		Host string `mapstructure:"host"`
	}

	service struct {
		Config map[string]interface{} `mapstructure:"config"`
		Deploy []deploy               `mapstructure:"deploy"`
	}

	Monitor struct {
		Global       map[string]interface{} `mapstructure:"global"`
		NodeExporter service                `mapstructure:"node_exporter"`
		Prometheus   service                `mapstructure:"prometheus"`
		Grafana      service                `mapstructure:"grafana"`
		MonitroSync  service                `mapstructure:"monitor_sync"`
	}

	MonitorConfig struct {
		kind         string
		id           string // role_host
		role         string
		host         string
		hostSequence int
		config       map[string]interface{}
		variables    *variable.Variables
		ctx          *topology.Context
		order        int
	}

	serviceTarget struct {
		Targets []string          `json:"targets"`
		Labels  map[string]string `json:"labels"`
	}

	FilterMonitorOption struct {
		Id   string
		Role string
		Host string
	}

	MonitorDiff struct {
		DiffType      int
		MonitorConfig *MonitorConfig
	}
)

func (m *MonitorConfig) getConfig() map[string]interface{} {
	return m.config
}

func (m *MonitorConfig) getString(data *map[string]interface{}, key string) string {
	v := (*data)[strings.ToLower(key)]
	if v == nil {
		return ""
	}
	return v.(string)
}

func (m *MonitorConfig) getStrings(data *map[string]interface{}, key string) []string {
	v := (*data)[strings.ToLower(key)]
	if v == nil {
		return []string{}
	}
	return v.([]string)
}

func (m *MonitorConfig) getInt(data *map[string]interface{}, key string) int {
	v := (*data)[strings.ToLower(key)]
	if v == nil {
		return -1
	}
	return v.(int)
}

func (m *MonitorConfig) GetKind() string {
	return m.kind
}

func (m *MonitorConfig) GetId() string {
	return m.id
}

func (m *MonitorConfig) GetRole() string {
	return m.role
}

func (m *MonitorConfig) GetHost() string {
	return m.host
}

func (m *MonitorConfig) GetHostSequence() int {
	return m.hostSequence
}

func (m *MonitorConfig) GetNodeIps() []string {
	return m.getStrings(&m.config, KEY_NODE_IPS)
}

func (m *MonitorConfig) GetNodeTargets() string {
	return m.getString(&m.config, KEY_NODE_TARGETS)
}

func (m *MonitorConfig) GetNodeListenPort() int {
	return m.getInt(&m.config, KRY_NODE_LISTEN_PORT)
}

func (m *MonitorConfig) GetPrometheusListenPort() int {
	return m.getInt(&m.config, KEY_PROMETHEUS_PORT)
}

func (m *MonitorConfig) GetImage() string {
	return m.getString(&m.config, KEY_CONTAINER_IMAGE)
}

func (m *MonitorConfig) GetListenPort() int {
	return m.getInt(&m.config, KEY_LISTEN_PORT)
}

func (m *MonitorConfig) GetDataDir() string {
	return m.getString(&m.config, KEY_DATA_DIR)
}

func (m *MonitorConfig) GetConfDir() string {
	return m.getString(&m.config, KEY_CONF_DIR)
}

func (m *MonitorConfig) GetProvisionDir() string {
	return m.getString(&m.config, KEY_PROVISIONING_DIR)
}

func (m *MonitorConfig) GetLogDir() string {
	return m.getString(&m.config, KEY_LOG_DIR)
}

func (m *MonitorConfig) GetPrometheusRetentionTime() string {
	return m.getString(&m.config, KEY_RETENTION_TIME)
}

func (m *MonitorConfig) GetPrometheusRetentionSize() string {
	return m.getString(&m.config, KEY_RETENTION_SIZE)
}

func (m *MonitorConfig) GetPrometheusTarget() string {
	return m.getString(&m.config, KEY_PROMETHEUS_TARGET)
}

func (m *MonitorConfig) GetPrometheusIp() string {
	return m.getString(&m.config, KEY_PROMETHEUS_IP)
}

func (m *MonitorConfig) GetGrafanaUser() string {
	return m.getString(&m.config, KEY_GRAFANA_USER)
}

func (m *MonitorConfig) GetGrafanaPassword() string {
	return m.getString(&m.config, KEY_GRAFANA_PASSWORD)
}

func (m *MonitorConfig) GetVariables() *variable.Variables { return m.variables }

func (m *MonitorConfig) GetServiceConfig() map[string]interface{} {
	return m.config
}

func (m *MonitorConfig) GetOrder() int {
	return m.order
}

func (m *MonitorConfig) GetContext() *topology.Context {
	return m.ctx
}

func getHost(c *Monitor, role string) []string {
	deploys := c.NodeExporter.Deploy
	explicitCentral := false
	switch role {
	case ROLE_PROMETHEUS:
		if len(c.Prometheus.Deploy) > 0 {
			deploys = c.Prometheus.Deploy
			explicitCentral = true
		}
	case ROLE_GRAFANA:
		if len(c.Grafana.Deploy) > 0 {
			deploys = c.Grafana.Deploy
			explicitCentral = true
		}
	case ROLE_MONITOR_SYNC:
		if len(c.MonitroSync.Deploy) > 0 {
			deploys = c.MonitroSync.Deploy
			explicitCentral = true
		}
	}
	hosts := []string{}
	seenHosts := map[string]bool{}
	for _, d := range deploys {
		if seenHosts[d.Host] {
			continue
		}
		seenHosts[d.Host] = true
		hosts = append(hosts, d.Host)
	}
	if role != ROLE_NODE_EXPORTER && !explicitCentral && len(hosts) > 0 {
		hosts = hosts[:1]
	}
	switch role {
	case ROLE_NODE_EXPORTER:
		if c.NodeExporter.Config == nil {
			c.NodeExporter.Config = map[string]interface{}{}
		}
		c.NodeExporter.Config[KEY_HOST] = hosts
	case ROLE_PROMETHEUS:
		if c.Prometheus.Config == nil {
			c.Prometheus.Config = map[string]interface{}{}
		}
		c.Prometheus.Config[KEY_HOST] = hosts
	case ROLE_GRAFANA:
		if c.Grafana.Config == nil {
			c.Grafana.Config = map[string]interface{}{}
		}
		c.Grafana.Config[KEY_HOST] = hosts
	}
	return hosts
}

func validateMonitorPlacement(c *Monitor, ctx *topology.Context, roles []string) error {
	central := ""
	for _, role := range roles {
		if role == ROLE_NODE_EXPORTER {
			continue
		}
		hosts := getHost(c, role)
		if len(hosts) != 1 || ctx.Lookup(hosts[0]) == "" {
			return fmt.Errorf("monitor role %s requires one registered central host", role)
		}
		if central != "" && hosts[0] != central {
			return fmt.Errorf("prometheus, grafana and monitor_sync must share one host for their local endpoints")
		}
		central = hosts[0]
	}
	return nil
}

// Resolve the explicitly configured monitoring hosts, including clients and
// caches that are absent from the metadata-service deployment topology.
func monitorNodeTargets(c *Monitor, ctx *topology.Context, cluster string) ([]string, string, error) {
	seen := map[string]bool{}
	addresses := map[string]string{}
	ips := []string{}
	targets := []serviceTarget{}
	port, ok := c.NodeExporter.Config[KEY_LISTEN_PORT].(int)
	if !ok || port <= 0 || port > 65535 {
		return nil, "", fmt.Errorf("invalid node_exporter listen_port")
	}
	for _, host := range getHost(c, ROLE_NODE_EXPORTER) {
		if seen[host] {
			continue
		}
		seen[host] = true
		ip := ctx.Lookup(host)
		if ip == "" {
			return nil, "", fmt.Errorf("monitor host %q is not registered in dingo hosts", host)
		}
		if other, exists := addresses[ip]; exists && other != host {
			return nil, "", fmt.Errorf("monitor hosts %q and %q share address %q", other, host, ip)
		}
		addresses[ip] = host
		ips = append(ips, ip)
		targets = append(targets, serviceTarget{Targets: []string{net.JoinHostPort(ip, strconv.Itoa(port))},
			Labels: map[string]string{"cluster": cluster, "component": "host", "node": host, "group": "server"}})
	}
	if len(targets) == 0 {
		return nil, "", fmt.Errorf("node_exporter deploy list must not be empty")
	}
	encoded, err := json.Marshal(targets)
	return ips, string(encoded), err
}

func parsePrometheusTarget(dcs []*topology.DeployConfig) (string, error) {
	targets := []serviceTarget{}
	tMap := make(map[string]serviceTarget)
	for _, dc := range dcs {
		role := dc.GetRole()
		ip := dc.GetListenIp()
		var item string
		switch role {
		case topology.ROLE_ETCD:
			item = fmt.Sprintf("%s:%d", ip, dc.GetListenClientPort())
		case topology.ROLE_CHUNKSERVER,
			// topology.ROLE_MDS_V1,
			topology.ROLE_METASERVER:
			item = fmt.Sprintf("%s:%d", ip, dc.GetListenPort())
		case topology.ROLE_SNAPSHOTCLONE:
			item = fmt.Sprintf("%s:%d", ip, dc.GetListenDummyPort())
		case topology.ROLE_FS_MDS,
			topology.ROLE_COORDINATOR,
			topology.ROLE_STORE:
			item = fmt.Sprintf("%s:%d", ip, dc.GetDingoServerPort())
		}
		if _, ok := tMap[role]; ok {
			t := tMap[role]
			t.Targets = append(t.Targets, item)
			tMap[role] = t
		} else {
			tMap[role] = serviceTarget{
				Labels:  map[string]string{"job": role},
				Targets: []string{item},
			}
		}
	}
	for _, v := range tMap {
		targets = append(targets, v)
	}
	target, err := json.Marshal(targets)
	if err != nil {
		return "", errno.ERR_PARSE_PROMETHEUS_TARGET_FAILED.E(err)
	}
	return string(target), nil
}

func parseHosts(dingocli *cli.DingoCli) ([]string, []string, []*topology.DeployConfig, error) {
	dcs, err := dingocli.ParseTopology()
	if err != nil || len(dcs) == 0 {
		return nil, nil, nil, err
	}
	hosts := []string{}
	hostIps := []string{}
	thostMap := make(map[string]bool)
	thostIpMap := make(map[string]bool)
	for _, dc := range dcs {
		thostMap[dc.GetHost()] = true
		thostIpMap[dc.GetListenIp()] = true
	}
	for key := range thostMap {
		hosts = append(hosts, key)
	}
	for key := range thostIpMap {
		hostIps = append(hostIps, key)
	}
	return hosts, hostIps, dcs, nil
}

func ParseMonitor(dingocli *cli.DingoCli) ([]*MonitorConfig, error) {
	return ParseMonitorInfo(dingocli, dingocli.Monitor().Monitor, INFO_TYPE_DATA)
}

// ParseMonitorFile parses monitor configuration from a file or from existing monitor data. infoType can be "file" or "data".
func ParseMonitorInfo(dingocli *cli.DingoCli, info string, infoType string) ([]*MonitorConfig, error) {
	_, _, dcs, err := parseHosts(dingocli)
	if err != nil {
		return nil, err
	}

	// parse monitor configure
	parser := viper.NewWithOptions(viper.KeyDelimiter("::"))
	parser.SetConfigType("yaml")

	switch infoType {
	case INFO_TYPE_FILE:
		parser.SetConfigFile(info)
		if err := parser.ReadInConfig(); err != nil {
			return nil, errno.ERR_PARSE_MONITOR_CONFIGURE_FAILED.E(err)
		}
	case INFO_TYPE_DATA:
		if len(info) != 0 && info != common.CLEANED_MONITOR_CONF {
			if err := parser.ReadConfig(bytes.NewBuffer([]byte(info))); err != nil {
				return nil, errno.ERR_PARSE_MONITOR_CONFIGURE_FAILED.E(err)
			}
		} else {
			return nil, errno.ERR_PARSE_MONITOR_CONFIGURE_FAILED
		}
	default:
		return nil, errno.ERR_PARSE_MONITOR_CONFIGURE_FAILED.F("invalid info type: %s", infoType)
	}

	config := Monitor{}
	if err := parser.Unmarshal(&config); err != nil {
		return nil, errno.ERR_PARSE_MONITOR_CONFIGURE_FAILED.E(err)
	}

	// get host -> hostname(ip)
	ctx := topology.NewContext()
	hcs, err := confHost.ParseHosts(dingocli.Hosts())
	if err != nil {
		return nil, err
	}
	for _, hc := range hcs {
		ctx.Add(hc.GetHost(), hc.GetHostname())
	}

	mkind := dcs[0].GetKind()
	// mconfImage := dcs[0].GetContainerImage()
	syncMonitorPath, ok := config.MonitroSync.Config[KEY_DATA_DIR].(string)
	if !ok || syncMonitorPath == "" {
		return nil, fmt.Errorf("monitor_sync.config.data_dir is required")
	}
	var hostIps []string
	var nodeTargets string
	if config.NodeExporter.Deploy != nil {
		hostIps, nodeTargets, err = monitorNodeTargets(&config, ctx, dingocli.ClusterName())
		if err != nil {
			return nil, err
		}
	}
	roles := []string{}
	switch {
	case config.NodeExporter.Deploy != nil:
		roles = append(roles, ROLE_NODE_EXPORTER)
		fallthrough
	case config.Prometheus.Deploy != nil:
		roles = append(roles, ROLE_PROMETHEUS)
		fallthrough
	case config.Grafana.Deploy != nil:
		roles = append(roles, ROLE_GRAFANA)
		fallthrough
	case config.MonitroSync.Config != nil:
		roles = append(roles, ROLE_MONITOR_SYNC)
	}
	ret := []*MonitorConfig{}
	if err := validateMonitorPlacement(&config, ctx, roles); err != nil {
		return nil, err
	}
	for _, role := range roles {
		// prometheus/grafana use as default host
		serviceHosts := getHost(&config, role)
		if len(serviceHosts) == 0 {
			return nil, fmt.Errorf("no host configured for monitor role %s", role)
		}
		host := serviceHosts[0]
		switch role {
		case ROLE_PROMETHEUS:
			target, err := parsePrometheusTarget(dcs)
			if err != nil {
				return nil, err
			}
			if config.NodeExporter.Deploy != nil {
				config.Prometheus.Config[KEY_NODE_IPS] = hostIps
				config.Prometheus.Config[KEY_NODE_TARGETS] = nodeTargets
				config.Prometheus.Config[KRY_NODE_LISTEN_PORT] = config.NodeExporter.Config[KEY_LISTEN_PORT]
				config.Prometheus.Config[KEY_CONF_DIR] = syncMonitorPath + "/" + ROLE_PROMETHEUS
				config.Prometheus.Config[KEY_DATA_DIR] = syncMonitorPath + "/" + ROLE_PROMETHEUS + "/data"
			}
			config.Prometheus.Config[KEY_PROMETHEUS_TARGET] = target
			ret = append(ret, &MonitorConfig{
				kind:   mkind,
				id:     fmt.Sprintf("%s_%s", role, host),
				role:   role,
				host:   host,
				config: config.Prometheus.Config,
				ctx:    ctx,
				order:  2,
			})
		case ROLE_GRAFANA:
			if config.Prometheus.Deploy != nil {
				config.Grafana.Config[KEY_PROMETHEUS_PORT] = config.Prometheus.Config[KEY_LISTEN_PORT]
				config.Grafana.Config[KEY_PROMETHEUS_IP] = ctx.Lookup(config.Prometheus.Config[KEY_HOST].([]string)[0])
				config.Grafana.Config[KEY_CONF_DIR] = syncMonitorPath + "/" + ROLE_GRAFANA
				config.Grafana.Config[KEY_DATA_DIR] = syncMonitorPath + "/" + ROLE_GRAFANA + "/data"
				config.Grafana.Config[KEY_PROVISIONING_DIR] = syncMonitorPath + "/" + ROLE_GRAFANA + "/provisioning"
			}
			ret = append(ret, &MonitorConfig{
				kind:   mkind,
				id:     fmt.Sprintf("%s_%s", role, host),
				role:   role,
				host:   host,
				config: config.Grafana.Config,
				ctx:    ctx,
				order:  3,
			},
			)
		case ROLE_NODE_EXPORTER:
			for hostSequence, h := range serviceHosts {
				ret = append(ret, &MonitorConfig{
					kind:         mkind,
					id:           fmt.Sprintf("%s_%s", role, h),
					role:         role,
					host:         h,
					hostSequence: hostSequence,
					config:       config.NodeExporter.Config,
					ctx:          ctx,
					order:        1,
				})
			}
		case ROLE_MONITOR_SYNC:
			config.MonitroSync.Config[KEY_ORIGIN_CONFIG_ID] = dcs[0].GetId()
			ret = append(ret, &MonitorConfig{
				kind:      mkind,
				id:        fmt.Sprintf("%s_%s", role, host),
				role:      role,
				host:      host,
				config:    config.MonitroSync.Config,
				variables: dcs[0].GetVariables(),
				ctx:       ctx,
				order:     0,
			})
		}
	}
	return ret, nil
}

func FilterMonitorConfig(dingocli *cli.DingoCli, mcs []*MonitorConfig,
	options FilterMonitorOption) []*MonitorConfig {
	ret := []*MonitorConfig{}
	for _, mc := range mcs {
		mcId := mc.GetId()
		role := mc.GetRole()
		host := mc.GetHost()
		serviceId := dingocli.GetServiceId(mcId)
		if (options.Id == "*" || options.Id == serviceId) &&
			(options.Role == "*" || options.Role == role) &&
			(options.Host == "*" || options.Host == host) {
			ret = append(ret, mc)
		}
	}
	return ret
}

// DiffMonitor compares two monitor configuration data and returns the differences.
// data1: existing monitor configuration, data2: new monitor configuration.
func DiffMonitor(dingocli *cli.DingoCli, data1, data2 string) ([]MonitorDiff, error) {
	if len(data1) == 0 {
		return nil, errno.ERR_EMPTY_CLUSTER_TOPOLOGY
	}

	mcs, err := ParseMonitorInfo(dingocli, data1, INFO_TYPE_DATA)
	if err != nil {
		return nil, err // err is error code
	}
	if len(mcs) == 0 {
		return nil, errno.ERR_NO_SERVICES_IN_TOPOLOGY
	}
	return diffMonitor(dingocli, data1, data2)
}

// return ids which belong to ids1, but not belong to ids2
func difference(ids1, ids2 map[string]*MonitorConfig) map[string]*MonitorConfig {
	ids := map[string]*MonitorConfig{}
	for k, v := range ids1 {
		if _, ok := ids2[k]; !ok {
			ids[k] = v
		}
	}

	return ids
}

func hash(mc *MonitorConfig) (uint64, error) {
	return hashstructure.Hash(mc.getConfig(), hashstructure.FormatV2, nil)
}

func same(mc1, mc2 *MonitorConfig) (bool, error) {
	hash1, err := hash(mc1)
	if err != nil {
		return false, errno.ERR_CREATE_HASH_FOR_TOPOLOGY_FAILED.E(err)
	}

	hash2, err := hash(mc2)
	if err != nil {
		return false, errno.ERR_CREATE_HASH_FOR_TOPOLOGY_FAILED.E(err)
	}

	return hash1 == hash2, nil
}

func diffMonitor(dingocli *cli.DingoCli, data1, data2 string) ([]MonitorDiff, error) {
	var mcs1, mcs2 []*MonitorConfig
	var err error

	mcs1, err = ParseMonitorInfo(dingocli, data1, INFO_TYPE_DATA)
	if err != nil {
		return nil, err
	}

	mcs2, err = ParseMonitorInfo(dingocli, data2, INFO_TYPE_DATA)
	if err != nil {
		return nil, err
	}

	ids1 := map[string]*MonitorConfig{}
	for _, dc := range mcs1 {
		ids1[dc.GetId()] = dc
	}

	ids2 := map[string]*MonitorConfig{}
	for _, dc := range mcs2 {
		ids2[dc.GetId()] = dc
	}

	diffs := []MonitorDiff{}

	// DELETE
	deleteIds := difference(ids1, ids2)
	for _, mc := range deleteIds {
		diffs = append(diffs, MonitorDiff{
			DiffType:      topology.DIFF_DELETE,
			MonitorConfig: mc,
		})
	}

	// ADD
	addIds := difference(ids2, ids1)
	for _, mc := range addIds {
		diffs = append(diffs, MonitorDiff{
			DiffType:      topology.DIFF_ADD,
			MonitorConfig: mc,
		})
	}

	// CHANGE
	for id, mc := range ids2 {
		if _, ok := deleteIds[id]; ok {
			continue
		} else if _, ok := addIds[id]; ok {
			continue
		}

		ok, err := same(ids1[id], mc)
		if err != nil {
			return nil, err
		} else if !ok {
			diffs = append(diffs, MonitorDiff{
				DiffType:      topology.DIFF_CHANGE,
				MonitorConfig: mc,
			})
		}
	}

	return diffs, nil
}
