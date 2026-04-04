package wallet

import (
	proto "google.golang.org/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
)

type CreateWalletRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateWalletRequest) Reset() {
	*x = CreateWalletRequest{}
	mi := &file_wallet_extra_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}
func (x *CreateWalletRequest) String() string { return protoimpl.X.MessageStringOf(x) }
func (*CreateWalletRequest) ProtoMessage()    {}
func (x *CreateWalletRequest) ProtoReflect() protoreflect.Message {
	mi := &file_wallet_extra_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}
func (*CreateWalletRequest) Descriptor() ([]byte, []int) { return file_wallet_extra_proto_rawDescGZIP(), []int{0} }
func (x *CreateWalletRequest) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}

type CreateWalletResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Balance       float64                `protobuf:"fixed64,2,opt,name=balance,proto3" json:"balance,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateWalletResponse) Reset() {
	*x = CreateWalletResponse{}
	mi := &file_wallet_extra_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}
func (x *CreateWalletResponse) String() string { return protoimpl.X.MessageStringOf(x) }
func (*CreateWalletResponse) ProtoMessage()    {}
func (x *CreateWalletResponse) ProtoReflect() protoreflect.Message {
	mi := &file_wallet_extra_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}
func (*CreateWalletResponse) Descriptor() ([]byte, []int) { return file_wallet_extra_proto_rawDescGZIP(), []int{1} }
func (x *CreateWalletResponse) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}
func (x *CreateWalletResponse) GetBalance() float64 {
	if x != nil {
		return x.Balance
	}
	return 0
}

type TopUpRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Amount        float64                `protobuf:"fixed64,2,opt,name=amount,proto3" json:"amount,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TopUpRequest) Reset() {
	*x = TopUpRequest{}
	mi := &file_wallet_extra_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}
func (x *TopUpRequest) String() string { return protoimpl.X.MessageStringOf(x) }
func (*TopUpRequest) ProtoMessage()    {}
func (x *TopUpRequest) ProtoReflect() protoreflect.Message {
	mi := &file_wallet_extra_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}
func (*TopUpRequest) Descriptor() ([]byte, []int) { return file_wallet_extra_proto_rawDescGZIP(), []int{2} }
func (x *TopUpRequest) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}
func (x *TopUpRequest) GetAmount() float64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

type TopUpResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	UserId        string                 `protobuf:"bytes,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Balance       float64                `protobuf:"fixed64,2,opt,name=balance,proto3" json:"balance,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TopUpResponse) Reset() {
	*x = TopUpResponse{}
	mi := &file_wallet_extra_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}
func (x *TopUpResponse) String() string { return protoimpl.X.MessageStringOf(x) }
func (*TopUpResponse) ProtoMessage()    {}
func (x *TopUpResponse) ProtoReflect() protoreflect.Message {
	mi := &file_wallet_extra_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}
func (*TopUpResponse) Descriptor() ([]byte, []int) { return file_wallet_extra_proto_rawDescGZIP(), []int{3} }
func (x *TopUpResponse) GetUserId() string {
	if x != nil {
		return x.UserId
	}
	return ""
}
func (x *TopUpResponse) GetBalance() float64 {
	if x != nil {
		return x.Balance
	}
	return 0
}

var File_wallet_extra_proto protoreflect.FileDescriptor

var (
	file_wallet_extra_proto_rawDescOnce sync.Once
	file_wallet_extra_proto_rawDescData []byte
)

func file_wallet_extra_proto_rawDesc() []byte {
	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("wallet_extra.proto"),
		Package: proto.String("wallet"),
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("github.com/peer-ledger/gen/wallet;wallet")},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("CreateWalletRequest"), Field: []*descriptorpb.FieldDescriptorProto{newWalletStringField("user_id", 1)}},
			{Name: proto.String("CreateWalletResponse"), Field: []*descriptorpb.FieldDescriptorProto{newWalletStringField("user_id", 1), newWalletDoubleField("balance", 2)}},
			{Name: proto.String("TopUpRequest"), Field: []*descriptorpb.FieldDescriptorProto{newWalletStringField("user_id", 1), newWalletDoubleField("amount", 2)}},
			{Name: proto.String("TopUpResponse"), Field: []*descriptorpb.FieldDescriptorProto{newWalletStringField("user_id", 1), newWalletDoubleField("balance", 2)}},
		},
	}
	raw, err := proto.Marshal(fd)
	if err != nil {
		panic(err)
	}
	return raw
}

func file_wallet_extra_proto_rawDescGZIP() []byte {
	file_wallet_extra_proto_rawDescOnce.Do(func() {
		file_wallet_extra_proto_rawDescData = protoimpl.X.CompressGZIP(file_wallet_extra_proto_rawDesc())
	})
	return file_wallet_extra_proto_rawDescData
}

var file_wallet_extra_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_wallet_extra_proto_goTypes = []any{
	(*CreateWalletRequest)(nil),
	(*CreateWalletResponse)(nil),
	(*TopUpRequest)(nil),
	(*TopUpResponse)(nil),
}

func init() { file_wallet_extra_proto_init() }
func file_wallet_extra_proto_init() {
	if File_wallet_extra_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_wallet_extra_proto_rawDesc(),
			NumMessages:   4,
		},
		GoTypes:      file_wallet_extra_proto_goTypes,
		MessageInfos: file_wallet_extra_proto_msgTypes,
	}.Build()
	File_wallet_extra_proto = out.File
	file_wallet_extra_proto_goTypes = nil
}

func newWalletStringField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
}

func newWalletDoubleField(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum(),
	}
}
