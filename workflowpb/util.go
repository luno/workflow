package workflowpb

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/luno/workflow"
	werrors "github.com/luno/workflow/internal/errors"
)

func ProtoMarshal(r *workflow.Record) ([]byte, error) {
	pb, err := proto.Marshal(ToProto(r))
	if err != nil {
		return nil, werrors.Wrap(err, "failed to proto marshal record")
	}

	return pb, nil
}

func ToProto(r *workflow.Record) *Record {
	return &Record{
		Id:           r.ID,
		WorkflowName: r.WorkflowName,
		ForeignId:    r.ForeignID,
		RunId:        r.RunID,
		Status:       int32(r.Status),
		RunState:     int32(r.RunState),
		Object:       r.Object,
		CreatedAt:    timestamppb.New(r.CreatedAt),
		UpdatedAt:    timestamppb.New(r.UpdatedAt),
	}
}

func UnmarshalRecord(b []byte) (*workflow.Record, error) {
	var wpb Record
	err := proto.Unmarshal(b, &wpb)
	if err != nil {
		return nil, werrors.Wrap(err, "failed to proto marshal record")
	}

	return &workflow.Record{
		ID:           wpb.Id,
		WorkflowName: wpb.WorkflowName,
		ForeignID:    wpb.ForeignId,
		RunID:        wpb.RunId,
		Status:       int(wpb.Status),
		RunState:     workflow.RunState(wpb.RunState),
		Object:       wpb.Object,
		CreatedAt:    wpb.CreatedAt.AsTime(),
		UpdatedAt:    wpb.UpdatedAt.AsTime(),
	}, nil
}
