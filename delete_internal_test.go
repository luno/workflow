package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunDelete(t *testing.T) {
	type object struct {
		pii    string
		notPII string
	}

	testErr := errors.New("test error")

	testCases := []struct {
		Name        string
		storeFn     func(ctx context.Context, record *Record) error
		lookupFn    func(ctx context.Context, runID string) (*Record, error)
		deleteFn    func(wr *Record) ([]byte, error)
		expectedErr error
	}{
		{
			Name: "Golden path - custom delete",
			storeFn: func(ctx context.Context, record *Record) error {
				require.Equal(t, RunStateDataDeleted, record.RunState)
				return nil
			},
			lookupFn: func(ctx context.Context, runID string) (*Record, error) {
				o := object{
					pii:    "my name",
					notPII: "name of the month",
				}

				b, err := Marshal(&o)
				require.NoError(t, err)

				return &Record{
					Object:   b,
					RunState: RunStateRequestedDataDeleted,
				}, nil
			},
			deleteFn: func(wr *Record) ([]byte, error) {
				var o object

				err := Unmarshal(wr.Object, &o)
				require.NoError(t, err)

				o.pii = ""

				return Marshal(&o)
			},
			expectedErr: nil,
		},
		{
			Name: "Golden path - default delete",
			storeFn: func(ctx context.Context, record *Record) error {
				require.Equal(t, RunStateDataDeleted, record.RunState)
				return nil
			},
			lookupFn: func(ctx context.Context, runID string) (*Record, error) {
				o := object{
					pii:    "my name",
					notPII: "name of the month",
				}

				b, err := Marshal(&o)
				require.NoError(t, err)

				return &Record{
					Object:   b,
					RunState: RunStateRequestedDataDeleted,
				}, nil
			},
			expectedErr: nil,
		},
		{
			Name: "Return err on lookup error",
			lookupFn: func(ctx context.Context, runID string) (*Record, error) {
				return nil, testErr
			},
			expectedErr: testErr,
		},
		{
			Name: "Return err on store error",
			lookupFn: func(ctx context.Context, runID string) (*Record, error) {
				return &Record{
					RunState: RunStateRequestedDataDeleted,
				}, nil
			},
			storeFn: func(ctx context.Context, record *Record) error {
				return testErr
			},
			expectedErr: testErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := t.Context()
			err := runDelete(
				tc.storeFn,
				tc.lookupFn,
				tc.deleteFn,
			)(ctx, &Event{})
			require.ErrorIs(t, err, tc.expectedErr)
		})
	}
}
