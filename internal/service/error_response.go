package service

import (
	"context"

	goahttp "goa.design/goa/v3/http"
	goa "goa.design/goa/v3/pkg"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

func NewErrorResponse(ctx context.Context, err error) goahttp.Statuser {
	if err == nil {
		return nil
	}

	var newerr *errors.Error
	switch e := err.(type) {
	case *errors.Error:
		newerr = e
	case *goa.ServiceError:
		// Use goahttp.ErrorResponse to determine error kind
		goaerr := goahttp.NewErrorResponse(ctx, e)
		kind := errors.GetKind(goaerr.StatusCode())
		newerr = &errors.Error{
			ID:      e.ID,
			Kind:    kind,
			Message: e.Message,
			Err:     e,
		}
	default:
		newerr = &errors.Error{
			ID:      errors.NewID(),
			Kind:    errors.Internal,
			Message: e.Error(),
			Err:     e,
		}
	}

	return newerr
}
