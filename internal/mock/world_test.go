package mock

import "testing"

// REQ: MOCK-3
// Port 0 means "any free port" for every demo API, not 0, 1, 2, 3, which
// only root can bind and which failed CI as soon as it ran as a normal user.
func TestStart_PortZeroPicksFreePortsForEveryAPI(t *testing.T) {
	w, err := Start(0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	seen := map[int]string{}
	for name, port := range w.Ports {
		if port < 1024 {
			t.Errorf("%s got port %d; want an ephemeral port", name, port)
		}
		if other, ok := seen[port]; ok {
			t.Errorf("%s and %s share port %d", name, other, port)
		}
		seen[port] = name
	}
	if len(w.Ports) != 5 {
		t.Fatalf("want 5 demo APIs, got %v", w.Ports)
	}
}
