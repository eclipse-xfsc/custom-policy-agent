package regofunc

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

const (
	didSeparator   = ":"
	urlSeparator   = "/"
	defaultURLPath = ".well-known"
)

type DIDWebFuncs struct{}

type DID struct {
	scheme string // scheme is always "did"
	method string // method is the specific did method - "web" in this case
	path   string // path is the unique URI assigned by the DID method
}

func NewDIDWebFuncs() *DIDWebFuncs {
	return &DIDWebFuncs{}
}

func (dw *DIDWebFuncs) DIDToURLFunc() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "did_to_url",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
			var did string

			if err := ast.As(a.Value, &did); err != nil {
				return nil, fmt.Errorf("invalid DID: %s", err)
			}
			if did == "" {
				return nil, errors.New("DID cannot be empty")
			}

			u, err := dw.didToURL(did)
			if err != nil {
				return nil, err
			}

			return ast.StringTerm(u.String()), nil
		}
}

func (dw *DIDWebFuncs) URLToDIDFunc() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "url_to_did",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {
			var u string

			if err := ast.As(a.Value, &u); err != nil {
				return nil, fmt.Errorf("invalid URL: %s", err)
			}
			if u == "" {
				return nil, errors.New("URL cannot be empty")
			}
			uri, err := url.Parse(u)
			if err != nil {
				return nil, errors.New("cannot parse URL")
			}
			if uri.Host == "" || uri.Scheme != "https" {
				return nil, errors.New("invalid URL for did:web method")
			}

			did := dw.urlToDID(uri)

			return ast.StringTerm(did.String()), nil
		}
}

// didToURL transforms a valid DID, created by the "did:web" Method Specification, to a URL.
// Documentation can be found here: https://w3c-ccg.github.io/did-method-web/
func (dw *DIDWebFuncs) didToURL(DID string) (*url.URL, error) {
	ss := strings.Split(DID, didSeparator)
	if len(ss) < 3 {
		return nil, errors.New("invalid DID, host is not found")
	}
	if ss[0] != "did" || ss[1] != "web" {
		return nil, errors.New("invalid DID, method is unknown")
	}

	path := defaultURLPath
	if len(ss) > 3 {
		path = ""
		for i := 3; i < len(ss); i++ {
			path = path + urlSeparator + ss[i]
		}
	}
	path = path + urlSeparator + "did.json"

	host, err := url.PathUnescape(ss[2])
	if err != nil {
		return nil, errors.New("failed to url decode host from DID")
	}

	return &url.URL{
		Scheme: "https",
		Host:   host,
		Path:   path,
	}, nil
}

// urlToDID transforms a valid URL to a DID created following the "did:web" Method Specification.
// Documentation can be found here: https://w3c-ccg.github.io/did-method-web/
func (dw *DIDWebFuncs) urlToDID(uri *url.URL) *DID {
	p := strings.TrimSuffix(uri.Path, "did.json")
	sp := strings.Split(p, urlSeparator)

	path := url.QueryEscape(uri.Host)
	for _, v := range sp {
		if v == defaultURLPath {
			break
		}
		if v == "" {
			continue
		}
		path = path + didSeparator + url.QueryEscape(v)
	}

	return &DID{
		scheme: "did",
		method: "web",
		path:   strings.Trim(path, didSeparator),
	}
}

// String returns a string representation of this DID.
func (d *DID) String() string {
	return fmt.Sprintf("%s:%s:%s", d.scheme, d.method, d.path)
}
