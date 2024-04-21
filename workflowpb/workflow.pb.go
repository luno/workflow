// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.32.0
// 	protoc        v4.24.4
// source: workflow.proto

package workflowpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Record struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id           int64  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	RunId        string `protobuf:"bytes,2,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	WorkflowName string `protobuf:"bytes,3,opt,name=workflow_name,json=workflowName,proto3" json:"workflow_name,omitempty"`
	ForeignId    string `protobuf:"bytes,4,opt,name=foreign_id,json=foreignId,proto3" json:"foreign_id,omitempty"`
	// Deprecated: Marked as deprecated in workflow.proto.
	IsStart bool `protobuf:"varint,5,opt,name=is_start,json=isStart,proto3" json:"is_start,omitempty"` // Use RunState
	// Deprecated: Marked as deprecated in workflow.proto.
	IsEnd     bool                   `protobuf:"varint,6,opt,name=is_end,json=isEnd,proto3" json:"is_end,omitempty"` // Use RunState
	Object    []byte                 `protobuf:"bytes,7,opt,name=object,proto3" json:"object,omitempty"`
	CreatedAt *timestamppb.Timestamp `protobuf:"bytes,8,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	Status    int32                  `protobuf:"varint,9,opt,name=status,proto3" json:"status,omitempty"`
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,10,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	RunState  int32                  `protobuf:"varint,11,opt,name=run_state,json=runState,proto3" json:"run_state,omitempty"`
}

func (x *Record) Reset() {
	*x = Record{}
	if protoimpl.UnsafeEnabled {
		mi := &file_workflow_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Record) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Record) ProtoMessage() {}

func (x *Record) ProtoReflect() protoreflect.Message {
	mi := &file_workflow_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Record.ProtoReflect.Descriptor instead.
func (*Record) Descriptor() ([]byte, []int) {
	return file_workflow_proto_rawDescGZIP(), []int{0}
}

func (x *Record) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Record) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *Record) GetWorkflowName() string {
	if x != nil {
		return x.WorkflowName
	}
	return ""
}

func (x *Record) GetForeignId() string {
	if x != nil {
		return x.ForeignId
	}
	return ""
}

// Deprecated: Marked as deprecated in workflow.proto.
func (x *Record) GetIsStart() bool {
	if x != nil {
		return x.IsStart
	}
	return false
}

// Deprecated: Marked as deprecated in workflow.proto.
func (x *Record) GetIsEnd() bool {
	if x != nil {
		return x.IsEnd
	}
	return false
}

func (x *Record) GetObject() []byte {
	if x != nil {
		return x.Object
	}
	return nil
}

func (x *Record) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

func (x *Record) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Record) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}

func (x *Record) GetRunState() int32 {
	if x != nil {
		return x.RunState
	}
	return 0
}

var File_workflow_proto protoreflect.FileDescriptor

var file_workflow_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0a, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x70, 0x62, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xf0, 0x02,
	0x0a, 0x06, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x15, 0x0a, 0x06, 0x72, 0x75, 0x6e, 0x5f,
	0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x75, 0x6e, 0x49, 0x64, 0x12,
	0x23, 0x0a, 0x0d, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x5f, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77,
	0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x66, 0x6f, 0x72, 0x65, 0x69, 0x67, 0x6e, 0x5f,
	0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x66, 0x6f, 0x72, 0x65, 0x69, 0x67,
	0x6e, 0x49, 0x64, 0x12, 0x1d, 0x0a, 0x08, 0x69, 0x73, 0x5f, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x08, 0x42, 0x02, 0x18, 0x01, 0x52, 0x07, 0x69, 0x73, 0x53, 0x74, 0x61,
	0x72, 0x74, 0x12, 0x19, 0x0a, 0x06, 0x69, 0x73, 0x5f, 0x65, 0x6e, 0x64, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x08, 0x42, 0x02, 0x18, 0x01, 0x52, 0x05, 0x69, 0x73, 0x45, 0x6e, 0x64, 0x12, 0x16, 0x0a,
	0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x6f,
	0x62, 0x6a, 0x65, 0x63, 0x74, 0x12, 0x39, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64,
	0x5f, 0x61, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74,
	0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x39, 0x0a, 0x0a, 0x75, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x64, 0x41, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x72, 0x75, 0x6e, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x18, 0x0b, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x72, 0x75, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65,
	0x42, 0x0f, 0x5a, 0x0d, 0x2e, 0x2e, 0x2f, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x70,
	0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_workflow_proto_rawDescOnce sync.Once
	file_workflow_proto_rawDescData = file_workflow_proto_rawDesc
)

func file_workflow_proto_rawDescGZIP() []byte {
	file_workflow_proto_rawDescOnce.Do(func() {
		file_workflow_proto_rawDescData = protoimpl.X.CompressGZIP(file_workflow_proto_rawDescData)
	})
	return file_workflow_proto_rawDescData
}

var file_workflow_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_workflow_proto_goTypes = []interface{}{
	(*Record)(nil),                // 0: workflowpb.Record
	(*timestamppb.Timestamp)(nil), // 1: google.protobuf.Timestamp
}
var file_workflow_proto_depIdxs = []int32{
	1, // 0: workflowpb.Record.created_at:type_name -> google.protobuf.Timestamp
	1, // 1: workflowpb.Record.updated_at:type_name -> google.protobuf.Timestamp
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_workflow_proto_init() }
func file_workflow_proto_init() {
	if File_workflow_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_workflow_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Record); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_workflow_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_workflow_proto_goTypes,
		DependencyIndexes: file_workflow_proto_depIdxs,
		MessageInfos:      file_workflow_proto_msgTypes,
	}.Build()
	File_workflow_proto = out.File
	file_workflow_proto_rawDesc = nil
	file_workflow_proto_goTypes = nil
	file_workflow_proto_depIdxs = nil
}
