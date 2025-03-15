package geecache

import(
	"fmt"
	"log"
	"net/http"
	"strings"
	"net/url"
	"io/ioutil"
	"sync"
	"geecache/consistenthash"
	pb "geecache/geecachepb"
	"github.com/golang/protobuf/proto"
)

const  (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50 //一个节点拥有的虚拟节点
)
type HTTPPool struct {  	//HTTP的核心数据结构包括服务端和客户端
	self 		string						//自己的地址 本机的名/IP 端口
	basePath 	string						//节点间通信地址的前缀
	mu			sync.Mutex
	peers		*consistenthash.Map   //将节点的IP//PORT的等信息映射为哈希值，以及哈希值对应的真实节点的映射 
									  //new add(peer) get(key)这样就可以通过key获取对应的IP//端口
	httpGetters map[string]*httpGetter	//一个httpGetter对应一个baseURL   key是IP:PORT
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self: 	  self,			  //当前节点的地址 “http://localhost:8001"
		basePath: defaultBasePath,//路径前缀
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

//从请求报文 请求获取路径
//解析请求路径 ，检查路径是否合法
//从路径获取 组名和键
//获取组名
//获取键
//
// package http

// type Handler interface {
//     ServeHTTP(w ResponseWriter, r *Request)
// }

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {  //服务端核心函数，任何声明了ServeHTTP的类都可以注册http.Handler
	if !strings.HasPrefix(r.URL.Path, p.basePath) { //路径是否以basePath开头
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// <basepath>/<groupname>/<key> required
	//path := strings.TrimPrefix(r.URL.Path, p.basePath+"/") 
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2) //请求解析路径，从basepath后开始 解析第一个"/"分割为两段
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)//哈希表查找

	if group == nil {
		http.Error(w, "no such group: " + groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key) //找到对应的组，根据组查询key ，返回二进制数据
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})//序列化二进制信息
	if err != nil{
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")//响应内容是二进制数据
	w.Write(body)

}

var _ PeerPicker = (*HTTPPool)(nil)

type httpGetter struct {		//http客户端
	baseURL string				
}

func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil) //给虚拟节点的哈希值分配空间，初始化哈希表
	p.peers.Add(peers...) //将节点映射到哈希空间
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}// //http：114.514.0/8080+basePath?
	}
}

func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)//根据key获取对应节点
		return p.httpGetters[peer], true//返回节点对应的httpgetter 采用interface peergetter指向
	}
	return nil, false
}

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error { //客户端请求函数
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),//进行URL编码，可以放在URL中 最终生成 //http://example.com/_geecache/group/key
		url.QueryEscape(in.GetKey()),
	)
	res, err := http.Get(u) //在哪个URL找，哪个组，/键值 //http://example.com/_geecache/
	if err != nil {
		return err
	}
	defer res.Body.Close()  //确保响应体在函数返回前关闭

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil { //反序列化bytes，结果放在out
		return fmt.Errorf("decoding response body: %v", err)
	}

	return nil 
}

var _ PeerGetter = (*httpGetter)(nil) //确保httpGetter实现了PeerGetter接口的所有方法
//如果将来 PeerGetter 接口发生变化（例如新增方法），编译器会提示 httpGetter 需要更新。