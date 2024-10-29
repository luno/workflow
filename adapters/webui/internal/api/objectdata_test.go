package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luno/workflow"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/webui/internal/api"
)

type testObjectData struct {
	Name  string
	Email string
}

func TestObjectDataHandler(t *testing.T) {
	expectedResponseData := testObjectData{
		Name:  "Andrew Wormald",
		Email: "andrew@workflow.com",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lookupFn := func(ctx context.Context, id int64) (*workflow.Record, error) {
			b, err := workflow.Marshal(&expectedResponseData)
			require.NoError(t, err)

			return &workflow.Record{
				Object: b,
			}, nil
		}
		api.ObjectData(lookupFn)(w, r)
	}))
	t.Cleanup(srv.Close)

	body, err := json.Marshal(api.ObjectDataRequest{RecordID: 1})
	require.NoError(t, err)

	resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
	require.NoError(t, err)

	require.Equal(t, 200, resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var actualResp testObjectData
	err = json.Unmarshal(respBody, &actualResp)
	require.NoError(t, err)

	require.Equal(t, expectedResponseData, actualResp)
}
