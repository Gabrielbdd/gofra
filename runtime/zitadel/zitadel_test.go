package zitadel_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Gabrielbdd/gofra/runtime/zitadel"
)

func TestNewAuthInterceptor_UnarySetsHeaders(t *testing.T) {
	interceptor := zitadel.NewAuthInterceptor("test-pat", "org-123")

	var gotAuth, gotOrg string
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		gotAuth = req.Header().Get("Authorization")
		gotOrg = req.Header().Get("x-zitadel-orgid")
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	wrapped := interceptor.WrapUnary(next)
	_, err := wrapped(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}

	if want := "Bearer test-pat"; gotAuth != want {
		t.Errorf("Authorization = %q; want %q", gotAuth, want)
	}
	if gotOrg != "org-123" {
		t.Errorf("x-zitadel-orgid = %q; want org-123", gotOrg)
	}
}

func TestNewAuthInterceptor_UnaryOmitsOrgWhenEmpty(t *testing.T) {
	interceptor := zitadel.NewAuthInterceptor("pat", "")

	var sawOrgHeader bool
	next := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		_, sawOrgHeader = req.Header()["X-Zitadel-Orgid"]
		return connect.NewResponse(&emptypb.Empty{}), nil
	}

	wrapped := interceptor.WrapUnary(next)
	_, err := wrapped(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	if err != nil {
		t.Fatalf("wrapped: %v", err)
	}

	if sawOrgHeader {
		t.Error("x-zitadel-orgid should be absent when OrgID is empty")
	}
}
