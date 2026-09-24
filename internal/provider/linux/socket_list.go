package linux

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// socketListTask implements the linux.socket.list capability.
type socketListTask struct{ provider *Provider }

const socketListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Socket List Parameters",
  "description": "Parameters for listing network sockets.",
  "properties": {
    "protocol": {
      "type": "string",
      "description": "Filter by protocol (tcp, tcp6, udp, unix).",
      "enum": ["tcp", "tcp6", "udp", "unix"]
    },
    "state": {
      "type": "string",
      "description": "Filter by TCP state (e.g., LISTEN, ESTABLISHED)."
    }
  }
}`

func (t *socketListTask) Name() string       { return "linux.socket.list" }
func (t *socketListTask) JSONSchema() string { return socketListSchema }

func (t *socketListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	slog.Info("socket.list starting", "capability", t.Name())

	protocolFilter, _ := params["protocol"].(string)
	stateFilter, _ := params["state"].(string)

	reader := t.provider.CurrentReader()

	sockets, err := listSockets(reader, protocolFilter, stateFilter)
	if err != nil {
		slog.Info("socket.list failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"sockets": sockets,
		"count":   len(sockets),
	}
	slog.Info("socket.list succeeded", "capability", t.Name(), "count", len(sockets))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*socketListTask)(nil)

// tcpStateMap maps hex TCP state values to human-readable names.
var tcpStateMap = map[string]string{
	"01": "ESTABLISHED",
	"02": "SYN_SENT",
	"03": "SYN_RECV",
	"04": "FIN_WAIT1",
	"05": "FIN_WAIT2",
	"06": "TIME_WAIT",
	"07": "CLOSE",
	"08": "CLOSE_WAIT",
	"09": "LAST_ACK",
	"0A": "LISTEN",
	"0B": "CLOSING",
}

func tcpState(st string) string {
	st = strings.ToUpper(st)
	if s, ok := tcpStateMap[st]; ok {
		return s
	}
	return st
}

func listSockets(r *procfs.Reader, protocolFilter, stateFilter string) ([]SocketInfo, error) {
	var sockets []SocketInfo

	if protocolFilter == "" || protocolFilter == "tcp" {
		s, err := parseTCP(r.Path("proc", "net", "tcp"), false)
		if err == nil {
			sockets = append(sockets, s...)
		}
	}
	if protocolFilter == "" || protocolFilter == "tcp6" {
		s, err := parseTCP(r.Path("proc", "net", "tcp6"), true)
		if err == nil {
			sockets = append(sockets, s...)
		}
	}
	if protocolFilter == "" || protocolFilter == "udp" {
		s, err := parseUDP(r.Path("proc", "net", "udp"), false)
		if err == nil {
			sockets = append(sockets, s...)
		}
		s6, err := parseUDP(r.Path("proc", "net", "udplite"), false)
		if err == nil {
			for i := range s6 {
				s6[i].Protocol = "udplite"
			}
			sockets = append(sockets, s6...)
		}
	}
	if protocolFilter == "" || protocolFilter == "unix" {
		s, err := parseUnix(r.Path("proc", "net", "unix"))
		if err == nil {
			sockets = append(sockets, s...)
		}
	}

	if stateFilter != "" {
		stateFilter = strings.ToUpper(stateFilter)
		var filtered []SocketInfo
		for _, s := range sockets {
			if strings.EqualFold(s.State, stateFilter) {
				filtered = append(filtered, s)
			}
		}
		sockets = filtered
	}

	return sockets, nil
}

func parseTCP(path string, ipv6 bool) ([]SocketInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	proto := "tcp"
	if ipv6 {
		proto = "tcp6"
	}

	var sockets []SocketInfo
	// Skip header line.
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		localAddr, localPort, err := parseHexAddressPort(fields[1])
		if err != nil {
			continue
		}
		remoteAddr, remotePort, err := parseHexAddressPort(fields[2])
		if err != nil {
			continue
		}

		state := tcpState(fields[3])
		inode := uint64(0)
		uid := 0
		if len(fields) >= 10 {
			inode, _ = strconv.ParseUint(fields[9], 10, 64)
		}
		if len(fields) >= 8 {
			uid, _ = strconv.Atoi(fields[7])
		}

		sockets = append(sockets, SocketInfo{
			Protocol:      proto,
			LocalAddress:  localAddr,
			LocalPort:     localPort,
			RemoteAddress: remoteAddr,
			RemotePort:    remotePort,
			State:         state,
			Inode:         inode,
			UID:           uid,
		})
	}
	return sockets, nil
}

func parseUDP(path string, ipv6 bool) ([]SocketInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	proto := "udp"
	if ipv6 {
		proto = "udp6"
	}

	var sockets []SocketInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		localAddr, localPort, err := parseHexAddressPort(fields[1])
		if err != nil {
			continue
		}
		remoteAddr, remotePort, err := parseHexAddressPort(fields[2])
		if err != nil {
			continue
		}

		state := tcpState(fields[3])
		if state == "" || state == "0" {
			state = "UNCONN"
		}
		inode := uint64(0)
		uid := 0
		if len(fields) >= 10 {
			inode, _ = strconv.ParseUint(fields[9], 10, 64)
		}
		if len(fields) >= 8 {
			uid, _ = strconv.Atoi(fields[7])
		}

		sockets = append(sockets, SocketInfo{
			Protocol:      proto,
			LocalAddress:  localAddr,
			LocalPort:     localPort,
			RemoteAddress: remoteAddr,
			RemotePort:    remotePort,
			State:         state,
			Inode:         inode,
			UID:           uid,
		})
	}
	return sockets, nil
}

func parseUnix(path string) ([]SocketInfo, error) {
	lines, err := readLinesFromPath(path)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}

	var sockets []SocketInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// unix format: Num RefCount Protocol Flags Type St Inode Path
		state := tcpState(fields[5])
		if state == "" {
			state = fields[5]
		}
		inode, _ := strconv.ParseUint(fields[6], 10, 64)
		sockPath := ""
		if len(fields) > 7 {
			sockPath = strings.Join(fields[7:], " ")
		}

		localAddr := sockPath
		if localAddr == "" {
			localAddr = fmt.Sprintf("inode:%d", inode)
		}

		sockets = append(sockets, SocketInfo{
			Protocol:     "unix",
			LocalAddress: localAddr,
			State:        state,
			Inode:        inode,
		})
	}
	return sockets, nil
}

func parseHexAddressPort(s string) (string, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid address:port format")
	}

	port64, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, fmt.Errorf("parse port: %w", err)
	}
	port := int(port64)

	addrHex := parts[0]
	if len(addrHex) == 8 {
		// IPv4: reverse byte order
		addrBytes, err := strconv.ParseUint(addrHex, 16, 32)
		if err != nil {
			return "", 0, fmt.Errorf("parse address: %w", err)
		}
		addr := net.IPv4(byte(addrBytes), byte(addrBytes>>8), byte(addrBytes>>16), byte(addrBytes>>24))
		return addr.String(), port, nil
	}

	if len(addrHex) == 32 {
		// IPv6
		var addr net.IP = make(net.IP, 16)
		for i := 0; i < 16; i++ {
			b, err := strconv.ParseUint(addrHex[i*2:(i+1)*2], 16, 8)
			if err != nil {
				return "", 0, fmt.Errorf("parse ipv6 address: %w", err)
			}
			addr[i] = byte(b)
		}
		// /proc/net/tcp6 stores in network byte order groups of 4 bytes reversed
		// Actually let's just do the simple reverse-of-4-byte-groups
		for i := 0; i < 4; i++ {
			group := addr[i*4 : (i+1)*4]
			for j := 0; j < 2; j++ {
				group[j], group[3-j] = group[3-j], group[j]
			}
		}
		return addr.String(), port, nil
	}

	return "", 0, fmt.Errorf("unknown address length")
}

func readLinesFromPath(path string) ([]string, error) {
	data, err := procfs.DefaultReader.ReadFileString(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(data), "\n"), nil
}
