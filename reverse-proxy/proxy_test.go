package reverse_proxy

import "testing"

func TestParseEndpointFromGrpcRequestPath(t *testing.T) {
	expected := "com.veigit.dimpocp.fsosi.grpc.v1"
	fullPath := "/com.veigit.dimpocp.fsosi.grpc.v1.Fsosi/ListFso"
	endpoint, err := ParseEndpointFromGrpcRequestPath(fullPath)
	t.Logf("fullpath: %v", fullPath)
	t.Logf("parsed endpoint: %v", endpoint)
	if err != nil {
		t.Error(err)
	}
	if endpoint != expected {
		t.Errorf("expected: %v, but got %v", expected, endpoint)
	}
}
