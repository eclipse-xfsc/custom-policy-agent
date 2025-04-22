package regofunc_test

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-xfsc/custom-policy-agent/internal/regofunc"
)

func TestGetHeaderFunc(t *testing.T) {
	headers := map[string]string{
		"Authorization": "Bearer XXX",
		"X-Location":    "https://example.com",
	}

	t.Run("get Authorization header", func(t *testing.T) {
		r := rego.New(
			rego.Query(`external.http.header("Authorization")`),
			rego.Function1(regofunc.GetHeaderFunc(headers)),
		)
		resultSet, err := r.Eval(context.Background())
		assert.NoError(t, err)

		result := resultSet[0].Expressions[0].Value

		assert.NoError(t, err)
		assert.Equal(t, "Bearer XXX", result)
	})

	t.Run("get X-Location header", func(t *testing.T) {
		r := rego.New(
			rego.Query(`external.http.header("X-Location")`),
			rego.Function1(regofunc.GetHeaderFunc(headers)),
		)
		resultSet, err := r.Eval(context.Background())
		assert.NoError(t, err)

		result := resultSet[0].Expressions[0].Value

		assert.NoError(t, err)
		assert.Equal(t, "https://example.com", result)
	})
}
