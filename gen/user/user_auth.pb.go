package user

import (
	proto "google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

type RegisterRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Email         string                 `protobuf:"bytes,2,opt,name=email,proto3" json:"email,omitempty"`
	Password      string                 `protobuf:"bytes,3,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RegisterRequest) Reset() {
	*x = RegisterRequest{}
	mi := &file_user_auth_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegisterRequest) String() string { return protoimpl.X.MessageStringOf(x) }
func (*RegisterRequest) ProtoMessage()    {}

func (x *RegisterRequest) ProtoReflect() protoreflect.Message {
	mi := &file_user_auth_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*RegisterRequest) Descriptor() ([]byte, []int) {
	return file_user_auth_proto_rawDescGZIP(), []int{0}
}

func (x *RegisterRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *RegisterRequest) GetEmail() string {
	if x != nil {
		return x.Email
	}
	return ""
}

func (x *RegisterRequest) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type RegisterResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Email         string                 `protobuf:"bytes,3,opt,name=email,proto3" json:"email,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RegisterResponse) Reset() {
	*x = RegisterResponse{}
	mi := &file_user_auth_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegisterResponse) String() string { return protoimpl.X.MessageStringOf(x) }
func (*RegisterResponse) ProtoMessage()    {}

func (x *RegisterResponse) ProtoReflect() protoreflect.Message {
	mi := &file_user_auth_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*RegisterResponse) Descriptor() ([]byte, []int) {
	return file_user_auth_proto_rawDescGZIP(), []int{1}
}

func (x *RegisterResponse) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

func (x *RegisterResponse) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *RegisterResponse) GetEmail() string {
	if x != nil {
		return x.Email
	}
	return ""
}

type LoginRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Email         string                 `protobuf:"bytes,1,opt,name=email,proto3" json:"email,omitempty"`
	Password      string                 `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LoginRequest) Reset() {
	*x = LoginRequest{}
	mi := &file_user_auth_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LoginRequest) String() string { return protoimpl.X.MessageStringOf(x) }
func (*LoginRequest) ProtoMessage()    {}

func (x *LoginRequest) ProtoReflect() protoreflect.Message {
	mi := &file_user_auth_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*LoginRequest) Descriptor() ([]byte, []int) {
	return file_user_auth_proto_rawDescGZIP(), []int{2}
}

func (x *LoginRequest) GetEmail() string {
	if x != nil {
		return x.Email
	}
	return ""
}

func (x *LoginRequest) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type LoginResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Email         string                 `protobuf:"bytes,3,opt,name=email,proto3" json:"email,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LoginResponse) Reset() {
	*x = LoginResponse{}
	mi := &file_user_auth_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LoginResponse) String() string { return protoimpl.X.MessageStringOf(x) }
func (*LoginResponse) ProtoMessage()    {}

func (x *LoginResponse) ProtoReflect() protoreflect.Message {
	mi := &file_user_auth_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*LoginResponse) Descriptor() ([]byte, []int) {
	return file_user_auth_proto_rawDescGZIP(), []int{3}
}

func (x *LoginResponse) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

func (x *LoginResponse) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *LoginResponse) GetEmail() string {
	if x != nil {
		return x.Email
	}
	return ""
}

var File_user_auth_proto protoreflect.FileDescriptor

var (
	file_user_auth_proto_rawDescOnce sync.Once
	file_user_auth_proto_rawDescData []byte
)

func file_user_auth_proto_rawDesc() []byte {
	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("user_auth.proto"),
		Package: proto.String("user"),
		Options: &descriptorpb.FileOptions{
			GoPackage: proto.String("github.com/peer-ledger/gen/user;user"),
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("RegisterRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					newStringField("name", 1),
					newStringField("email", 2),
					newStringField("password", 3),
				},
			},
			{
				Name: proto.String("RegisterResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					newStringField("user_id", 1),
					newStringField("name", 2),
					newStringField("email", 3),
				},
			},
			{
				Name: proto.String("LoginRequest"),
				Field: []*descriptorpb.FieldDescriptorProto{
					newStringField("email", 1),
					newStringField("password", 2),
				},
			},
			{
				Name: proto.String("LoginResponse"),
				Field: []*descriptorpb.FieldDescriptorProto{
					newStringField("user_id", 1),
					newStringField("name", 2),
					newStringField("email", 3),
				},
			},
		},
	}

	raw, err := proto.Marshal(fd)
	if err != nil {
		panic(err)
	}
	return raw
}

func file_user_auth_proto_rawDescGZIP() []byte {
	file_user_auth_proto_rawDescOnce.Do(func() {
		file_user_auth_proto_rawDescData = protoimpl.X.CompressGZIP(file_user_auth_proto_rawDesc())
	})
	return file_user_auth_proto_rawDescData
}

var file_user_auth_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_user_auth_proto_goTypes = []any{
	(*RegisterRequest)(nil),
	(*RegisterResponse)(nil),
	(*LoginRequest)(nil),
	(*LoginResponse)(nil),
}

func init() { file_user_auth_proto_init() }

func file_user_auth_proto_init() {
	if File_user_auth_proto != nil {
		return
	}

	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_user_auth_proto_rawDesc(),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_user_auth_proto_goTypes,
		DependencyIndexes: nil,
		MessageInfos:      file_user_auth_proto_msgTypes,
	}.Build()

	File_user_auth_proto = out.File
	file_user_auth_proto_goTypes = nil
}

func newStringField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}

var _ unsafe.Pointer
