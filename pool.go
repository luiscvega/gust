package gust

import "github.com/garyburd/redigo/redis"

type Pool struct {
	*redis.Pool
}

func NewPool(server string) (*Pool, error) {
	pool := &Pool{}

	pool.Pool = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}

			return c, err
		},
	}

	c := pool.Get()
	defer c.Close()

	var err error

	saveDigest, err = redis.String(c.Do("SCRIPT", "LOAD", saveScript))
	if err != nil {
		return nil, err
	}

	deleteDigest, err = redis.String(c.Do("SCRIPT", "LOAD", deleteScript))
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func (pool Pool) Save(src interface{}) error {
	c := pool.Get()
	defer c.Close()

	return Save(c, src)
}

func (pool Pool) Fetch(dst interface{}, id string) error {
	c := pool.Get()
	defer c.Close()

	return Fetch(c, dst, id)
}

func (pool Pool) With(dst interface{}, unique string, value string) error {
	c := pool.Get()
	defer c.Close()

	return With(c, dst, unique, value)
}

func (pool Pool) FetchMany(dst interface{}, ids []string) error {
	c := pool.Get()
	defer c.Close()

	return FetchMany(c, dst, ids)
}

func (pool Pool) FetchAll(dst interface{}) error {
	c := pool.Get()
	defer c.Close()

	return FetchAll(c, dst)
}

func (pool Pool) Find(dst interface{}, queries ...string) error {
	c := pool.Get()
	defer c.Close()

	return Find(c, dst, queries...)
}

func (pool Pool) Delete(modelName, modelId string) (bool, error) {
	c := pool.Get()
	defer c.Close()

	return Delete(c, modelName, modelId)
}
