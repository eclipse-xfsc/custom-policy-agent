package policydata

import (
	"encoding/json"
	"time"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

type DataConfig struct {
	URL    string
	Method string
	Period Duration
	Body   interface{}
}

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}
