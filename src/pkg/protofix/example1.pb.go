package protofix

type Dummy struct {
	NotSomethingICareAbout   string `protobuf:"bytes,2,opt,name=id" json:"id,omitempty"`
}

type Commit struct {
	Id   string `protobuf:"bytes,2,opt,name=id" json:"id,omitempty"`
}
