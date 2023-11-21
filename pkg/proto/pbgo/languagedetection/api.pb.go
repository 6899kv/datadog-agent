// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.23.4
// source: datadog/languagedetection/api.proto

package languagedetection

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Process struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Pid     int32    `protobuf:"varint,1,opt,name=pid,proto3" json:"pid,omitempty"`
	Command string   `protobuf:"bytes,2,opt,name=command,proto3" json:"command,omitempty"`
	Cmdline []string `protobuf:"bytes,3,rep,name=cmdline,proto3" json:"cmdline,omitempty"`
}

func (x *Process) Reset() {
	*x = Process{}
	if protoimpl.UnsafeEnabled {
		mi := &file_datadog_languagedetection_api_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Process) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Process) ProtoMessage() {}

func (x *Process) ProtoReflect() protoreflect.Message {
	mi := &file_datadog_languagedetection_api_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Process.ProtoReflect.Descriptor instead.
func (*Process) Descriptor() ([]byte, []int) {
	return file_datadog_languagedetection_api_proto_rawDescGZIP(), []int{0}
}

func (x *Process) GetPid() int32 {
	if x != nil {
		return x.Pid
	}
	return 0
}

func (x *Process) GetCommand() string {
	if x != nil {
		return x.Command
	}
	return ""
}

func (x *Process) GetCmdline() []string {
	if x != nil {
		return x.Cmdline
	}
	return nil
}

// Should closely match `languagemodels.Language`
type Language struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Version string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
}

func (x *Language) Reset() {
	*x = Language{}
	if protoimpl.UnsafeEnabled {
		mi := &file_datadog_languagedetection_api_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Language) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Language) ProtoMessage() {}

func (x *Language) ProtoReflect() protoreflect.Message {
	mi := &file_datadog_languagedetection_api_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Language.ProtoReflect.Descriptor instead.
func (*Language) Descriptor() ([]byte, []int) {
	return file_datadog_languagedetection_api_proto_rawDescGZIP(), []int{1}
}

func (x *Language) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Language) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

type DetectLanguageRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Processes []*Process `protobuf:"bytes,1,rep,name=processes,proto3" json:"processes,omitempty"`
}

func (x *DetectLanguageRequest) Reset() {
	*x = DetectLanguageRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_datadog_languagedetection_api_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DetectLanguageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DetectLanguageRequest) ProtoMessage() {}

func (x *DetectLanguageRequest) ProtoReflect() protoreflect.Message {
	mi := &file_datadog_languagedetection_api_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DetectLanguageRequest.ProtoReflect.Descriptor instead.
func (*DetectLanguageRequest) Descriptor() ([]byte, []int) {
	return file_datadog_languagedetection_api_proto_rawDescGZIP(), []int{2}
}

func (x *DetectLanguageRequest) GetProcesses() []*Process {
	if x != nil {
		return x.Processes
	}
	return nil
}

type DetectLanguageResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Languages []*Language `protobuf:"bytes,1,rep,name=languages,proto3" json:"languages,omitempty"`
}

func (x *DetectLanguageResponse) Reset() {
	*x = DetectLanguageResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_datadog_languagedetection_api_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DetectLanguageResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DetectLanguageResponse) ProtoMessage() {}

func (x *DetectLanguageResponse) ProtoReflect() protoreflect.Message {
	mi := &file_datadog_languagedetection_api_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DetectLanguageResponse.ProtoReflect.Descriptor instead.
func (*DetectLanguageResponse) Descriptor() ([]byte, []int) {
	return file_datadog_languagedetection_api_proto_rawDescGZIP(), []int{3}
}

func (x *DetectLanguageResponse) GetLanguages() []*Language {
	if x != nil {
		return x.Languages
	}
	return nil
}

var File_datadog_languagedetection_api_proto protoreflect.FileDescriptor

var file_datadog_languagedetection_api_proto_rawDesc = []byte{
	0x0a, 0x23, 0x64, 0x61, 0x74, 0x61, 0x64, 0x6f, 0x67, 0x2f, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61,
	0x67, 0x65, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x61, 0x70, 0x69, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x19, 0x64, 0x61, 0x74, 0x61, 0x64, 0x6f, 0x67, 0x2e, 0x6c,
	0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x22, 0x4f, 0x0a, 0x07, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x12, 0x10, 0x0a, 0x03, 0x70,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x03, 0x70, 0x69, 0x64, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6d, 0x64, 0x6c, 0x69,
	0x6e, 0x65, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6d, 0x64, 0x6c, 0x69, 0x6e,
	0x65, 0x22, 0x38, 0x0a, 0x08, 0x4c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x59, 0x0a, 0x15, 0x44,
	0x65, 0x74, 0x65, 0x63, 0x74, 0x4c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x40, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x65,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x22, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x64, 0x6f,
	0x67, 0x2e, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x52, 0x09, 0x70, 0x72, 0x6f,
	0x63, 0x65, 0x73, 0x73, 0x65, 0x73, 0x22, 0x5b, 0x0a, 0x16, 0x44, 0x65, 0x74, 0x65, 0x63, 0x74,
	0x4c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x41, 0x0a, 0x09, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x64, 0x6f, 0x67, 0x2e, 0x6c, 0x61,
	0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x4c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x52, 0x09, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61,
	0x67, 0x65, 0x73, 0x42, 0x22, 0x5a, 0x20, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x70, 0x62, 0x67, 0x6f, 0x2f, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x64, 0x65,
	0x74, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_datadog_languagedetection_api_proto_rawDescOnce sync.Once
	file_datadog_languagedetection_api_proto_rawDescData = file_datadog_languagedetection_api_proto_rawDesc
)

func file_datadog_languagedetection_api_proto_rawDescGZIP() []byte {
	file_datadog_languagedetection_api_proto_rawDescOnce.Do(func() {
		file_datadog_languagedetection_api_proto_rawDescData = protoimpl.X.CompressGZIP(file_datadog_languagedetection_api_proto_rawDescData)
	})
	return file_datadog_languagedetection_api_proto_rawDescData
}

var file_datadog_languagedetection_api_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_datadog_languagedetection_api_proto_goTypes = []interface{}{
	(*Process)(nil),                // 0: datadog.languagedetection.Process
	(*Language)(nil),               // 1: datadog.languagedetection.Language
	(*DetectLanguageRequest)(nil),  // 2: datadog.languagedetection.DetectLanguageRequest
	(*DetectLanguageResponse)(nil), // 3: datadog.languagedetection.DetectLanguageResponse
}
var file_datadog_languagedetection_api_proto_depIdxs = []int32{
	0, // 0: datadog.languagedetection.DetectLanguageRequest.processes:type_name -> datadog.languagedetection.Process
	1, // 1: datadog.languagedetection.DetectLanguageResponse.languages:type_name -> datadog.languagedetection.Language
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_datadog_languagedetection_api_proto_init() }
func file_datadog_languagedetection_api_proto_init() {
	if File_datadog_languagedetection_api_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_datadog_languagedetection_api_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Process); i {
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
		file_datadog_languagedetection_api_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Language); i {
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
		file_datadog_languagedetection_api_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DetectLanguageRequest); i {
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
		file_datadog_languagedetection_api_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DetectLanguageResponse); i {
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
			RawDescriptor: file_datadog_languagedetection_api_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_datadog_languagedetection_api_proto_goTypes,
		DependencyIndexes: file_datadog_languagedetection_api_proto_depIdxs,
		MessageInfos:      file_datadog_languagedetection_api_proto_msgTypes,
	}.Build()
	File_datadog_languagedetection_api_proto = out.File
	file_datadog_languagedetection_api_proto_rawDesc = nil
	file_datadog_languagedetection_api_proto_goTypes = nil
	file_datadog_languagedetection_api_proto_depIdxs = nil
}
