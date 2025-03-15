package geecache

type ByteView struct {
	b []byte  //选择byte 能够支持任意数据类型的存储，比如字符串，图片
}
// b小写 表示只读，外部程序不可修改，只能通过byteslice返回一份缓存拷贝

func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte { //克隆一份缓存切片
	return cloneBytes(v.b)
}

func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}