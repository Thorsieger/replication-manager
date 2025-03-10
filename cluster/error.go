// replication-manager - Replication Manager Monitoring and CLI for MariaDB and MySQL
// Copyright 2017-2021 SIGNAL18 CLOUD SAS
// Authors: Guillaume Lefranc <guillaume@signal18.io>
//
//	Stephane Varoqui  <svaroqui@gmail.com>
//
// This source code is licensed under the GNU General Public License, version 3.
// Redistribution/Reuse of this code is permitted under the GNU v3 license, as
// an additional term, ALL code must carry the original Author(s) credit in comment form.
// See LICENSE in this directory for the integral text.
package cluster

var clusterError = map[string]string{
	"ERR00001": "Monitor freeze while running critical section",
	"ERR00002": "Waiting for a user manual failover",
	"ERR00004": "Database %s access denied: %s",
	"ERR00005": "Could not get privileges for user %s@%s: %s",
	"ERR00006": "User must have REPLICATION CLIENT privilege on %s",
	"ERR00007": "User must have REPLICATION SLAVE privilege on %s",
	"ERR00008": "User must have SUPER privilege %s",
	"ERR00009": "User must have RELOAD privilege %s",
	"ERR00010": "Could not find a slave in topology",
	"ERR00011": "Found multiple masters in topology but not explicitely setup",
	"ERR00012": "Could not find a master in topology",
	"ERR00013": "Binary log disabled on slave: %s",
	"ERR00014": "Could not get binlog dump count on server %s: %s",
	"ERR00015": "Could not get privileges for user %s on server %s: %s",
	"ERR00016": "Master is unreachable but slaves are replicating",
	"ERR00017": "Unable to fetch MaxScale monitoring information",
	"ERR00018": "Could not connect to MaxScale: %s",
	"ERR00019": "Could not get MaxScale maxadmin server list: %s",
	"ERR00020": "Could not get MaxScale maxinfo server list: %s",
	"ERR00021": "Cluster state down",
	"ERR00022": "Running in passive mode",
	"ERR00023": "Constraint failed: state %s, conf.Interactive %t cluster.isMaxMasterFailedCountReach %t",
	"ERR00024": "Constraint failed: isExternalOk %t,isActiveArbitration %t,isBeetwenFailoverTimeTooShort %t ,isMaxClusterFailoverCountReach %t, isOneSlaveHeartbeatIncreasing %t, isMaxscaleSupectRunning %t",
	"ERR00025": "Could not get MaxScale maxinfo server list: %s",
	"ERR00026": "First node restarted is a slave, non-interactive mode",
	"ERR00027": "Number of cluster failovers exceeded",
	"ERR00028": "Slave %s can still communicate with the master",
	"ERR00029": "Time between failovers too short",
	"ERR00030": "Maxscale %s can still communicate with the master",
	"ERR00031": "External API can still communicate with the master",
	"ERR00032": "No candidates found in slaves list",
	"ERR00033": "Skip slave in election %s have no master log file, slave might have failed",
	"ERR00034": "Skip slave in election %s repl not electable for switchover",
	"ERR00035": "Skip slave in election %s multi-master and is already the master",
	"ERR00036": "Skip slave in election %s is relay",
	"ERR00037": "Skip slave in election %s in ignore list",
	"ERR00038": "Skip slave in election %s repl not electable for failover",
	"ERR00039": "Skip slave in election %s repl not electable",
	"ERR00040": "Skip slave in election %s does not ping or has no binlogs",
	"ERR00041": "Skip slave in election %s has more than %d seconds of replication delay (%d)",
	"ERR00042": "Skip slave in election %s SQL Thread is stopped",
	"ERR00043": "Skip slave in election %s Semisync report unsynced",
	"ERR00044": "Can't connect to OpenSVC collector %s",
	"ERR00045": "Found forbidden relay topology, trying to fix",
	"ERR00046": "Can't fix relay topology: high replication delay",
	"ERR00047": "Skip slave in election %s - Maintenance mode",
	"ERR00048": "Broken muti master ring",
	"ERR00049": "Waiting old master to rejoin in positional mode to rejoin slave",
	"ERR00050": "Can't connect to proxy %s",
	"ERR00051": "ProxySQL connection error: %s",
	"ERR00052": "Could not get stats to HaProxy: %s",
	"ERR00053": "ProxySQL can't load users (%s)",
	"ERR00054": "ProxySQL can't add user (%s)",
	"ERR00055": "Arbitrator unreachable (%s)",
	"ERR00056": "Master user %s is not defined on replication candidate %s",
	"ERR00057": "Database duplicate users not allowed in proxysql %s",
	"ERR00058": "Sphinx connection error: %s",
	"ERR00059": "Ignored server %s not found in configured server list",
	"ERR00060": "To many non closed task in scheduler, donor may not work on server %s",
	"ERR00061": "No user:password credential specified",
	"ERR00062": "DNS resolution for host %s error %s",
	"ERR00063": "Extra master in master slave topology rejoin it after split brain",
	"ERR00064": "Server %s is not a slave of declared master %s, and replication no relay is enable: Pointing to %s",
	"ERR00065": "No crash found on current master when rejoining slave %s to %s",
	"ERR00066": "No crash found on current master when rejoining standalone %s to %s",
	"ERR00067": "Found slave to rejoin %s slave was previously in state %s replication io thread  %s, pointing currently to %s",
	"ERR00068": "Arbitration looser",
	"ERR00069": "ProxySQL could not set %s as reader (%s) different state OFFLINE_HARD",
	"ERR00070": "ProxySQL could not set %s as reader (%s) different state ONLINE",
	"ERR00071": "ProxySQL could not set discoverd master %s as writer (%s)",
	"ERR00072": "ProxySQL could not set discoverd slave %s as reader (%s)",
	"ERR00073": "Could not get events on server %s",
	"ERR00074": "Prefered server %s not found in configured server list",
	"ERR00075": "Can't fecth Processlist %s",
	"ERR00076": "Connections reach 80 pourcent threshold: %s",
	"ERR00077": "All databases state down",
	"ERR00078": "Could not resolve IP from connection %s@%s: with hostname %s on server %s",
	"ERR00079": "Disk %s usage high on %s",
	"ERR00080": "Connection use old TLS keys on %s",
	"ERR00081": "Connection use no TLS keys on %s",
	"ERR00082": "Could not get agents from orchestrator %s",
	"ERR00083": "Different cluster uuid found on %s:%s %s:%s",
	"ERR00084": "Cluster have no master when slave %s was started",
	"ERR00085": "No replica found for routing reads",
	"ERR00086": "Sharding proxy refresh no database monitor yet initialize",
	"ERR00087": "Skip slave in election %s IO Thread is stopped with valid leader",
	"ERR00088": "Authentification error in replication IO thread",
	"ERR00089": "Authentification error to Vault %s",
	"ERR00090": "Monitoring save config enable but no encryption key for password, see the keygen command",
	"WARN0022": "Rejoining standalone server %s to master %s",
	"WARN0023": "Number of failed master ping has been reached",
	"WARN0045": "Provision task is in queue",
	"WARN0046": "Provision task is waiting",
	"WARN0047": "Entreprise provision of MariaDB Sharding Cluster not yet implemented",
	"WARN0048": "No semisync settings on slave %s",
	"WARN0049": "No binlog format ROW on slave %s and flashback activated",
	"WARN0050": "No Heartbeat <= 1s on slave %s",
	"WARN0051": "No GTID replication on slave %s",
	"WARN0052": "No InnoDB durability on slave %s",
	"WARN0053": "No replication checksum on slave %s",
	"WARN0054": "No log of replication queries in slow query on slave %s",
	"WARN0055": "ROW or MIXED binlog format and replicate_annotate_row_events is off on slave %s",
	"WARN0056": "No compression of binlog on slave %s",
	"WARN0057": "No log-slave-updates on slave %s",
	"WARN0058": "No GTID strict mode on slave %s",
	"WARN0059": "No replication crash-safe settings on slave %s",
	"WARN0060": "No semisync settings on master %s",
	"WARN0061": "No binlog format ROW on master %s and flashback activated",
	"WARN0062": "No Heartbeat <= 1s on master %s",
	"WARN0064": "No InnoDB durability on master %s",
	"WARN0065": "No replication checksum on master %s",
	"WARN0066": "No log of replication queries in slow query on master %s",
	"WARN0067": "RBR is on and Binlog Annotation is off on master %s",
	"WARN0068": "No compression of binlog on slave %s",
	"WARN0069": "No log-slave-updates on master %s",
	"WARN0070": "No GTID strict mode on master %s",
	"WARN0071": "No replication crash-safe settings on master %s",
	"WARN0072": "Running optimize table %s on server %s",
	"WARN0073": "Running physical backup %s on server %s",
	"WARN0074": "Reseeding physical backup %s on server %s",
	"WARN0075": "Reseeding logical backup %s on server %s",
	"WARN0076": "Flashback physical backup %s on server %s",
	"WARN0077": "Flashback logical backup %s on server %s",
	"WARN0078": "Haproxy version to old to get statistics",
	"WARN0079": "Cluster is split brain",
	"WARN0080": "Cluster lost majority",
	"WARN0081": "Cluster arbitrator error in reporting %s",
	"WARN0082": "Cluster arbitrator error in arbitration %s",
	"WARN0083": "Arbitration winner",
	"WARN0084": "Variable diff:\n %s",
	"WARN0085": "Server %s in capture mode",
	"WARN0086": "Checksum table waiting replication sync on slave %s",
	"WARN0087": "Cluster same server_id %s %s",
	"WARN0088": "High number of slow queries %s ",
	"WARN0089": "ShardProxy Could not fetch master schemas %s",
	"WARN0090": "Cluster arbitrator unreachable %s",
	"WARN0091": "Server as errant transaction %s",
	"WARN0092": "ProxySQL could not load query rules from runtime (%s)",
	"WARN0093": "Restic fetch repo issue: %s\n%s\n%s",
	"WARN0094": "Restic purge repo issue: %s\n%s\n%s",
	"WARN0095": "Restic init repo issue: %s\n%s\n%s",
	"WARN0096": "Restart database server via job request %s",
	"WARN0097": "Stop database server via job request %s",
	"WARN0098": "ProxySQL could not load global variables from runtime (%s)",
	"WARN0099": "MariaDB version as replication issue https://jira.mariadb.org/browse/MDEV-20821",
	"WARN0100": "No space left on device pn %s",
	"WARN0101": "Cluster does not have backup",
	"WARN0102": "The config file must be merge because an immutable parameter has been changed. Use the config-merge command to save your changes.",
	"WARN0103": "Enforce replication mode idempotent but  strict on server %s",
	"WARN0104": "Enforce replication mode strict but idempotent on server %s",
}
