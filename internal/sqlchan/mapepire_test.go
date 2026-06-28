// SPDX-License-Identifier: GPL-3.0-or-later

package sqlchan

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

// mockTunnel dials a real TCP address (the test server) instead of going through SSH.
type mockTunnel struct{ addr string }

func (m mockTunnel) DialTunnel(_ string) (net.Conn, error) { return net.Dial("tcp", m.addr) }

// fakeMapepire is a plaintext-WebSocket server speaking the subset of the
// mapepire protocol the client uses. It echoes canned responses by request type.
func fakeMapepire(t *testing.T, handler func(req reqMsg) respMsg) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var req reqMsg
			if err := json.Unmarshal(data, &req); err != nil {
				return
			}
			resp := handler(req)
			resp.ID = req.ID
			out, _ := json.Marshal(resp)
			if err := c.Write(ctx, websocket.MessageText, out); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// dialFake connects the Mapepire client to a plaintext test server (no TLS).
func dialFake(t *testing.T, srv *httptest.Server) *Mapepire {
	t.Helper()
	addr := srv.Listener.Addr().String()
	ws, _, err := websocket.Dial(context.Background(), "ws://"+addr+"/db/", &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return mockTunnel{addr}.DialTunnel(addr)
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("dial fake: %v", err)
	}
	return &Mapepire{ws: ws, opts: MapepireOptions{Timeout: defaultTestTimeout}}
}

const defaultTestTimeout = 5_000_000_000 // 5s

func TestMapepireQuery(t *testing.T) {
	srv := fakeMapepire(t, func(req reqMsg) respMsg {
		if req.Type != "sql" {
			return respMsg{Success: false, Error: "unexpected type"}
		}
		return respMsg{
			Success:    true,
			HasResults: true,
			Data: []json.RawMessage{
				json.RawMessage(`{"ID":1,"NAME":"hola"}`),
				json.RawMessage(`{"ID":2,"NAME":"chau"}`),
			},
		}
	})
	m := dialFake(t, srv)
	defer m.Close()

	rows, err := m.Query("SELECT * FROM T")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 2 || asString(rows[0]["NAME"]) != "hola" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestMapepireExec(t *testing.T) {
	srv := fakeMapepire(t, func(req reqMsg) respMsg {
		return respMsg{Success: true}
	})
	m := dialFake(t, srv)
	defer m.Close()

	if err := m.Exec("CREATE TABLE T (ID INT)"); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestMapepireSQLError(t *testing.T) {
	srv := fakeMapepire(t, func(req reqMsg) respMsg {
		return respMsg{Success: false, Error: "object not found", SQLState: "42704"}
	})
	m := dialFake(t, srv)
	defer m.Close()

	_, err := m.Query("SELECT * FROM NOPE")
	var se *SQLError
	if !errors.As(err, &se) {
		t.Fatalf("expected *SQLError, got %T: %v", err, err)
	}
	if se.Detail == "" || !contains(se.Detail, "42704") {
		t.Errorf("detail = %q, want SQLSTATE 42704", se.Detail)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
