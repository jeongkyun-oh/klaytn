// Code generated by protoc-gen-go. DO NOT EDIT.
// source: messages.proto

package protobuf

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Empty struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Empty) Reset()         { *m = Empty{} }
func (m *Empty) String() string { return proto.CompactTextString(m) }
func (*Empty) ProtoMessage()    {}
func (*Empty) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_f73a0d5eb029aff0, []int{0}
}
func (m *Empty) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Empty.Unmarshal(m, b)
}
func (m *Empty) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Empty.Marshal(b, m, deterministic)
}
func (dst *Empty) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Empty.Merge(dst, src)
}
func (m *Empty) XXX_Size() int {
	return xxx_messageInfo_Empty.Size(m)
}
func (m *Empty) XXX_DiscardUnknown() {
	xxx_messageInfo_Empty.DiscardUnknown(m)
}

var xxx_messageInfo_Empty proto.InternalMessageInfo

type TransactOpts struct {
	FromAddress          string   `protobuf:"bytes,1,opt,name=from_address,json=fromAddress,proto3" json:"from_address,omitempty"`
	PrivateKey           string   `protobuf:"bytes,2,opt,name=private_key,json=privateKey,proto3" json:"private_key,omitempty"`
	Nonce                int64    `protobuf:"varint,3,opt,name=nonce,proto3" json:"nonce,omitempty"`
	Value                int64    `protobuf:"varint,4,opt,name=value,proto3" json:"value,omitempty"`
	GasPrice             int64    `protobuf:"varint,5,opt,name=gas_price,json=gasPrice,proto3" json:"gas_price,omitempty"`
	GasLimit             int64    `protobuf:"varint,6,opt,name=gas_limit,json=gasLimit,proto3" json:"gas_limit,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *TransactOpts) Reset()         { *m = TransactOpts{} }
func (m *TransactOpts) String() string { return proto.CompactTextString(m) }
func (*TransactOpts) ProtoMessage()    {}
func (*TransactOpts) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_f73a0d5eb029aff0, []int{1}
}
func (m *TransactOpts) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TransactOpts.Unmarshal(m, b)
}
func (m *TransactOpts) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TransactOpts.Marshal(b, m, deterministic)
}
func (dst *TransactOpts) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TransactOpts.Merge(dst, src)
}
func (m *TransactOpts) XXX_Size() int {
	return xxx_messageInfo_TransactOpts.Size(m)
}
func (m *TransactOpts) XXX_DiscardUnknown() {
	xxx_messageInfo_TransactOpts.DiscardUnknown(m)
}

var xxx_messageInfo_TransactOpts proto.InternalMessageInfo

func (m *TransactOpts) GetFromAddress() string {
	if m != nil {
		return m.FromAddress
	}
	return ""
}

func (m *TransactOpts) GetPrivateKey() string {
	if m != nil {
		return m.PrivateKey
	}
	return ""
}

func (m *TransactOpts) GetNonce() int64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *TransactOpts) GetValue() int64 {
	if m != nil {
		return m.Value
	}
	return 0
}

func (m *TransactOpts) GetGasPrice() int64 {
	if m != nil {
		return m.GasPrice
	}
	return 0
}

func (m *TransactOpts) GetGasLimit() int64 {
	if m != nil {
		return m.GasLimit
	}
	return 0
}

type TransactionReq struct {
	Opts                 *TransactOpts `protobuf:"bytes,1,opt,name=opts,proto3" json:"opts,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *TransactionReq) Reset()         { *m = TransactionReq{} }
func (m *TransactionReq) String() string { return proto.CompactTextString(m) }
func (*TransactionReq) ProtoMessage()    {}
func (*TransactionReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_f73a0d5eb029aff0, []int{2}
}
func (m *TransactionReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TransactionReq.Unmarshal(m, b)
}
func (m *TransactionReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TransactionReq.Marshal(b, m, deterministic)
}
func (dst *TransactionReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TransactionReq.Merge(dst, src)
}
func (m *TransactionReq) XXX_Size() int {
	return xxx_messageInfo_TransactionReq.Size(m)
}
func (m *TransactionReq) XXX_DiscardUnknown() {
	xxx_messageInfo_TransactionReq.DiscardUnknown(m)
}

var xxx_messageInfo_TransactionReq proto.InternalMessageInfo

func (m *TransactionReq) GetOpts() *TransactOpts {
	if m != nil {
		return m.Opts
	}
	return nil
}

type TransactionResp struct {
	TxHash               string   `protobuf:"bytes,1,opt,name=tx_hash,json=txHash,proto3" json:"tx_hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *TransactionResp) Reset()         { *m = TransactionResp{} }
func (m *TransactionResp) String() string { return proto.CompactTextString(m) }
func (*TransactionResp) ProtoMessage()    {}
func (*TransactionResp) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_f73a0d5eb029aff0, []int{3}
}
func (m *TransactionResp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TransactionResp.Unmarshal(m, b)
}
func (m *TransactionResp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TransactionResp.Marshal(b, m, deterministic)
}
func (dst *TransactionResp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TransactionResp.Merge(dst, src)
}
func (m *TransactionResp) XXX_Size() int {
	return xxx_messageInfo_TransactionResp.Size(m)
}
func (m *TransactionResp) XXX_DiscardUnknown() {
	xxx_messageInfo_TransactionResp.DiscardUnknown(m)
}

var xxx_messageInfo_TransactionResp proto.InternalMessageInfo

func (m *TransactionResp) GetTxHash() string {
	if m != nil {
		return m.TxHash
	}
	return ""
}

func init() {
	proto.RegisterType((*Empty)(nil), "protobuf.Empty")
	proto.RegisterType((*TransactOpts)(nil), "protobuf.TransactOpts")
	proto.RegisterType((*TransactionReq)(nil), "protobuf.TransactionReq")
	proto.RegisterType((*TransactionResp)(nil), "protobuf.TransactionResp")
}

func init() { proto.RegisterFile("messages.proto", fileDescriptor_messages_f73a0d5eb029aff0) }

var fileDescriptor_messages_f73a0d5eb029aff0 = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x8f, 0xc1, 0x4a, 0xc3, 0x40,
	0x10, 0x86, 0x89, 0x6d, 0xd3, 0x76, 0x52, 0x2a, 0x2c, 0xa2, 0x0b, 0x1e, 0xac, 0x39, 0x95, 0x1e,
	0x72, 0xd0, 0xab, 0x17, 0x0f, 0x82, 0xa0, 0xa0, 0x04, 0xef, 0x61, 0x9a, 0x4e, 0x93, 0xc5, 0x26,
	0xbb, 0xee, 0x6c, 0x4b, 0xf3, 0x66, 0x3e, 0x9e, 0x64, 0xd3, 0x88, 0x9e, 0x96, 0xef, 0xfb, 0x67,
	0x87, 0xf9, 0x61, 0x5e, 0x11, 0x33, 0x16, 0xc4, 0x89, 0xb1, 0xda, 0x69, 0x31, 0xf1, 0xcf, 0x7a,
	0xbf, 0x8d, 0xc7, 0x30, 0x7a, 0xaa, 0x8c, 0x6b, 0xe2, 0xef, 0x00, 0x66, 0x1f, 0x16, 0x6b, 0xc6,
	0xdc, 0xbd, 0x19, 0xc7, 0xe2, 0x16, 0x66, 0x5b, 0xab, 0xab, 0x0c, 0x37, 0x1b, 0x4b, 0xcc, 0x32,
	0x58, 0x04, 0xcb, 0x69, 0x1a, 0xb5, 0xee, 0xb1, 0x53, 0xe2, 0x06, 0x22, 0x63, 0xd5, 0x01, 0x1d,
	0x65, 0x9f, 0xd4, 0xc8, 0x33, 0x3f, 0x01, 0x27, 0xf5, 0x42, 0x8d, 0xb8, 0x80, 0x51, 0xad, 0xeb,
	0x9c, 0xe4, 0x60, 0x11, 0x2c, 0x07, 0x69, 0x07, 0xad, 0x3d, 0xe0, 0x6e, 0x4f, 0x72, 0xd8, 0x59,
	0x0f, 0xe2, 0x1a, 0xa6, 0x05, 0x72, 0x66, 0xac, 0xca, 0x49, 0x8e, 0x7c, 0x32, 0x29, 0x90, 0xdf,
	0x5b, 0xee, 0xc3, 0x9d, 0xaa, 0x94, 0x93, 0xe1, 0x6f, 0xf8, 0xda, 0x72, 0xfc, 0x00, 0xf3, 0xfe,
	0x72, 0xa5, 0xeb, 0x94, 0xbe, 0xc4, 0x0a, 0x86, 0xda, 0xb8, 0xee, 0xe6, 0xe8, 0xee, 0x32, 0xe9,
	0xeb, 0x26, 0x7f, 0x1b, 0xa6, 0x7e, 0x26, 0x5e, 0xc1, 0xf9, 0xbf, 0xdf, 0x6c, 0xc4, 0x15, 0x8c,
	0xdd, 0x31, 0x2b, 0x91, 0xcb, 0x53, 0xeb, 0xd0, 0x1d, 0x9f, 0x91, 0xcb, 0x75, 0xe8, 0x17, 0xdd,
	0xff, 0x04, 0x00, 0x00, 0xff, 0xff, 0xf1, 0xa9, 0x57, 0x24, 0x50, 0x01, 0x00, 0x00,
}
