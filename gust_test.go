package gust

import "testing"

type Address struct {
	Line1 string
}

type User struct {
	Id      string
	Age     int `gust:"unique"`
	Name    string
	Email   string `gust:"unique"`
	Sex     string `gust:"index"`
	College string `gust:"index"`
	Address Address
}

func TestA(t *testing.T) {
	pool := NewPool()
	defer pool.Close()

	c := pool.Get()
	defer c.Close()

	c.Do("FLUSHALL")

	user := User{"", 28, "Luis", "luis@vega.com", "", "Ateneo", Address{Line1: "79"}}
	err := Save(c, &user)
	if err != nil {
		t.Error(err)
	}
}
