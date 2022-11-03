package document

import (
	"context"

	"github.com/ernestrc/blue/retry"
)

// ConsistentUpdate calls Update on the given service and tries to update
// the underlying document with the provided retry strategy and the
// callback function which gets called before attempting to update
// the document in storage. This can be used to scan the updated
// doc for any data that has been updated in the process of performing
// this operation and to update preconditions with the latest values.
//
// The given doc value will be updated so it should be a valid value as
// defined by Service.Get.
func ConsistentUpdate(
	ctx context.Context, svc Service, ID string, doc interface{},
	retryStrategy retry.Strategy, callback func() ([]Update, []Precondition),
) error {
	return retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
		updates, preconditions := callback()
		err := svc.Update(ctx, ID, updates, preconditions...)
		if err == nil {
			// update to the latest version
			return false, svc.Get(ctx, ID, doc)
		}
		if err != ErrPreconditionFailed {
			return false, err
		}
		err = svc.Get(ctx, ID, doc)
		return err == nil, ErrPreconditionFailed
	})
}
