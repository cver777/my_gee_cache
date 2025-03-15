package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

type Map struct {
	hash 		Hash			//哈希函数 默认使用crc32.ChecksumIEEE
	replicas 	int				//虚拟节点的个数 ，提高负载均衡效果
	keys		[]int 			//sorted	  以便二分查找 存储所有虚拟节点的哈希值
	hashMap 	map[int]string	//映射哈希值到真实节点
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:	  fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE    
	}
	return m
}

func (m *Map) Add(keys ...string) {
	for _, key := range keys {//对每个节点 分配replicas个虚拟节点 
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))// key + i 再计算哈希值
			m.keys = append(m.keys, hash) //动态数组末尾追加
			m.hashMap[hash] = key         //添加虚拟节点到真实节点的映射
		}
	}
	sort.Ints(m.keys)//对 添加了节点的哈希值进行排序 方便二分查找
}

func (m *Map) Get(key string) string { //给定key，数据不在当前节点，调用这个函数找key对应的节点
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key))) //计算当前节点的哈希值 

	idx := sort.Search(len(m.keys), func(i int) bool { //二分查找  
		return m.keys[i] >= hash                      
	})

	return m.hashMap[m.keys[idx%len(m.keys)]]//如果不取模，会返回len(m.keys)  找这个节点获取数据
}