// SPDX-License-Identifier: GPL-3.0-or-later

package sqlchan

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// Tunnel opens a TCP connection to an address as seen from the IBM i host
// (satisfied by transport.SSH.DialTunnel). mapepire usually binds localhost, so
// it is reached through the SSH connection rather than a public port.
type Tunnel interface {
	DialTunnel(addr string) (net.Conn, error)
}

// MapepireOptions configures a mapepire connection.
type MapepireOptions struct {
	Tunnel   Tunnel // how to reach the daemon (SSH direct-tcpip)
	Host     string // host name as the daemon sees it (default "localhost")
	Port     int    // daemon port (default 8076)
	User     string
	Password string
	Insecure bool          // accept the daemon's self-signed cert (default true)
	Timeout  time.Duration // per-request timeout (default 30s)
}

// Mapepire is a Conn backed by the mapepire-server WebSocket protocol.
type Mapepire struct {
	ws   *websocket.Conn
	opts MapepireOptions
	seq  int
}

// reqMsg is a mapepire request.
type reqMsg struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	SQL  string `json:"sql,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// respMsg is a mapepire response (subset we use).
type respMsg struct {
	ID         string            `json:"id"`
	Success    bool              `json:"success"`
	Error      string            `json:"error"`
	SQLState   string            `json:"sql_state"`
	HasResults bool              `json:"has_results"`
	Data       []json.RawMessage `json:"data"`
}

// DialMapepire opens a mapepire connection over the tunnel.
func DialMapepire(ctx context.Context, opts MapepireOptions) (*Mapepire, error) {
	if opts.Tunnel == nil {
		return nil, fmt.Errorf("mapepire: a tunnel is required")
	}
	if opts.Host == "" {
		opts.Host = "localhost"
	}
	if opts.Port == 0 {
		opts.Port = 8076
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))

	// HTTP client whose TLS dial goes through the SSH tunnel.
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				raw, err := opts.Tunnel.DialTunnel(addr)
				if err != nil {
					return nil, err
				}
				tlsConn := tls.Client(raw, &tls.Config{
					ServerName:         opts.Host,
					InsecureSkipVerify: opts.Insecure || true, // daemon ships a self-signed cert
				})
				if err := tlsConn.HandshakeContext(ctx); err != nil {
					raw.Close()
					return nil, fmt.Errorf("mapepire TLS handshake: %w", err)
				}
				return tlsConn, nil
			},
		},
	}

	auth := base64.StdEncoding.EncodeToString([]byte(opts.User + ":" + opts.Password))
	ws, _, err := websocket.Dial(ctx, "wss://"+addr+"/db/", &websocket.DialOptions{
		HTTPClient: httpClient,
		HTTPHeader: http.Header{"Authorization": {"Basic " + auth}},
	})
	if err != nil {
		return nil, fmt.Errorf("mapepire connect: %w", err)
	}

	m := &Mapepire{ws: ws, opts: opts}
	if err := m.handshake(ctx); err != nil {
		_ = ws.Close(websocket.StatusInternalError, "handshake failed")
		return nil, err
	}
	return m, nil
}

// handshake establishes the DB connection within the mapepire session. The
// daemon requires a "connect" message before any SQL ("Not connected" otherwise).
func (m *Mapepire) handshake(ctx context.Context) error {
	m.seq++
	req := reqMsg{ID: "bindle-connect-" + strconv.Itoa(m.seq), Type: "connect"}
	if err := writeJSON(ctx, m.ws, req); err != nil {
		return fmt.Errorf("mapepire connect send: %w", err)
	}
	var resp respMsg
	if err := readJSON(ctx, m.ws, &resp); err != nil {
		return fmt.Errorf("mapepire connect recv: %w", err)
	}
	if !resp.Success {
		return &SQLError{Stmt: "<connect>", Detail: firstNonEmpty(resp.Error, "connect rejected")}
	}
	return nil
}

// Exec runs a statement and discards rows.
func (m *Mapepire) Exec(stmt string) error {
	_, err := m.sql(stmt, 0)
	return err
}

// Query runs a statement and returns its rows.
func (m *Mapepire) Query(stmt string) ([]Row, error) {
	return m.sql(stmt, 2147483647)
}

func (m *Mapepire) sql(stmt string, rows int) ([]Row, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.opts.Timeout)
	defer cancel()

	m.seq++
	req := reqMsg{ID: "bindle-" + strconv.Itoa(m.seq), Type: "sql", SQL: stmt, Rows: rows}
	if err := writeJSON(ctx, m.ws, req); err != nil {
		return nil, fmt.Errorf("mapepire send: %w", err)
	}
	var resp respMsg
	if err := readJSON(ctx, m.ws, &resp); err != nil {
		return nil, fmt.Errorf("mapepire recv: %w", err)
	}
	if !resp.Success {
		detail := resp.Error
		if resp.SQLState != "" {
			detail = "SQLSTATE=" + resp.SQLState + " " + detail
		}
		return nil, &SQLError{Stmt: stmt, Detail: detail}
	}
	out := make([]Row, 0, len(resp.Data))
	for _, raw := range resp.Data {
		var r Row
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("mapepire row decode: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// SetSchema sets the connection's current schema so unqualified object names in
// migrations resolve there. mapepire defaults to *SQL naming (schema = user), so
// this is required for migrations targeting another library.
func (m *Mapepire) SetSchema(schema string) error {
	return m.Exec("SET CURRENT SCHEMA " + schema)
}

// Close ends the WebSocket connection.
func (m *Mapepire) Close() error {
	return m.ws.Close(websocket.StatusNormalClosure, "bye")
}

func writeJSON(ctx context.Context, ws *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return ws.Write(ctx, websocket.MessageText, data)
}

func readJSON(ctx context.Context, ws *websocket.Conn, v any) error {
	_, data, err := ws.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
