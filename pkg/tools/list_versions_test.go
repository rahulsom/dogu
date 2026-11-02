package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListVersionsNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.tab" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte("version\tdate\nv20.1.0\tdate\nv22.0.0\tdate\nv20.1.0\tdate\n"))
	}))
	defer server.Close()
	got, err := ListVersions(context.Background(), "node", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"v22.0.0", "v20.1.0"}) {
		t.Fatalf("got %v", got)
	}
}
func TestListVersionsErrors(t *testing.T) {
	if _, err := ListVersions(context.Background(), "unknown", ""); err == nil {
		t.Fatal("expected unknown tool error")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	if _, err := ListVersions(context.Background(), "node", server.URL); err == nil {
		t.Fatal("expected HTTP error")
	}
}
