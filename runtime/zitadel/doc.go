// Package zitadel provides helpers for calling the ZITADEL Management and
// v2 Connect RPC APIs from consumer applications built on Gofra.
//
// This package is consumer-facing: the generated starter does not import
// it. Use it from application code that needs to provision organizations,
// users, projects, or applications in ZITADEL on behalf of the deployment.
//
// The companion package [github.com/Gabrielbdd/gofra/runtime/zitadel/secret]
// reads a Personal Access Token from a file path or environment variable
// and is designed to pair with this package's [NewAuthInterceptor].
//
// # Scope
//
// This package deliberately does not bundle generated Connect service
// clients (Organization, User, Project, Application). Consumer apps pick
// their preferred source for those stubs — the upstream ZITADEL .proto
// files compiled with buf, the zitadel-go module, or hand-written wrappers.
// Gofra stays out of that decision so ZITADEL module cadence does not drive
// framework releases.
//
// # Example
//
//	pat, err := zitadelsecret.Read(zitadelsecret.Source{
//	    FilePath: os.Getenv("APP_ZITADEL_PROVISIONER_PAT_FILE"),
//	})
//	if err != nil {
//	    return err
//	}
//	interceptor := zitadel.NewAuthInterceptor(pat, "")
//	orgs := orgv2connect.NewOrganizationServiceClient(
//	    http.DefaultClient,
//	    "http://localhost:8081",
//	    connect.WithInterceptors(interceptor),
//	)
package zitadel
