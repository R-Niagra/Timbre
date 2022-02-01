package noderunner

import "github.com/guyu96/go-timbre/net"

//GetDaemonNode returns the daemon node
func GetDaemonNode() *net.Node {
	return daemonNode
}
