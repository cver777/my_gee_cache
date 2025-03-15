package singleflight

import "sync"

type call struct {
	wg sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {//第一个请求 发现m为空，初始化
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok { //后续的并发请求会在这里获取数据
		g.mu.Unlock()
		c.wg.Wait()  //他们会等待第一个请求执行完，确保call已经被赋值
		return c.val, c.err
	}
	c := new(call)//创建一个call指针
	c.wg.Add(1)
	g.m[key] = c //实例化c 分配空间
	g.mu.Unlock()

	c.val, c.err = fn() //调用fn获取数据
	c.wg.Done()			//释放等待的Gorotines

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}