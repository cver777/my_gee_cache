package geecache

import (
	"fmt"
	"log"
	"sync"
	"geecache/singleflight"
	pb "geecache/geecachepb"
)

type Group struct { //负责与用户交互 一个组包含 组名，回调函数，缓存
	name      string		
	getter 	  Getter
	mainCache cache			
	peers	  PeerPicker
	loader 	  *singleflight.Group
}

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error) //回调函数

func (f GetterFunc) Get(key string) ([]byte, error) { //将普通get函数转换为getter接口实现，这样可以通过group调用get函数
	return f(key)
}

//全局变量
var (
	mu sync.RWMutex
	groups = make(map[string]*Group)//组名 组
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {//针对不同的信息创建不同的组，比如学生成绩，学生信息组，学生课程组
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name: 	   name,
		getter:    getter,//组对应的客户端
		mainCache: cache{cacheBytes: cacheBytes},
		loader:	   &singleflight.Group{},
	}
	groups[name] = g //添加映射
	log.Println("NewGroup created:", name)  // 加日志
	return g
}

func GetGroup(name string) *Group {//根据组名获取组，一个组包含cache，name和回调函数getter
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	log.Println("GetGroup lookup:", name, "found:", g != nil)  // 加日志
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { //本地cache找到对应的值，返回
		log.Println("[GeeCache] hit")
		return v, nil
	}

	return g.load(key) //本地没有找到
}

func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker call more than once")
	}
	g.peers = peers
}

// Get(key) -> mainCache.get(key) 失败 -> load(key)
// 	-> PickPeer(key) 选择远程节点
// 		-> 如果 key 在远程节点：getFromPeer(peer, key) 获取数据
// 		-> 如果 key 在自己节点：getLocally(key) 调用 getter 从数据库查询
// 			-> populateCache(key, value) 存入缓存

func (g *Group) load(key string) (value ByteView,err error) {//本地cache 不存在， 分布式场景 尝试从其他节点获取，目前调用用户回调函数获取源数据
	viewi, err := g.loader.Do(key, func() (interface{}, error){
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok{ //根据键值返回对应的客户端
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[geeCache] failed to get from peer", err)
			}
		}
		return g.getLocally(key)//缓存不命中
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
	
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)//调用回调函数获取，回调函数由用户定义，所以由用户决定从什么地方获取缓存值
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)} //byteview的构造函数 ，b是类成员
	g.populateCache(key, value) 
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {//加载完数据，将数据添加到缓存
	g.mainCache.add(key, value) //添加到maincache
}

func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}