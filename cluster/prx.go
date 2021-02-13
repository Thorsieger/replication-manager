// replication-manager - Replication Manager Monitoring and CLI for MariaDB and MySQL
// Copyright 2017 Signal 18 SARL
// Authors: Guillaume Lefranc <guillaume@signal18.io>
//          Stephane Varoqui  <svaroqui@gmail.com>
// This source code is licensed under the GNU General Public License, version 3.
// Redistribution/Reuse of this code is permitted under the GNU v3 license, as
// an additional term, ALL code must carry the original Author(s) credit in comment form.
// See LICENSE in this directory for the integral text.
package cluster

import (
	"fmt"
	"hash/crc64"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/signal18/replication-manager/config"
	"github.com/signal18/replication-manager/graphite"
	"github.com/signal18/replication-manager/router/myproxy"
	"github.com/signal18/replication-manager/router/proxysql"
	"github.com/signal18/replication-manager/utils/crypto"
	"github.com/signal18/replication-manager/utils/dbhelper"
	"github.com/signal18/replication-manager/utils/misc"
	"github.com/signal18/replication-manager/utils/state"
	"github.com/spf13/pflag"
)

// Proxy defines a proxy
type Proxy struct {
	DatabaseProxy
	Id              string               `json:"id"`
	Name            string               `json:"name"`
	Type            string               `json:"type"`
	Host            string               `json:"host"`
	HostIPV6        string               `json:"hostIPV6"`
	Port            string               `json:"port"`
	TunnelPort      int                  `json:"tunnelPort"`
	TunnelWritePort int                  `json:"tunnelWritePort"`
	Tunnel          bool                 `json:"tunnel"`
	User            string               `json:"user"`
	Pass            string               `json:"-"`
	WritePort       int                  `json:"writePort"`
	ReadPort        int                  `json:"readPort"`
	ReadWritePort   int                  `json:"readWritePort"`
	ReaderHostgroup int                  `json:"readerHostGroup"`
	WriterHostgroup int                  `json:"writerHostGroup"`
	BackendsWrite   []Backend            `json:"backendsWrite"`
	BackendsRead    []Backend            `json:"backendsRead"`
	Version         string               `json:"version"`
	InternalProxy   *myproxy.Server      `json:"internalProxy"`
	ShardProxy      *ServerMonitor       `json:"shardProxy"`
	ClusterGroup    *Cluster             `json:"-"`
	Datadir         string               `json:"datadir"`
	QueryRules      []proxysql.QueryRule `json:"queryRules"`
	State           string               `json:"state"`
	PrevState       string               `json:"prevState"`
	FailCount       int                  `json:"failCount"`
	SlapOSDatadir   string               `json:"slaposDatadir"`
	Process         *os.Process          `json:"process"`
	Variables       map[string]string    `json:"-"`
	ServiceName     string               `json:"serviceName"`
	Agent           string               `json:"agent"`
}

func (p *Proxy) GetAgent() string {
	return p.Agent
}

func (p *Proxy) GetType() string {
	return p.Type
}

func (p *Proxy) GetHost() string {
	return p.Host
}

func (p *Proxy) GetPort() string {
	return p.Port
}

func (p *Proxy) GetWritePort() int {
	return p.WritePort
}

func (p *Proxy) GetId() string {
	return p.Id
}

func (p *Proxy) GetState() string {
	return p.State
}

func (p *Proxy) SetState(v string) {
	p.State = v
}

func (p *Proxy) GetUser() string {
	return p.User
}

func (p *Proxy) GetPass() string {
	return p.Pass
}

func (p *Proxy) GetFailCount() int {
	return p.FailCount
}

func (p *Proxy) SetFailCount(c int) {
	p.FailCount = c
}

func (p *Proxy) SetCredential(credential string) {
	p.User, p.Pass = misc.SplitPair(credential)
}

func (p *Proxy) GetPrevState() string {
	return p.PrevState
}

func (p *Proxy) SetPrevState(state string) {
	p.PrevState = state
}

func (p *Proxy) SetSuspect() {
	p.State = stateSuspect
}

type DatabaseProxy interface {
	AddFlags(flags *pflag.FlagSet, conf config.Config)
	Init()
	Refresh() error
	Failover()
	SetMaintenance(server *ServerMonitor)
	GetType() string
	DelRestartCookie()
	DelReprovisionCookie()

	SetProvisionCookie()
	HasProvisionCookie() bool
	IsRunning() bool
	SetRestartCookie()
	HasRestartCookie() bool
	SetReprovCookie()
	HasReprovCookie() bool

	SetCredential(credential string)

	GetFailCount() int
	SetFailCount(c int)

	GetAgent() string
	GetName() string
	GetHost() string
	GetPort() string
	GetWritePort() int
	GetReadWritePort() int
	GetReadPort() int
	GetId() string
	GetState() string
	SetState(v string)
	GetUser() string
	GetPass() string
	GetServiceName() string

	GetPrevState() string
	SetPrevState(state string)

	GetCluster() (*sqlx.DB, error)

	SetMaintenanceHaproxy(server *ServerMonitor)

	IsFilterInTags(filter string) bool
	HasWaitStartCookie() bool
	HasWaitStopCookie() bool
	IsDown() bool

	DelProvisionCookie()
	DelWaitStartCookie()
	DelWaitStopCookie()
	GetProxyConfig() string
	// GetInitContainer(collector opensvc.Collector) string
	GetBindAddress() string
	GetBindAddressExtraIPV6() string
	GetUseSSL() string
	GetUseCompression() string
	GetDatadir() string
	GetEnv() map[string]string
	GetConfigProxyModule(variable string) string

	SendStats() error

	OpenSVCGetProxyDefaultSection() map[string]string
	SetWaitStartCookie()
	SetWaitStopCookie()

	SetSuspect()
}

type Backend struct {
	Host           string `json:"host"`
	Port           string `json:"port"`
	Status         string `json:"status"`
	PrxName        string `json:"prxName"`
	PrxStatus      string `json:"prxStatus"`
	PrxConnections string `json:"prxConnections"`
	PrxHostgroup   string `json:"prxHostgroup"`
	PrxByteOut     string `json:"prxByteOut"`
	PrxByteIn      string `json:"prxByteIn"`
	PrxLatency     string `json:"prxLatency"`
	PrxMaintenance bool   `json:"prxMaintenance"`
}

type proxyList []DatabaseProxy

func (cluster *Cluster) newProxyList() error {
	nbproxies := 0

	crcTable := crc64.MakeTable(crc64.ECMA) // http://golang.org/pkg/hash/crc64/#pkg-constants
	if cluster.Conf.MxsHost != "" && cluster.Conf.MxsOn {
		nbproxies += len(strings.Split(cluster.Conf.MxsHost, ","))
	}
	if cluster.Conf.HaproxyOn {
		nbproxies += len(strings.Split(cluster.Conf.HaproxyHosts, ","))
	}
	if cluster.Conf.MdbsProxyHosts != "" && cluster.Conf.MdbsProxyOn {
		nbproxies += len(strings.Split(cluster.Conf.MdbsProxyHosts, ","))
	}
	if cluster.Conf.ProxysqlOn {
		nbproxies += len(strings.Split(cluster.Conf.ProxysqlHosts, ","))
	}
	if cluster.Conf.MysqlRouterOn {
		nbproxies += len(strings.Split(cluster.Conf.MysqlRouterHosts, ","))
	}
	if cluster.Conf.SphinxOn {
		nbproxies += len(strings.Split(cluster.Conf.SphinxHosts, ","))
	}
	if cluster.Conf.ExtProxyOn {
		nbproxies++
	}
	// internal myproxy
	if cluster.Conf.MyproxyOn {
		nbproxies++
	}
	cluster.Proxies = make([]DatabaseProxy, nbproxies)

	cluster.LogPrintf(LvlInfo, "Loading %d proxies", nbproxies)

	var ctproxy = 0
	var err error

	if cluster.Conf.MxsHost != "" && cluster.Conf.MxsOn {

		for k, proxyHost := range strings.Split(cluster.Conf.MxsHost, ",") {
			// prx := new(Proxy)
			prx := new(MaxscaleProxy)
			prx.Type = config.ConstProxyMaxscale
			prx.SetPlacement(k, cluster.Conf.ProvProxAgents, cluster.Conf.SlapOSMaxscalePartitions, cluster.Conf.MxsHostsIPV6)
			prx.Port = cluster.Conf.MxsPort
			prx.User = cluster.Conf.MxsUser
			prx.Pass = cluster.Conf.MxsPass
			if cluster.key != nil {
				p := crypto.Password{Key: cluster.key}
				p.CipherText = prx.Pass
				p.Decrypt()
				prx.Pass = p.PlainText
			}
			prx.ReadPort = cluster.Conf.MxsReadPort
			prx.WritePort = cluster.Conf.MxsWritePort
			prx.ReadWritePort = cluster.Conf.MxsReadWritePort
			prx.Name = proxyHost
			prx.Host = proxyHost
			if cluster.Conf.ProvNetCNI {
				prx.Host = prx.Host + "." + cluster.Name + ".svc." + cluster.Conf.ProvOrchestratorCluster
			}
			prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
			prx.ClusterGroup = cluster

			prx.SetDataDir()
			prx.SetServiceName(cluster.Name, prx.Name)
			cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
			prx.State = stateSuspect
			cluster.Proxies[ctproxy] = prx
			if err != nil {
				cluster.LogPrintf(LvlErr, "Could not open connection to proxy %s %s: %s", prx.Host, prx.GetPort(), err)
			}
			ctproxy++
		}
	}
	if cluster.Conf.HaproxyOn {

		for k, proxyHost := range strings.Split(cluster.Conf.HaproxyHosts, ",") {
			prx := new(HaproxyProxy)
			prx.SetPlacement(k, cluster.Conf.ProvProxAgents, cluster.Conf.SlapOSHaProxyPartitions, cluster.Conf.HaproxyHostsIPV6)
			prx.Type = config.ConstProxyHaproxy
			prx.Port = strconv.Itoa(cluster.Conf.HaproxyAPIPort)
			prx.ReadPort = cluster.Conf.HaproxyReadPort
			prx.WritePort = cluster.Conf.HaproxyWritePort
			prx.ReadWritePort = cluster.Conf.HaproxyWritePort
			prx.Name = proxyHost
			prx.Host = proxyHost
			if cluster.Conf.ProvNetCNI {
				prx.Host = prx.Host + "." + cluster.Name + ".svc." + cluster.Conf.ProvOrchestratorCluster
			}
			prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
			prx.ClusterGroup = cluster
			prx.SetDataDir()
			prx.SetServiceName(cluster.Name, prx.Name)
			cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
			prx.State = stateSuspect
			cluster.Proxies[ctproxy] = prx
			if err != nil {
				cluster.LogPrintf(LvlErr, "Could not open connection to proxy %s %s: %s", prx.Host, prx.GetPort(), err)
			}

			ctproxy++
		}
	}
	if cluster.Conf.ExtProxyOn {
		prx := new(Proxy)
		prx.Type = config.ConstProxyExternal
		prx.Host, prx.Port = misc.SplitHostPort(cluster.Conf.ExtProxyVIP)
		prx.WritePort, _ = strconv.Atoi(prx.GetPort())
		prx.ReadPort = prx.WritePort
		prx.ReadWritePort = prx.WritePort
		if prx.Name == "" {
			prx.Name = prx.Host
		}
		prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
		prx.ClusterGroup = cluster
		prx.SetDataDir()
		prx.SetServiceName(cluster.Name, prx.Name)
		cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
		prx.State = stateSuspect
		cluster.Proxies[ctproxy] = prx
		ctproxy++
	}
	if cluster.Conf.ProxysqlOn {

		for k, proxyHost := range strings.Split(cluster.Conf.ProxysqlHosts, ",") {
			prx := NewProxySQLProxy(cluster.Name, proxyHost, cluster.Conf)
			prx.SetPlacement(k, cluster.Conf.ProvProxAgents, cluster.Conf.SlapOSProxySQLPartitions, cluster.Conf.ProxysqlHostsIPV6)

			if cluster.key != nil {
				p := crypto.Password{Key: cluster.key}
				p.CipherText = prx.Pass
				p.Decrypt()
				prx.Pass = p.PlainText
			}

			prx.ClusterGroup = cluster
			prx.SetDataDir()
			prx.SetServiceName(cluster.Name, prx.Name)
			cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
			prx.State = stateSuspect
			cluster.Proxies[ctproxy] = prx
			if err != nil {
				cluster.LogPrintf(LvlErr, "Could not open connection to proxy %s %s: %s", prx.Host, prx.GetPort(), err)
			}
			ctproxy++
		}
	}
	if cluster.Conf.MdbsProxyHosts != "" && cluster.Conf.MdbsProxyOn {
		for k, proxyHost := range strings.Split(cluster.Conf.MdbsProxyHosts, ",") {
			prx := new(MariadbShardProxy)
			prx.SetPlacement(k, cluster.Conf.ProvProxAgents, cluster.Conf.SlapOSShardProxyPartitions, cluster.Conf.MdbsHostsIPV6)
			prx.Type = config.ConstProxySpider
			prx.Host, prx.Port = misc.SplitHostPort(proxyHost)
			prx.User, prx.Pass = misc.SplitPair(cluster.Conf.MdbsProxyCredential)
			prx.ReadPort, _ = strconv.Atoi(prx.GetPort())
			prx.ReadWritePort, _ = strconv.Atoi(prx.GetPort())
			prx.Name = proxyHost
			if cluster.Conf.ProvNetCNI {
				if cluster.Conf.ClusterHead == "" {
					prx.Host = prx.Host + "." + cluster.Name + ".svc." + cluster.Conf.ProvOrchestratorCluster
				} else {
					prx.Host = prx.Host + "." + cluster.Conf.ClusterHead + ".svc." + cluster.Conf.ProvOrchestratorCluster
				}
				prx.Port = "3306"
			}
			prx.WritePort, _ = strconv.Atoi(prx.GetPort())
			prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
			prx.ClusterGroup = cluster
			prx.SetDataDir()
			prx.SetServiceName(cluster.Name, prx.Name)
			cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
			prx.State = stateSuspect
			cluster.Proxies[ctproxy] = prx
			if err != nil {
				cluster.LogPrintf(LvlErr, "Could not open connection to proxy %s %s: %s", prx.Host, prx.GetPort(), err)
			}
			cluster.LogPrintf(LvlDbg, "New MdbShardProxy proxy created: %s %s", prx.Host, prx.GetPort())
			ctproxy++
		}
	}
	if cluster.Conf.SphinxHosts != "" && cluster.Conf.SphinxOn {
		for k, proxyHost := range strings.Split(cluster.Conf.SphinxHosts, ",") {
			prx := new(SphinxProxy)
			prx.SetPlacement(k, cluster.Conf.ProvProxAgents, cluster.Conf.SlapOSSphinxPartitions, cluster.Conf.SphinxHostsIPV6)
			prx.Type = config.ConstProxySphinx

			prx.Port = cluster.Conf.SphinxQLPort
			prx.User = ""
			prx.Pass = ""
			prx.ReadPort, _ = strconv.Atoi(prx.GetPort())
			prx.WritePort, _ = strconv.Atoi(prx.GetPort())
			prx.ReadWritePort, _ = strconv.Atoi(prx.GetPort())
			prx.Name = proxyHost
			prx.Host = proxyHost
			if cluster.Conf.ProvNetCNI {
				prx.Host = prx.Host + "." + cluster.Name + ".svc." + cluster.Conf.ProvOrchestratorCluster
			}
			prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
			prx.ClusterGroup = cluster
			prx.SetDataDir()
			prx.SetServiceName(cluster.Name, prx.Name)
			cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
			prx.State = stateSuspect
			cluster.Proxies[ctproxy] = prx
			if err != nil {
				cluster.LogPrintf(LvlErr, "Could not open connection to proxy %s %s: %s", prx.Host, prx.GetPort(), err)
			}
			cluster.LogPrintf(LvlDbg, "New SphinxSearch proxy created: %s %s", prx.Host, prx.GetPort())
			ctproxy++
		}
	}
	if cluster.Conf.MyproxyOn {
		prx := new(MyProxyProxy)
		prx.Type = config.ConstProxyMyProxy
		prx.Port = strconv.Itoa(cluster.Conf.MyproxyPort)
		prx.Host = "0.0.0.0"
		prx.ReadPort = cluster.Conf.MyproxyPort
		prx.WritePort = cluster.Conf.MyproxyPort
		prx.ReadWritePort = cluster.Conf.MyproxyPort
		prx.User = cluster.Conf.MyproxyUser
		prx.Pass = cluster.Conf.MyproxyPassword
		if prx.Name == "" {
			prx.Name = prx.Host
		}
		prx.Id = "px" + strconv.FormatUint(crc64.Checksum([]byte(cluster.Name+prx.Name+":"+strconv.Itoa(prx.WritePort)), crcTable), 10)
		if prx.Host == "" {
			prx.Host = "repman." + cluster.Name + ".svc." + cluster.Conf.ProvOrchestratorCluster
		}
		prx.ClusterGroup = cluster
		prx.SetDataDir()
		prx.SetServiceName(cluster.Name, prx.Name)
		cluster.LogPrintf(LvlInfo, "New proxy monitored %s: %s:%s", prx.Type, prx.Host, prx.GetPort())
		prx.State = stateSuspect
		cluster.Proxies[ctproxy] = prx
		ctproxy++
	}

	return nil
}

func (cluster *Cluster) InjectProxiesTraffic() {
	var definer string
	// Found server from ServerId
	if cluster.GetMaster() != nil {
		for _, pr := range cluster.Proxies {
			if pr.GetType() == config.ConstProxySphinx || pr.GetType() == config.ConstProxyMyProxy {
				// Does not yet understand CREATE OR REPLACE VIEW
				continue
			}
			db, err := pr.GetCluster()
			if err != nil {
				cluster.sme.AddState("ERR00050", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00050"], err), ErrFrom: "TOPO"})
			} else {
				if pr.GetType() == config.ConstProxyMyProxy {
					definer = "DEFINER = root@localhost"
				} else {
					definer = ""
				}
				_, err := db.Exec("CREATE OR REPLACE " + definer + " VIEW replication_manager_schema.pseudo_gtid_v as select '" + misc.GetUUID() + "' from dual")

				if err != nil {
					cluster.sme.AddState("ERR00050", state.State{ErrType: "ERROR", ErrDesc: fmt.Sprintf(clusterError["ERR00050"], err), ErrFrom: "TOPO"})
					db.Exec("CREATE DATABASE IF NOT EXISTS replication_manager_schema")

				}
				db.Close()
			}
		}
	}
}

func (cluster *Cluster) IsProxyEqualMaster() bool {
	// Found server from ServerId
	if cluster.GetMaster() != nil {
		for _, pr := range cluster.Proxies {
			db, err := pr.GetCluster()
			if err != nil {
				if cluster.IsVerbose() {
					cluster.LogPrintf(LvlErr, "Can't get a proxy connection: %s", err)
				}
				return false
			}
			defer db.Close()
			var sv map[string]string
			sv, _, err = dbhelper.GetVariables(db, cluster.GetMaster().DBVersion)
			if err != nil {
				if cluster.IsVerbose() {
					cluster.LogPrintf(LvlErr, "Can't get variables: %s", err)
				}
				return false
			}
			var sid uint64
			sid, err = strconv.ParseUint(sv["SERVER_ID"], 10, 64)
			if err != nil {
				if cluster.IsVerbose() {
					cluster.LogPrintf(LvlErr, "Can't form proxy server_id convert: %s", err)
				}
				return false
			}
			if cluster.IsVerbose() {
				cluster.LogPrintf(LvlInfo, "Proxy compare master: %d %d", cluster.GetMaster().ServerID, uint(sid))
			}
			if cluster.GetMaster().ServerID == uint64(sid) || pr.GetType() == config.ConstProxySpider {
				return true
			}
		}
	}
	return false
}

func (cluster *Cluster) SetProxyServerMaintenance(serverid uint64) {
	// Found server from ServerId
	for _, pr := range cluster.Proxies {
		server := cluster.GetServerFromId(serverid)
		if cluster.Conf.HaproxyOn {
			if prx, ok := pr.(*HaproxyProxy); ok {
				if cluster.Conf.HaproxyMode == "runtimeapi" {
					prx.SetMaintenance(server)
				}
				if cluster.Conf.HaproxyMode == "standby" {
					prx.Init()
				}
			}
		}
		if cluster.Conf.MxsOn {
			if prx, ok := pr.(*MaxscaleProxy); ok {
				if cluster.GetMaster() != nil {
					prx.SetMaintenance(server)
				}
			}
		}
		if cluster.Conf.ProxysqlOn {
			if prx, ok := pr.(*ProxySQLProxy); ok {
				if cluster.GetMaster() != nil {
					prx.SetMaintenance(server)
				}
			}
		}
	}
	cluster.initConsul()
}

// called  by server monitor if state change
func (cluster *Cluster) backendStateChangeProxies() {
	cluster.initConsul()
}

// Used to monitor proxies call by main monitor loop
func (cluster *Cluster) refreshProxies(wcg *sync.WaitGroup) {
	defer wcg.Done()

	for _, pr := range cluster.Proxies {
		var err error
		err = pr.Refresh()
		if err == nil {
			pr.SetFailCount(0)
			pr.SetState(stateProxyRunning)
			if pr.HasWaitStartCookie() {
				pr.DelWaitStartCookie()
			}
		} else {
			fc := pr.GetFailCount() + 1
			// TODO: Can pr.ClusterGroup be different from cluster *Cluster? code doesn't imply it. if not change to
			// cl, err := pr.GetCluster()
			// cl.Conf.MaxFail
			if fc >= cluster.Conf.MaxFail {
				if fc == cluster.Conf.MaxFail {
					cluster.LogPrintf("INFO", "Declaring %s proxy as failed %s:%s %s", pr.GetType(), pr.GetHost(), pr.GetPort(), err)
				}
				pr.SetState(stateFailed)
				pr.DelWaitStopCookie()
				pr.DelRestartCookie()
			} else {
				pr.SetState(stateSuspect)
			}
		}
		if pr.GetPrevState() != pr.GetState() {
			pr.SetPrevState(pr.GetState())
		}
		if cluster.Conf.GraphiteMetrics {
			pr.SendStats()
		}
	}

}

func (cluster *Cluster) failoverProxies() {
	for _, pr := range cluster.Proxies {
		cluster.LogPrintf(LvlInfo, "Failover Proxy Type: %s Host: %s Port: %s", pr.GetType(), pr.GetHost(), pr.GetPort())
		pr.Failover()
	}
	cluster.initConsul()
}

// TODO: reduce to
// for { pr.Init() }
func (cluster *Cluster) initProxies() {
	for _, pr := range cluster.Proxies {
		cluster.LogPrintf(LvlInfo, "New proxy monitored: %s %s:%s", pr.GetType(), pr.GetHost(), pr.GetPort())
		pr.Init()
	}
	cluster.initConsul()
}

func (cluster *Cluster) SendProxyStats(proxy DatabaseProxy) error {
	return proxy.SendStats()
}

func (proxy *Proxy) SendStats() error {
	cluster := proxy.ClusterGroup
	graph, err := graphite.NewGraphite(cluster.Conf.GraphiteCarbonHost, cluster.Conf.GraphiteCarbonPort)
	if err != nil {
		return err
	}
	for _, wbackend := range proxy.BackendsWrite {
		var metrics = make([]graphite.Metric, 4)
		replacer := strings.NewReplacer("`", "", "?", "", " ", "_", ".", "-", "(", "-", ")", "-", "/", "_", "<", "-", "'", "-", "\"", "-", ":", "-")
		server := "rw-" + replacer.Replace(wbackend.PrxName)
		metrics[0] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.bytes_send", proxy.Type, proxy.Id, server), wbackend.PrxByteOut, time.Now().Unix())
		metrics[1] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.bytes_received", proxy.Type, proxy.Id, server), wbackend.PrxByteOut, time.Now().Unix())
		metrics[2] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.connections", proxy.Type, proxy.Id, server), wbackend.PrxConnections, time.Now().Unix())
		metrics[3] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.latency", proxy.Type, proxy.Id, server), wbackend.PrxLatency, time.Now().Unix())
		graph.SendMetrics(metrics)
	}
	for _, wbackend := range proxy.BackendsRead {
		var metrics = make([]graphite.Metric, 4)
		replacer := strings.NewReplacer("`", "", "?", "", " ", "_", ".", "-", "(", "-", ")", "-", "/", "_", "<", "-", "'", "-", "\"", "-", ":", "-")
		server := "ro-" + replacer.Replace(wbackend.PrxName)
		metrics[0] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.bytes_send", proxy.Type, proxy.Id, server), wbackend.PrxByteOut, time.Now().Unix())
		metrics[1] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.bytes_received", proxy.Type, proxy.Id, server), wbackend.PrxByteOut, time.Now().Unix())
		metrics[2] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.connections", proxy.Type, proxy.Id, server), wbackend.PrxConnections, time.Now().Unix())
		metrics[3] = graphite.NewMetric(fmt.Sprintf("proxy.%s%s.%s.latency", proxy.Type, proxy.Id, server), wbackend.PrxLatency, time.Now().Unix())
		graph.SendMetrics(metrics)
	}

	graph.Disconnect()

	return nil
}
