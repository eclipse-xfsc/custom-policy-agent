package regofunc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc/regofuncfakes"
	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"
)

func TestStorageGetData(t *testing.T) {
	expected := `{"example":"data"}`

	storageFunc := regofunc.NewStorageFuncs(&regofuncfakes.FakeStorage{GetDataStub: func(ctx context.Context, s string) (any, error) {
		return map[string]interface{}{"example": "data"}, nil
	}})

	r := rego.New(
		rego.Query(`storage.get("exampleKey")`),
		rego.Function1(storageFunc.GetData()),
	)
	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Equal(t, expected, string(resultBytes))
}

func TestStorageSetData(t *testing.T) {
	storageFunc := regofunc.NewStorageFuncs(&regofuncfakes.FakeStorage{SetDataStub: func(ctx context.Context, s string, m map[string]interface{}) error {
		return nil
	}})

	r := rego.New(
		rego.Query(`storage.set("example", {"example":"data"})`),
		rego.Function2(storageFunc.SetData()),
	)
	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Contains(t, "null", string(resultBytes))
}

func TestStorageDeleteData(t *testing.T) {
	storageFunc := regofunc.NewStorageFuncs(&regofuncfakes.FakeStorage{DeleteDataStub: func(ctx context.Context, s string) error {
		return nil
	}})

	r := rego.New(
		rego.Query(`storage.delete("example")`),
		rego.Function1(storageFunc.DeleteData()),
	)
	resultSet, err := r.Eval(context.Background())
	assert.NoError(t, err)

	resultBytes, err := json.Marshal(resultSet[0].Expressions[0].Value)
	assert.NoError(t, err)
	assert.Contains(t, "null", string(resultBytes))
}
