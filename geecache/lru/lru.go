package lru

import (
	"container/list" //数据结构的包
	"math/rand"
	"time"
	"fmt"
)


//lru算法实现 双向链表+哈希表
//实现增删改查的复杂度是O(1) 查询用哈希表来查询， 键是key，value是链表的结点指针
//最近访问过的数据移动到队头，新增的元素移到队头，满了队尾元素移除


var DefaultExpireRandom time.Duration = 3 * time.Minute //默认过期时间的范围为0-3分钟 ，（避免缓存雪崩）

type NowFunc func() time.Time

var nowFunc NowFunc = time.Now

type Cache struct{
	maxBytes int64	//当前缓存能够使用的最大byte
	nbytes   int64 //当前使用的字节数
	ll       *list.List //双向链表， 包含哨兵节点Element 和 长度
	cache	 map[string]*list.Element // 链表节点结构体 包含前后元素 所属链表list 存储的值(Value 一个interface接口)

	OnEvicted func(key string, value Value)

	Now      NowFunc

	ExpireRandom time.Duration
}

type entry struct { //
	key string
	value Value
	expire time.Time  //过期时间
	addTime time.Time//缓存项的存活时间
}

type Value interface{ //父类指针
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes: maxBytes,
		ll		: list.New(),
		cache 	: make(map[string]*list.Element),//动态分配
		OnEvicted: onEvicted,//某个缓存淘汰是对键值进行额外的操作， 比如日志记录或者通知其他组件

		Now     : nowFunc,
		ExpireRandom: DefaultExpireRandom,
	}
} 

func (c *Cache) Get(key string) (value Value, ok bool) {
	ele, ok := c.cache[key] //获取ele的指针
	if ok {
		kv := ele.Value.(*entry)// Value interface{} 类型断言 
		
		//如果kv过期了，移除缓存
		if kv.expire.Before(time.Now()) {
			fmt.Println("Key expired:", kv.key) // 调试信息
			c.ll.Remove(ele)
			c.removeElement(ele)
			return nil, false
		}
		//没有过期，更新键值对的过期时间
		expireTime := kv.expire.Sub(kv.addTime)
		kv.expire  = time.Now().Add(expireTime)
		kv.addTime = time.Now()
		c.ll.MoveToFront(ele)//移到队头
		
		return kv.value, true
	}
	return nil, false
}

func (c *Cache) Add(key string, value Value, expire time.Time) {
	// randDuration 是用户添加的过期时间进行一定范围的随机，用于防止大量缓存同一时间过期而发生缓存雪崩
	randDuration := time.Duration(rand.Int63n(int64(c.ExpireRandom)))

	if ele, ok := c.cache[key]; ok {//存在将其替换
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)//指针传递以便修改
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value;
		kv.expire = expire.Add(randDuration)
	} else {
		ele := c.ll.PushFront(&entry{key, value, expire.Add(randDuration), time.Now()})//实际传递的是指针
		c.cache[key] = ele
		c.nbytes += int64(value.Len()) + int64(len(key))
	}

	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		fmt.Println("Evicting oldest entry") // 添加调试信息
		c.RemoveOldest()
	}
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		c.removeElement(ele)
	}
}

func (c *Cache) removeElement(ele *list.Element) {
	if ele == nil {
        return
    }
	kv := ele.Value.(*entry)
	delete(c.cache, kv.key)
	c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)//缓存被移除， 执行回调函数
	}
}
func (c *Cache) Len() int{
	return c.ll.Len()
}