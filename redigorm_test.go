package redigorm

import (
	"fmt"
	"testing"
)

type Address struct {
	Line1 string `redis:"Line1"`
}

type User struct {
	Id      string  `redis:"id"`
	Age     int     `redis:"age"`
	Name    string  `redis:"name"`
	Email   string  `redis:"email" omg:"unique"`
	Sex     string  `redis:"sex" omg:"index"`
	College string  `redis:"college" omg:"index"`
	Address Address `redis:"address"`
}

func TestA(t *testing.T) {
	c := Open()
	defer c.Close()

	c.Do("FLUSHALL")

	Save(c, &User{"", 28, "Luis", "luis@vega.com", "male", "Ateneo", Address{"79"}})
	Save(c, &User{"", 33, "Miguel", "miguel@vega.com", "male", "UP", Address{"79"}})
	Save(c, &User{"", 31, "Paola", "paola@vega.com", "female", "Ateneo", Address{"79"}})

	user := User{}
	err := With(c, &user, "email", "luis@vega.com")
	fmt.Println(err)
	fmt.Println("USER:", user)

	users := []User{}
	err = Find(c, &users, "college:Ateneo", "sex:male")
	fmt.Println(err)
	fmt.Println("USERS:", users)

	users = []User{}
	err = FetchAll(c, &users)
	fmt.Println(err)
	fmt.Println("USERS:", users)
}
