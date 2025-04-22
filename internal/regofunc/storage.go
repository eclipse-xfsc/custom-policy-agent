package regofunc

import (
	"context"
	"fmt"
	"strings"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
)

//go:generate counterfeiter . Storage

type Storage interface {
	GetData(ctx context.Context, key string) (any, error)
	SetData(ctx context.Context, key string, data map[string]interface{}) error
	DeleteData(ctx context.Context, key string) error
}

type StorageFuncs struct {
	storage Storage
}

func NewStorageFuncs(storage Storage) *StorageFuncs {
	return &StorageFuncs{storage: storage}
}

func (sf *StorageFuncs) GetData() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "storage.get",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aKey *ast.Term) (*ast.Term, error) {
			var key string

			if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key: %s", err)
			}
			if strings.TrimSpace(key) == "" {
				return nil, errors.New("key cannot be empty")
			}

			data, err := sf.storage.GetData(bctx.Context, key)
			if err != nil {
				return nil, err
			}

			var val ast.Value
			val, err = ast.InterfaceToValue(data)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}

func (sf *StorageFuncs) SetData() (*rego.Function, rego.Builtin2) {
	return &rego.Function{
			Name:    "storage.set",
			Decl:    types.NewFunction(types.Args(types.S, types.A), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aKey, aData *ast.Term) (*ast.Term, error) {
			var key string
			var data map[string]interface{}

			if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key: %s", err)
			}
			if strings.TrimSpace(key) == "" {
				return nil, errors.New("key cannot be empty")
			}

			if err := ast.As(aData.Value, &data); err != nil {
				return nil, fmt.Errorf("invalid data: %s", err)
			}

			err := sf.storage.SetData(bctx.Context, key, data)
			if err != nil {
				return nil, err
			}

			var val ast.Value
			val, err = ast.InterfaceToValue(err)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}

func (sf *StorageFuncs) DeleteData() (*rego.Function, rego.Builtin1) {
	return &rego.Function{
			Name:    "storage.delete",
			Decl:    types.NewFunction(types.Args(types.S), types.A),
			Memoize: true,
		},
		func(bctx rego.BuiltinContext, aKey *ast.Term) (*ast.Term, error) {
			var key string

			if err := ast.As(aKey.Value, &key); err != nil {
				return nil, fmt.Errorf("invalid key: %s", err)
			}
			if strings.TrimSpace(key) == "" {
				return nil, errors.New("key cannot be empty")
			}

			err := sf.storage.DeleteData(bctx.Context, key)
			if err != nil {
				return nil, err
			}

			var val ast.Value
			val, err = ast.InterfaceToValue(err)
			if err != nil {
				return nil, err
			}

			return ast.NewTerm(val), nil
		}
}
